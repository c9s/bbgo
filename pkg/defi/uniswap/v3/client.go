package v3

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"sort"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/chain/evm"
	"github.com/c9s/bbgo/pkg/chain/evm/multicall"
	"github.com/c9s/bbgo/pkg/chain/evm/token"
)

// ErrReadOnly is returned by Swap when the client has no sender.
var ErrReadOnly = errors.New("this uniswap v3 client is read-only, configure a private key on the chain session")

// ErrNoPool is returned when no pool exists for a pair at any fee tier.
var ErrNoPool = errors.New("no pool exists for this pair")

// TxSender is the transaction-sending capability the swap needs. *evm.Sender
// implements it.
type TxSender = token.TxSender

// Options configures a Client. Only the caller is mandatory.
type Options struct {
	// Deployment selects a named deployment, such as "uniswap" or
	// "pancakeswap". Empty means the chain's default.
	Deployment string

	// Override replaces the deployment addresses entirely, for a fork or an
	// unlisted v3 clone.
	Override *Deployment

	// Sender enables Swap.
	Sender TxSender

	// Tokens resolves token metadata; without it, metadata is read directly.
	Tokens *token.Registry

	// Allowance is the approval policy used before swapping.
	Allowance token.AllowanceConfig

	// Multicall overrides the chain's Multicall3 address.
	Multicall common.Address
}

// Client is one v3 deployment on one chain.
type Client struct {
	caller    bind.ContractCaller
	sender    TxSender
	tokens    *token.Registry
	allowance token.AllowanceConfig
	multicall common.Address

	deployment Deployment
}

// NewClient resolves the deployment and verifies that its contracts exist on
// chain, so a wrong address fails here rather than as an empty quote later.
func NewClient(
	ctx context.Context, chain *evm.Chain, caller bind.ContractCaller, options Options,
) (*Client, error) {
	deployment := Deployment{}

	if options.Override != nil {
		deployment = *options.Override
		deployment.ChainID = chain.ID

		if len(deployment.FeeTiers) == 0 {
			deployment.FeeTiers = []uint32{FeeTier100, FeeTier500, FeeTier3000, FeeTier10000}
		}
	} else {
		resolved, err := LookupDeployment(chain.ID, options.Deployment)
		if err != nil {
			return nil, err
		}

		deployment = resolved
	}

	multicallAddress := options.Multicall
	if multicallAddress == (common.Address{}) {
		multicallAddress = chain.Multicall3
	}

	client := &Client{
		caller:     caller,
		sender:     nonNilSender(options.Sender),
		tokens:     options.Tokens,
		allowance:  options.Allowance,
		multicall:  multicallAddress,
		deployment: deployment,
	}

	contracts := map[string]common.Address{
		"factory": deployment.Factory,
		"quoter":  deployment.QuoterV2,
	}

	if client.sender != nil {
		contracts["router"] = deployment.SwapRouter
	}

	for name, address := range contracts {
		if address == (common.Address{}) {
			return nil, fmt.Errorf("the %s address of the %s deployment is not configured", name, deployment.Name)
		}

		code, err := caller.CodeAt(ctx, address, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to check the %s contract at %s: %w", name, address.Hex(), err)
		}

		if len(code) == 0 {
			return nil, fmt.Errorf("no contract is deployed at the %s address %s on %s",
				name, address.Hex(), chain.Name)
		}
	}

	log.Infof("%s", deployment)

	return client, nil
}

// nonNilSender guards against a typed nil, which a caller produces by passing a
// nil *evm.Sender into the interface field.
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

// Deployment returns the resolved deployment.
func (c *Client) Deployment() Deployment {
	return c.deployment
}

// RouterAddress is the address that must be approved before swapping.
func (c *Client) RouterAddress() common.Address {
	return c.deployment.SwapRouter
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

// Token resolves the metadata of a token, through the registry when there is
// one and directly from the chain otherwise.
func (c *Client) Token(ctx context.Context, address common.Address) (token.Token, error) {
	if c.tokens != nil {
		return c.tokens.Lookup(ctx, address)
	}

	decimals, err := token.Decimals(ctx, c.caller, address)
	if err != nil {
		return token.Token{}, err
	}

	t := token.Token{Address: address, Decimals: decimals, Symbol: address.Hex()}
	if symbol, err := token.Symbol(ctx, c.caller, address); err == nil && symbol != "" {
		t.Symbol = symbol
	}

	return t, nil
}

// PoolAddress asks the factory for the pool of a pair at one fee tier. It
// returns the zero address when no such pool exists.
func (c *Client) PoolAddress(
	ctx context.Context, tokenA, tokenB common.Address, fee uint32,
) (common.Address, error) {
	values, err := call(ctx, c.caller, factoryABI, c.deployment.Factory,
		"getPool", tokenA, tokenB, new(big.Int).SetUint64(uint64(fee)))
	if err != nil {
		return common.Address{}, err
	}

	return asAddress(values, 0)
}

var poolStateMethods = []string{"slot0", "liquidity", "token0", "token1", "fee", "tickSpacing"}

// Pool reads the full state of one pool in a single batched call.
func (c *Client) Pool(ctx context.Context, address common.Address) (*Pool, error) {
	pools, err := c.pools(ctx, []common.Address{address})
	if err != nil {
		return nil, err
	}

	if len(pools) == 0 {
		return nil, fmt.Errorf("the pool at %s did not answer", address.Hex())
	}

	return pools[0], nil
}

// pools reads several pools in one batched call, skipping the ones that do not
// answer.
func (c *Client) pools(ctx context.Context, addresses []common.Address) ([]*Pool, error) {
	if len(addresses) == 0 {
		return nil, nil
	}

	calls := make([]multicall.Call, 0, len(addresses)*len(poolStateMethods))
	for _, address := range addresses {
		for _, method := range poolStateMethods {
			data, err := poolABI.Pack(method)
			if err != nil {
				return nil, err
			}

			calls = append(calls, multicall.Call{Target: address, AllowFailure: true, CallData: data})
		}
	}

	results, err := multicall.Aggregate3Chunked(ctx, c.caller, c.multicall, calls, 0)
	if err != nil {
		return nil, err
	}

	out := make([]*Pool, 0, len(addresses))

	for i, address := range addresses {
		window := results[i*len(poolStateMethods) : (i+1)*len(poolStateMethods)]

		pool, err := decodePool(address, window)
		if err != nil {
			log.WithError(err).Warnf("uniswap v3: skipping the pool at %s", address.Hex())
			continue
		}

		out = append(out, pool)
	}

	return out, nil
}

func decodePool(address common.Address, results []multicall.Result) (*Pool, error) {
	for i, method := range poolStateMethods {
		if !results[i].Success {
			return nil, fmt.Errorf("%s reverted", method)
		}
	}

	slot0, err := poolABI.Unpack("slot0", results[0].ReturnData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode slot0: %w", err)
	}

	sqrtPriceX96, err := asBig(slot0, 0)
	if err != nil {
		return nil, err
	}

	tick, err := asBig(slot0, 1)
	if err != nil {
		return nil, err
	}

	unlocked, ok := slot0[6].(bool)
	if !ok {
		return nil, fmt.Errorf("unexpected slot0 unlocked type %T", slot0[6])
	}

	liquidityValues, err := poolABI.Unpack("liquidity", results[1].ReturnData)
	if err != nil {
		return nil, err
	}

	liquidity, err := asBig(liquidityValues, 0)
	if err != nil {
		return nil, err
	}

	token0Values, err := poolABI.Unpack("token0", results[2].ReturnData)
	if err != nil {
		return nil, err
	}

	token0, err := asAddress(token0Values, 0)
	if err != nil {
		return nil, err
	}

	token1Values, err := poolABI.Unpack("token1", results[3].ReturnData)
	if err != nil {
		return nil, err
	}

	token1, err := asAddress(token1Values, 0)
	if err != nil {
		return nil, err
	}

	feeValues, err := poolABI.Unpack("fee", results[4].ReturnData)
	if err != nil {
		return nil, err
	}

	fee, err := asBig(feeValues, 0)
	if err != nil {
		return nil, err
	}

	spacingValues, err := poolABI.Unpack("tickSpacing", results[5].ReturnData)
	if err != nil {
		return nil, err
	}

	tickSpacing, err := asBig(spacingValues, 0)
	if err != nil {
		return nil, err
	}

	return &Pool{
		Address:      address,
		Token0:       token0,
		Token1:       token1,
		Fee:          uint32(fee.Uint64()),
		TickSpacing:  int32(tickSpacing.Int64()),
		SqrtPriceX96: sqrtPriceX96,
		Tick:         int32(tick.Int64()),
		Liquidity:    liquidity,
		Unlocked:     unlocked,
	}, nil
}

// FindPools returns every existing pool of a pair, deepest liquidity first. It
// probes the deployment's fee ladder in one batched call and then reads the
// pools that exist in a second one.
func (c *Client) FindPools(ctx context.Context, tokenA, tokenB common.Address) ([]*Pool, error) {
	calls := make([]multicall.Call, 0, len(c.deployment.FeeTiers))
	for _, fee := range c.deployment.FeeTiers {
		data, err := factoryABI.Pack("getPool", tokenA, tokenB, new(big.Int).SetUint64(uint64(fee)))
		if err != nil {
			return nil, err
		}

		calls = append(calls, multicall.Call{
			Target:       c.deployment.Factory,
			AllowFailure: true,
			CallData:     data,
		})
	}

	results, err := multicall.Aggregate3(ctx, c.caller, c.multicall, calls)
	if err != nil {
		return nil, err
	}

	addresses := make([]common.Address, 0, len(results))
	for _, result := range results {
		if !result.Success {
			continue
		}

		values, err := factoryABI.Unpack("getPool", result.ReturnData)
		if err != nil {
			continue
		}

		address, err := asAddress(values, 0)
		if err != nil || address == (common.Address{}) {
			continue
		}

		addresses = append(addresses, address)
	}

	pools, err := c.pools(ctx, addresses)
	if err != nil {
		return nil, err
	}

	sort.Slice(pools, func(i, j int) bool {
		return pools[i].Liquidity.Cmp(pools[j].Liquidity) > 0
	})

	return pools, nil
}

// FindPool returns the pool of a pair with the most in-range liquidity.
//
// The deepest pool is not always the one with the best price for a given size:
// use BestQuote when the answer has to account for the trade itself.
func (c *Client) FindPool(ctx context.Context, tokenA, tokenB common.Address) (*Pool, error) {
	pools, err := c.FindPools(ctx, tokenA, tokenB)
	if err != nil {
		return nil, err
	}

	if len(pools) == 0 {
		return nil, fmt.Errorf("%w: %s / %s on %s", ErrNoPool, tokenA.Hex(), tokenB.Hex(), c.deployment.Name)
	}

	return pools[0], nil
}

// Quote asks the quoter what one exact-input swap would return at one fee tier.
func (c *Client) Quote(
	ctx context.Context, tokenIn, tokenOut common.Address, fee uint32, amountIn *big.Int,
) (*Quote, error) {
	if amountIn == nil || amountIn.Sign() <= 0 {
		return nil, fmt.Errorf("the input amount must be positive")
	}

	params := quoteParams{
		TokenIn:           tokenIn,
		TokenOut:          tokenOut,
		AmountIn:          amountIn,
		Fee:               new(big.Int).SetUint64(uint64(fee)),
		SqrtPriceLimitX96: new(big.Int),
	}

	values, err := call(ctx, c.caller, quoterABI, c.deployment.QuoterV2, "quoteExactInputSingle", params)
	if err != nil {
		return nil, evm.WrapRevert("quote", err)
	}

	return decodeQuote(tokenIn, tokenOut, fee, amountIn, values)
}

// QuoteExactOut asks the quoter what input is needed for an exact output.
func (c *Client) QuoteExactOut(
	ctx context.Context, tokenIn, tokenOut common.Address, fee uint32, amountOut *big.Int,
) (*Quote, error) {
	if amountOut == nil || amountOut.Sign() <= 0 {
		return nil, fmt.Errorf("the output amount must be positive")
	}

	params := quoteOutputParams{
		TokenIn:           tokenIn,
		TokenOut:          tokenOut,
		Amount:            amountOut,
		Fee:               new(big.Int).SetUint64(uint64(fee)),
		SqrtPriceLimitX96: new(big.Int),
	}

	values, err := call(ctx, c.caller, quoterABI, c.deployment.QuoterV2, "quoteExactOutputSingle", params)
	if err != nil {
		return nil, evm.WrapRevert("quote", err)
	}

	amountIn, err := asBig(values, 0)
	if err != nil {
		return nil, err
	}

	quote, err := decodeQuote(tokenIn, tokenOut, fee, amountIn, values)
	if err != nil {
		return nil, err
	}

	quote.AmountIn = amountIn
	quote.AmountOut = amountOut

	return quote, nil
}

func decodeQuote(
	tokenIn, tokenOut common.Address, fee uint32, amountIn *big.Int, values []any,
) (*Quote, error) {
	amountOut, err := asBig(values, 0)
	if err != nil {
		return nil, err
	}

	sqrtAfter, err := asBig(values, 1)
	if err != nil {
		return nil, err
	}

	ticks, err := asBig(values, 2)
	if err != nil {
		return nil, err
	}

	gasEstimate, err := asBig(values, 3)
	if err != nil {
		return nil, err
	}

	return &Quote{
		TokenIn:                 tokenIn,
		TokenOut:                tokenOut,
		Fee:                     fee,
		AmountIn:                amountIn,
		AmountOut:               amountOut,
		SqrtPriceX96After:       sqrtAfter,
		InitializedTicksCrossed: uint32(ticks.Uint64()),
		GasEstimate:             gasEstimate,
	}, nil
}

// BestQuote quotes every fee tier of the deployment and returns the one with
// the largest output. Each tier is a separate eth_call because the quoter
// reverts for a tier with no pool, which would abort a batch.
func (c *Client) BestQuote(
	ctx context.Context, tokenIn, tokenOut common.Address, amountIn *big.Int,
) (*Quote, error) {
	var best *Quote
	var lastErr error

	for _, fee := range c.deployment.FeeTiers {
		quote, err := c.Quote(ctx, tokenIn, tokenOut, fee, amountIn)
		if err != nil {
			lastErr = err
			continue
		}

		if quote.AmountOut.Sign() == 0 {
			continue
		}

		if best == nil || quote.AmountOut.Cmp(best.AmountOut) > 0 {
			best = quote
		}
	}

	if best == nil {
		if lastErr != nil {
			return nil, fmt.Errorf("%w: %s / %s on %s: %w",
				ErrNoPool, tokenIn.Hex(), tokenOut.Hex(), c.deployment.Name, lastErr)
		}

		return nil, fmt.Errorf("%w: %s / %s on %s",
			ErrNoPool, tokenIn.Hex(), tokenOut.Hex(), c.deployment.Name)
	}

	return best, nil
}
