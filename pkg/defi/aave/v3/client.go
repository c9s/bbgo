package v3

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/chain/evm"
	"github.com/c9s/bbgo/pkg/chain/evm/multicall"
	"github.com/c9s/bbgo/pkg/chain/evm/token"
)

// ErrReadOnly is returned by the write methods when the client has no sender,
// which is the case for a chain session without a private key.
var ErrReadOnly = errors.New("this aave v3 client is read-only, configure a private key on the chain session")

// MaxAmount means "the whole balance" to Pool.withdraw and "the whole debt" to
// Pool.repay.
var MaxAmount = new(big.Int).Set(maxUint256)

// TxSender is the transaction-sending capability the write methods need.
// *evm.Sender implements it, and it is an interface so the actions can be
// tested without a node.
type TxSender = token.TxSender

// Options configures a Client. Only the caller is mandatory; a client without a
// sender is read-only.
type Options struct {
	// Provider overrides the built-in PoolAddressesProvider, which is how a
	// non-main market or a fork is pointed at.
	Provider common.Address

	// Sender enables the write methods.
	Sender TxSender

	// Tokens is used to resolve token metadata; without it, metadata is read
	// directly from the chain.
	Tokens *token.Registry

	// Allowance is the approval policy used before supplying and repaying.
	Allowance token.AllowanceConfig

	// Multicall overrides the chain's Multicall3 address.
	Multicall common.Address
}

// Client is an Aave v3 market on one chain.
type Client struct {
	caller    bind.ContractCaller
	sender    TxSender
	tokens    *token.Registry
	allowance token.AllowanceConfig
	multicall common.Address

	addresses Addresses
}

// NewClient resolves the market layout from the PoolAddressesProvider. That
// resolution is also the configuration check: a wrong provider address, or a
// chain without Aave, fails here instead of at the first trade.
func NewClient(
	ctx context.Context, chain *evm.Chain, caller bind.ContractCaller, options Options,
) (*Client, error) {
	provider := options.Provider
	if provider == (common.Address{}) {
		resolved, err := ProviderAddress(chain.ID)
		if err != nil {
			return nil, err
		}

		provider = resolved
	}

	multicallAddress := options.Multicall
	if multicallAddress == (common.Address{}) {
		multicallAddress = chain.Multicall3
	}

	client := &Client{
		caller:    caller,
		sender:    nonNilSender(options.Sender),
		tokens:    options.Tokens,
		allowance: options.Allowance,
		multicall: multicallAddress,
		addresses: Addresses{Provider: provider},
	}

	if err := client.resolveAddresses(ctx); err != nil {
		return nil, err
	}

	log.Infof("%s", client.addresses)

	return client, nil
}

func (c *Client) resolveAddresses(ctx context.Context) error {
	calls, err := buildCalls(c.addresses.Provider, addressesProviderABI, []string{
		"getPool", "getPoolDataProvider", "getPriceOracle", "getMarketId",
	})
	if err != nil {
		return err
	}

	// the market id is cosmetic, the three addresses are not
	calls[3].AllowFailure = true

	results, err := multicall.Aggregate3(ctx, c.caller, c.multicall, calls)
	if err != nil {
		return fmt.Errorf("failed to read the aave v3 addresses provider %s: %w",
			c.addresses.Provider.Hex(), err)
	}

	for i, name := range []string{"getPool", "getPoolDataProvider", "getPriceOracle"} {
		if !results[i].Success {
			return fmt.Errorf("%s reverted on the addresses provider %s, is it an aave v3 market?",
				name, c.addresses.Provider.Hex())
		}

		values, err := addressesProviderABI.Unpack(name, results[i].ReturnData)
		if err != nil {
			return fmt.Errorf("failed to decode %s: %w", name, err)
		}

		address, err := asAddress(values, 0)
		if err != nil {
			return err
		}

		if address == (common.Address{}) {
			return fmt.Errorf("%s returned the zero address on %s", name, c.addresses.Provider.Hex())
		}

		switch i {
		case 0:
			c.addresses.Pool = address
		case 1:
			c.addresses.DataProvider = address
		case 2:
			c.addresses.Oracle = address
		}
	}

	if results[3].Success {
		if values, err := addressesProviderABI.Unpack("getMarketId", results[3].ReturnData); err == nil {
			if id, ok := values[0].(string); ok {
				c.addresses.MarketID = id
			}
		}
	}

	return c.resolveBaseCurrency(ctx)
}

func (c *Client) resolveBaseCurrency(ctx context.Context) error {
	calls, err := buildCalls(c.addresses.Oracle, oracleABI, []string{"BASE_CURRENCY_UNIT", "BASE_CURRENCY"})
	if err != nil {
		return err
	}

	for i := range calls {
		calls[i].AllowFailure = true
	}

	results, err := multicall.Aggregate3(ctx, c.caller, c.multicall, calls)
	if err != nil {
		return fmt.Errorf("failed to read the aave v3 oracle %s: %w", c.addresses.Oracle.Hex(), err)
	}

	if results[0].Success {
		values, err := oracleABI.Unpack("BASE_CURRENCY_UNIT", results[0].ReturnData)
		if err == nil {
			unit, err := asBig(values, 0)
			if err == nil {
				c.addresses.BaseCurrencyUnit = unit
			}
		}
	}

	if c.addresses.BaseCurrencyUnit == nil || c.addresses.BaseCurrencyUnit.Sign() <= 0 {
		// every v3 market prices in USD with 8 decimals; fall back rather than
		// fail, because the amounts are still usable as raw values
		log.Warnf("aave v3: the oracle %s did not report BASE_CURRENCY_UNIT, assuming 1e8",
			c.addresses.Oracle.Hex())

		c.addresses.BaseCurrencyUnit = big.NewInt(1e8)
	}

	if results[1].Success {
		if values, err := oracleABI.Unpack("BASE_CURRENCY", results[1].ReturnData); err == nil {
			if address, err := asAddress(values, 0); err == nil {
				c.addresses.BaseCurrency = address
			}
		}
	}

	return nil
}

func buildCalls(target common.Address, contractABI interface {
	Pack(name string, args ...any) ([]byte, error)
}, methods []string) ([]multicall.Call, error) {
	calls := make([]multicall.Call, 0, len(methods))
	for _, method := range methods {
		data, err := contractABI.Pack(method)
		if err != nil {
			return nil, fmt.Errorf("failed to encode %s: %w", method, err)
		}

		calls = append(calls, multicall.Call{Target: target, CallData: data})
	}

	return calls, nil
}

// nonNilSender guards against a typed nil, which a caller produces by passing a
// nil *evm.Sender into the interface field: that would make CanWrite report true
// and then panic on the first write.
func nonNilSender(sender TxSender) TxSender {
	if sender == nil {
		return nil
	}

	value := reflect.ValueOf(sender)
	switch value.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Map, reflect.Slice, reflect.Func:
		if value.IsNil() {
			return nil
		}
	}

	return sender
}

// Addresses returns the resolved market layout.
func (c *Client) Addresses() Addresses {
	return c.addresses
}

// PoolAddress is the address that must be approved before supplying or repaying.
func (c *Client) PoolAddress() common.Address {
	return c.addresses.Pool
}

// CanWrite reports whether this client can send transactions.
func (c *Client) CanWrite() bool {
	return c.sender != nil
}

func (c *Client) requireSender() (TxSender, error) {
	if c.sender == nil {
		return nil, ErrReadOnly
	}

	return c.sender, nil
}

// Token resolves the metadata of an asset, through the registry when there is
// one and directly from the chain otherwise.
func (c *Client) Token(ctx context.Context, asset common.Address) (token.Token, error) {
	if c.tokens != nil {
		return c.tokens.Lookup(ctx, asset)
	}

	decimals, err := token.Decimals(ctx, c.caller, asset)
	if err != nil {
		return token.Token{}, err
	}

	t := token.Token{Address: asset, Decimals: decimals, Symbol: asset.Hex()}
	if symbol, err := token.Symbol(ctx, c.caller, asset); err == nil && symbol != "" {
		t.Symbol = symbol
	}

	return t, nil
}

// AccountData returns the account level risk view. The amounts are in the
// oracle base currency, not in token units.
func (c *Client) AccountData(ctx context.Context, user common.Address) (*AccountData, error) {
	values, err := call(ctx, c.caller, poolABI, c.addresses.Pool, "getUserAccountData", user)
	if err != nil {
		return nil, err
	}

	fields := make([]*big.Int, 6)
	for i := range fields {
		v, err := asBig(values, i)
		if err != nil {
			return nil, err
		}

		fields[i] = v
	}

	return &AccountData{
		TotalCollateralBase:     fields[0],
		TotalDebtBase:           fields[1],
		AvailableBorrowsBase:    fields[2],
		LiquidationThresholdBps: fields[3],
		LTVBps:                  fields[4],
		HealthFactor:            fields[5],
		baseCurrencyUnit:        c.addresses.BaseCurrencyUnit,
	}, nil
}

// ReservesList returns every asset listed in this market.
func (c *Client) ReservesList(ctx context.Context) ([]common.Address, error) {
	values, err := call(ctx, c.caller, poolABI, c.addresses.Pool, "getReservesList")
	if err != nil {
		return nil, err
	}

	addresses, ok := values[0].([]common.Address)
	if !ok {
		return nil, fmt.Errorf("unexpected getReservesList output type %T", values[0])
	}

	return addresses, nil
}

// Reserve returns the market wide state of one asset.
func (c *Client) Reserve(ctx context.Context, asset common.Address) (*Reserve, error) {
	values, err := call(ctx, c.caller, dataProviderABI, c.addresses.DataProvider, "getReserveData", asset)
	if err != nil {
		return nil, err
	}

	return decodeReserve(asset, values)
}

func decodeReserve(asset common.Address, values []any) (*Reserve, error) {
	fields := make([]*big.Int, 12)
	for i := range fields {
		v, err := asBig(values, i)
		if err != nil {
			return nil, err
		}

		fields[i] = v
	}

	return &Reserve{
		Asset:                   asset,
		Unbacked:                fields[0],
		AccruedToTreasuryScaled: fields[1],
		TotalAToken:             fields[2],
		TotalStableDebt:         fields[3],
		TotalVariableDebt:       fields[4],
		LiquidityRate:           fields[5],
		VariableBorrowRate:      fields[6],
		StableBorrowRate:        fields[7],
		AverageStableBorrowRate: fields[8],
		LiquidityIndex:          fields[9],
		VariableBorrowIndex:     fields[10],
		LastUpdated:             time.Unix(fields[11].Int64(), 0),
	}, nil
}

// UserReserve returns one account's position in one reserve, in token units.
func (c *Client) UserReserve(
	ctx context.Context, asset, user common.Address,
) (*UserReserve, error) {
	values, err := call(ctx, c.caller, dataProviderABI, c.addresses.DataProvider,
		"getUserReserveData", asset, user)
	if err != nil {
		return nil, err
	}

	return decodeUserReserve(asset, values)
}

func decodeUserReserve(asset common.Address, values []any) (*UserReserve, error) {
	fields := make([]*big.Int, 7)
	for i := range fields {
		v, err := asBig(values, i)
		if err != nil {
			return nil, err
		}

		fields[i] = v
	}

	// index 7 is stableRateLastUpdated, which this adapter has no use for
	collateral, err := asBool(values, 8)
	if err != nil {
		return nil, err
	}

	return &UserReserve{
		Asset:                    asset,
		ATokenBalance:            fields[0],
		StableDebt:               fields[1],
		VariableDebt:             fields[2],
		PrincipalStableDebt:      fields[3],
		ScaledVariableDebt:       fields[4],
		StableBorrowRate:         fields[5],
		LiquidityRate:            fields[6],
		UsageAsCollateralEnabled: collateral,
	}, nil
}

// UserReserves returns every reserve where the account holds a balance or a
// debt, reading the whole market in one batched call.
func (c *Client) UserReserves(ctx context.Context, user common.Address) ([]*UserReserve, error) {
	assets, err := c.ReservesList(ctx)
	if err != nil {
		return nil, err
	}

	calls := make([]multicall.Call, 0, len(assets))
	for _, asset := range assets {
		data, err := dataProviderABI.Pack("getUserReserveData", asset, user)
		if err != nil {
			return nil, err
		}

		calls = append(calls, multicall.Call{
			Target:       c.addresses.DataProvider,
			AllowFailure: true,
			CallData:     data,
		})
	}

	results, err := multicall.Aggregate3Chunked(ctx, c.caller, c.multicall, calls, 0)
	if err != nil {
		return nil, err
	}

	positions := make([]*UserReserve, 0, 4)
	for i, result := range results {
		if !result.Success {
			continue
		}

		values, err := dataProviderABI.Unpack("getUserReserveData", result.ReturnData)
		if err != nil {
			log.WithError(err).Warnf("aave v3: failed to decode the user reserve data of %s", assets[i].Hex())
			continue
		}

		position, err := decodeUserReserve(assets[i], values)
		if err != nil {
			log.WithError(err).Warnf("aave v3: failed to read the user reserve data of %s", assets[i].Hex())
			continue
		}

		if position.IsEmpty() {
			continue
		}

		positions = append(positions, position)
	}

	return positions, nil
}

// Reserves returns the market wide state of every listed asset in one batched
// call.
func (c *Client) Reserves(ctx context.Context) ([]*Reserve, error) {
	assets, err := c.ReservesList(ctx)
	if err != nil {
		return nil, err
	}

	calls := make([]multicall.Call, 0, len(assets))
	for _, asset := range assets {
		data, err := dataProviderABI.Pack("getReserveData", asset)
		if err != nil {
			return nil, err
		}

		calls = append(calls, multicall.Call{
			Target:       c.addresses.DataProvider,
			AllowFailure: true,
			CallData:     data,
		})
	}

	results, err := multicall.Aggregate3Chunked(ctx, c.caller, c.multicall, calls, 0)
	if err != nil {
		return nil, err
	}

	reserves := make([]*Reserve, 0, len(assets))
	for i, result := range results {
		if !result.Success {
			log.Warnf("aave v3: getReserveData reverted for %s", assets[i].Hex())
			continue
		}

		values, err := dataProviderABI.Unpack("getReserveData", result.ReturnData)
		if err != nil {
			return nil, fmt.Errorf("failed to decode the reserve data of %s: %w", assets[i].Hex(), err)
		}

		reserve, err := decodeReserve(assets[i], values)
		if err != nil {
			return nil, err
		}

		reserves = append(reserves, reserve)
	}

	return reserves, nil
}

// ReserveConfig returns the risk configuration of one reserve.
func (c *Client) ReserveConfig(ctx context.Context, asset common.Address) (*ReserveConfig, error) {
	values, err := call(ctx, c.caller, dataProviderABI, c.addresses.DataProvider,
		"getReserveConfigurationData", asset)
	if err != nil {
		return nil, err
	}

	numbers := make([]*big.Int, 5)
	for i := range numbers {
		v, err := asBig(values, i)
		if err != nil {
			return nil, err
		}

		numbers[i] = v
	}

	flags := make([]bool, 5)
	for i := range flags {
		v, err := asBool(values, 5+i)
		if err != nil {
			return nil, err
		}

		flags[i] = v
	}

	return &ReserveConfig{
		Asset:                    asset,
		Decimals:                 int(numbers[0].Int64()),
		LTVBps:                   numbers[1],
		LiquidationThresholdBps:  numbers[2],
		LiquidationBonusBps:      numbers[3],
		ReserveFactorBps:         numbers[4],
		UsageAsCollateralEnabled: flags[0],
		BorrowingEnabled:         flags[1],
		StableBorrowRateEnabled:  flags[2],
		IsActive:                 flags[3],
		IsFrozen:                 flags[4],
	}, nil
}

// ReserveTokens returns the aToken and debt token addresses of one reserve.
func (c *Client) ReserveTokens(ctx context.Context, asset common.Address) (*ReserveTokens, error) {
	values, err := call(ctx, c.caller, dataProviderABI, c.addresses.DataProvider,
		"getReserveTokensAddresses", asset)
	if err != nil {
		return nil, err
	}

	addresses := make([]common.Address, 3)
	for i := range addresses {
		v, err := asAddress(values, i)
		if err != nil {
			return nil, err
		}

		addresses[i] = v
	}

	return &ReserveTokens{
		AToken:            addresses[0],
		StableDebtToken:   addresses[1],
		VariableDebtToken: addresses[2],
	}, nil
}

// AssetPrice returns the oracle price of one asset, scaled by
// Addresses.BaseCurrencyUnit.
func (c *Client) AssetPrice(ctx context.Context, asset common.Address) (*big.Int, error) {
	values, err := call(ctx, c.caller, oracleABI, c.addresses.Oracle, "getAssetPrice", asset)
	if err != nil {
		return nil, err
	}

	return asBig(values, 0)
}

// AssetPrices returns the oracle prices of several assets in one call.
func (c *Client) AssetPrices(
	ctx context.Context, assets []common.Address,
) (map[common.Address]*big.Int, error) {
	if len(assets) == 0 {
		return nil, nil
	}

	values, err := call(ctx, c.caller, oracleABI, c.addresses.Oracle, "getAssetsPrices", assets)
	if err != nil {
		return nil, err
	}

	prices, ok := values[0].([]*big.Int)
	if !ok {
		return nil, fmt.Errorf("unexpected getAssetsPrices output type %T", values[0])
	}

	if len(prices) != len(assets) {
		return nil, fmt.Errorf("the oracle returned %d prices for %d assets", len(prices), len(assets))
	}

	out := make(map[common.Address]*big.Int, len(assets))
	for i, asset := range assets {
		out[asset] = prices[i]
	}

	return out, nil
}
