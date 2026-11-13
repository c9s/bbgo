// Package v3 is the Aave v3 adapter. It is shaped like the Aave contracts
// rather than like a generic lending interface: the same deployment layout
// (PoolAddressesProvider -> Pool / PoolDataProvider / AaveOracle) exists on
// Ethereum, BSC and Polygon, so one set of bindings covers all three chains.
//
// Units, which Aave does not make obvious:
//
//   - getUserAccountData amounts are denominated in the oracle base currency,
//     which is USD with 8 decimals on v3. They are not token amounts.
//   - healthFactor is WAD scaled (1e18) and is set to max uint256 when the
//     account has no debt.
//   - liquidityRate and variableBorrowRate are RAY scaled (1e27) per-second
//     compounding rates, not percentages.
//   - LTV, the liquidation threshold and the reserve factor are basis points.
//   - aToken balances rebase: the scaled balance times the liquidity index is
//     the real position.
package v3

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
)

// InterestRateModeVariable is the only borrow mode Aave v3 still supports; the
// stable rate mode was disabled and then removed.
const InterestRateModeVariable = 2

// ReferralCode is unused in Aave v3 but still part of the function signatures.
const ReferralCode = 0

const addressesProviderABIJSON = `[
  {"inputs":[],"name":"getPool","outputs":[{"type":"address"}],"stateMutability":"view","type":"function"},
  {"inputs":[],"name":"getPoolDataProvider","outputs":[{"type":"address"}],"stateMutability":"view","type":"function"},
  {"inputs":[],"name":"getPriceOracle","outputs":[{"type":"address"}],"stateMutability":"view","type":"function"},
  {"inputs":[],"name":"getMarketId","outputs":[{"type":"string"}],"stateMutability":"view","type":"function"}
]`

const poolABIJSON = `[
  {"inputs":[{"name":"user","type":"address"}],"name":"getUserAccountData","outputs":[
    {"name":"totalCollateralBase","type":"uint256"},
    {"name":"totalDebtBase","type":"uint256"},
    {"name":"availableBorrowsBase","type":"uint256"},
    {"name":"currentLiquidationThreshold","type":"uint256"},
    {"name":"ltv","type":"uint256"},
    {"name":"healthFactor","type":"uint256"}],"stateMutability":"view","type":"function"},
  {"inputs":[],"name":"getReservesList","outputs":[{"type":"address[]"}],"stateMutability":"view","type":"function"},
  {"inputs":[
    {"name":"asset","type":"address"},
    {"name":"amount","type":"uint256"},
    {"name":"onBehalfOf","type":"address"},
    {"name":"referralCode","type":"uint16"}],"name":"supply","outputs":[],"stateMutability":"nonpayable","type":"function"},
  {"inputs":[
    {"name":"asset","type":"address"},
    {"name":"amount","type":"uint256"},
    {"name":"to","type":"address"}],"name":"withdraw","outputs":[{"type":"uint256"}],"stateMutability":"nonpayable","type":"function"},
  {"inputs":[
    {"name":"asset","type":"address"},
    {"name":"amount","type":"uint256"},
    {"name":"interestRateMode","type":"uint256"},
    {"name":"referralCode","type":"uint16"},
    {"name":"onBehalfOf","type":"address"}],"name":"borrow","outputs":[],"stateMutability":"nonpayable","type":"function"},
  {"inputs":[
    {"name":"asset","type":"address"},
    {"name":"amount","type":"uint256"},
    {"name":"interestRateMode","type":"uint256"},
    {"name":"onBehalfOf","type":"address"}],"name":"repay","outputs":[{"type":"uint256"}],"stateMutability":"nonpayable","type":"function"},
  {"inputs":[
    {"name":"asset","type":"address"},
    {"name":"useAsCollateral","type":"bool"}],"name":"setUserUseReserveAsCollateral","outputs":[],"stateMutability":"nonpayable","type":"function"}
]`

const dataProviderABIJSON = `[
  {"inputs":[{"name":"asset","type":"address"}],"name":"getReserveData","outputs":[
    {"name":"unbacked","type":"uint256"},
    {"name":"accruedToTreasuryScaled","type":"uint256"},
    {"name":"totalAToken","type":"uint256"},
    {"name":"totalStableDebt","type":"uint256"},
    {"name":"totalVariableDebt","type":"uint256"},
    {"name":"liquidityRate","type":"uint256"},
    {"name":"variableBorrowRate","type":"uint256"},
    {"name":"stableBorrowRate","type":"uint256"},
    {"name":"averageStableBorrowRate","type":"uint256"},
    {"name":"liquidityIndex","type":"uint256"},
    {"name":"variableBorrowIndex","type":"uint256"},
    {"name":"lastUpdateTimestamp","type":"uint40"}],"stateMutability":"view","type":"function"},
  {"inputs":[
    {"name":"asset","type":"address"},
    {"name":"user","type":"address"}],"name":"getUserReserveData","outputs":[
    {"name":"currentATokenBalance","type":"uint256"},
    {"name":"currentStableDebt","type":"uint256"},
    {"name":"currentVariableDebt","type":"uint256"},
    {"name":"principalStableDebt","type":"uint256"},
    {"name":"scaledVariableDebt","type":"uint256"},
    {"name":"stableBorrowRate","type":"uint256"},
    {"name":"liquidityRate","type":"uint256"},
    {"name":"stableRateLastUpdated","type":"uint40"},
    {"name":"usageAsCollateralEnabled","type":"bool"}],"stateMutability":"view","type":"function"},
  {"inputs":[{"name":"asset","type":"address"}],"name":"getReserveTokensAddresses","outputs":[
    {"name":"aTokenAddress","type":"address"},
    {"name":"stableDebtTokenAddress","type":"address"},
    {"name":"variableDebtTokenAddress","type":"address"}],"stateMutability":"view","type":"function"},
  {"inputs":[{"name":"asset","type":"address"}],"name":"getReserveConfigurationData","outputs":[
    {"name":"decimals","type":"uint256"},
    {"name":"ltv","type":"uint256"},
    {"name":"liquidationThreshold","type":"uint256"},
    {"name":"liquidationBonus","type":"uint256"},
    {"name":"reserveFactor","type":"uint256"},
    {"name":"usageAsCollateralEnabled","type":"bool"},
    {"name":"borrowingEnabled","type":"bool"},
    {"name":"stableBorrowRateEnabled","type":"bool"},
    {"name":"isActive","type":"bool"},
    {"name":"isFrozen","type":"bool"}],"stateMutability":"view","type":"function"}
]`

const oracleABIJSON = `[
  {"inputs":[{"name":"asset","type":"address"}],"name":"getAssetPrice","outputs":[{"type":"uint256"}],"stateMutability":"view","type":"function"},
  {"inputs":[{"name":"assets","type":"address[]"}],"name":"getAssetsPrices","outputs":[{"type":"uint256[]"}],"stateMutability":"view","type":"function"},
  {"inputs":[],"name":"BASE_CURRENCY_UNIT","outputs":[{"type":"uint256"}],"stateMutability":"view","type":"function"},
  {"inputs":[],"name":"BASE_CURRENCY","outputs":[{"type":"address"}],"stateMutability":"view","type":"function"}
]`

var (
	addressesProviderABI = mustParseABI(addressesProviderABIJSON)
	poolABI              = mustParseABI(poolABIJSON)
	dataProviderABI      = mustParseABI(dataProviderABIJSON)
	oracleABI            = mustParseABI(oracleABIJSON)
)

func mustParseABI(s string) abi.ABI {
	parsed, err := abi.JSON(strings.NewReader(s))
	if err != nil {
		panic(fmt.Errorf("invalid built-in aave v3 abi: %w", err))
	}

	return parsed
}

// PoolABI returns the Pool ABI fragment used by this package, so callers can
// encode a call this adapter does not wrap yet.
func PoolABI() abi.ABI {
	return poolABI
}

// call performs one eth_call and unpacks the result.
func call(
	ctx context.Context,
	caller bind.ContractCaller,
	contractABI abi.ABI,
	address common.Address,
	method string,
	args ...any,
) ([]any, error) {
	data, err := contractABI.Pack(method, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to encode %s: %w", method, err)
	}

	output, err := caller.CallContract(ctx, ethereum.CallMsg{To: &address, Data: data}, nil)
	if err != nil {
		return nil, fmt.Errorf("%s on %s failed: %w", method, address.Hex(), err)
	}

	values, err := contractABI.Unpack(method, output)
	if err != nil {
		return nil, fmt.Errorf("failed to decode the result of %s on %s: %w", method, address.Hex(), err)
	}

	return values, nil
}

func asBig(values []any, index int) (*big.Int, error) {
	if index >= len(values) {
		return nil, fmt.Errorf("missing output at index %d", index)
	}

	switch v := values[index].(type) {
	case *big.Int:
		return v, nil
	case uint64:
		return new(big.Int).SetUint64(v), nil
	default:
		return nil, fmt.Errorf("unexpected output type %T at index %d", values[index], index)
	}
}

func asAddress(values []any, index int) (common.Address, error) {
	if index >= len(values) {
		return common.Address{}, fmt.Errorf("missing output at index %d", index)
	}

	address, ok := values[index].(common.Address)
	if !ok {
		return common.Address{}, fmt.Errorf("unexpected output type %T at index %d", values[index], index)
	}

	return address, nil
}

func asBool(values []any, index int) (bool, error) {
	if index >= len(values) {
		return false, fmt.Errorf("missing output at index %d", index)
	}

	b, ok := values[index].(bool)
	if !ok {
		return false, fmt.Errorf("unexpected output type %T at index %d", values[index], index)
	}

	return b, nil
}
