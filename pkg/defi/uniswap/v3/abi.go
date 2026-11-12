// Package v3 is the Uniswap v3 adapter, which also covers the v3 forks that
// kept the same interfaces, PancakeSwap v3 among them.
//
// Two deliberate choices:
//
//   - Pool addresses come from Factory.getPool rather than from the CREATE2
//     derivation. The derivation needs the deployment's POOL_INIT_CODE_HASH,
//     which differs between Uniswap and its forks, and getting it wrong yields
//     a plausible-looking address that simply has no code.
//   - QuoterV2 is declared as a view function here although it is nonpayable on
//     chain: it writes nothing, but Uniswap left it non-view so that it can
//     revert to return data in the multi-hop case. Declaring it view lets it be
//     called with eth_call, which is the documented way to quote.
//
// Units: a quote is always raw base units of the token, and the price helpers in
// math.go are the only place token decimals enter the picture.
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

const factoryABIJSON = `[
  {"inputs":[
    {"name":"tokenA","type":"address"},
    {"name":"tokenB","type":"address"},
    {"name":"fee","type":"uint24"}],
   "name":"getPool","outputs":[{"type":"address"}],"stateMutability":"view","type":"function"}
]`

// The pool's feeProtocol is a uint8 in Uniswap v3 and a uint32 in PancakeSwap
// v3. Every slot0 field occupies its own 32 byte word, so declaring the wider
// type decodes both: a Uniswap pool simply has zero high bytes.
const poolABIJSON = `[
  {"inputs":[],"name":"slot0","outputs":[
    {"name":"sqrtPriceX96","type":"uint160"},
    {"name":"tick","type":"int24"},
    {"name":"observationIndex","type":"uint16"},
    {"name":"observationCardinality","type":"uint16"},
    {"name":"observationCardinalityNext","type":"uint16"},
    {"name":"feeProtocol","type":"uint32"},
    {"name":"unlocked","type":"bool"}],"stateMutability":"view","type":"function"},
  {"inputs":[],"name":"liquidity","outputs":[{"type":"uint128"}],"stateMutability":"view","type":"function"},
  {"inputs":[],"name":"token0","outputs":[{"type":"address"}],"stateMutability":"view","type":"function"},
  {"inputs":[],"name":"token1","outputs":[{"type":"address"}],"stateMutability":"view","type":"function"},
  {"inputs":[],"name":"fee","outputs":[{"type":"uint24"}],"stateMutability":"view","type":"function"},
  {"inputs":[],"name":"tickSpacing","outputs":[{"type":"int24"}],"stateMutability":"view","type":"function"},
  {"anonymous":false,"inputs":[
    {"indexed":true,"name":"sender","type":"address"},
    {"indexed":true,"name":"recipient","type":"address"},
    {"indexed":false,"name":"amount0","type":"int256"},
    {"indexed":false,"name":"amount1","type":"int256"},
    {"indexed":false,"name":"sqrtPriceX96","type":"uint160"},
    {"indexed":false,"name":"liquidity","type":"uint128"},
    {"indexed":false,"name":"tick","type":"int24"}],"name":"Swap","type":"event"}
]`

const quoterABIJSON = `[
  {"inputs":[{"components":[
    {"name":"tokenIn","type":"address"},
    {"name":"tokenOut","type":"address"},
    {"name":"amountIn","type":"uint256"},
    {"name":"fee","type":"uint24"},
    {"name":"sqrtPriceLimitX96","type":"uint160"}],
    "name":"params","type":"tuple"}],
   "name":"quoteExactInputSingle","outputs":[
    {"name":"amountOut","type":"uint256"},
    {"name":"sqrtPriceX96After","type":"uint160"},
    {"name":"initializedTicksCrossed","type":"uint32"},
    {"name":"gasEstimate","type":"uint256"}],"stateMutability":"view","type":"function"},
  {"inputs":[{"components":[
    {"name":"tokenIn","type":"address"},
    {"name":"tokenOut","type":"address"},
    {"name":"amount","type":"uint256"},
    {"name":"fee","type":"uint24"},
    {"name":"sqrtPriceLimitX96","type":"uint160"}],
    "name":"params","type":"tuple"}],
   "name":"quoteExactOutputSingle","outputs":[
    {"name":"amountIn","type":"uint256"},
    {"name":"sqrtPriceX96After","type":"uint160"},
    {"name":"initializedTicksCrossed","type":"uint32"},
    {"name":"gasEstimate","type":"uint256"}],"stateMutability":"view","type":"function"}
]`

// SwapRouter02 dropped the deadline field that the original SwapRouter had, so
// a deadline is enforced by the caller rather than by the contract.
const routerABIJSON = `[
  {"inputs":[{"components":[
    {"name":"tokenIn","type":"address"},
    {"name":"tokenOut","type":"address"},
    {"name":"fee","type":"uint24"},
    {"name":"recipient","type":"address"},
    {"name":"amountIn","type":"uint256"},
    {"name":"amountOutMinimum","type":"uint256"},
    {"name":"sqrtPriceLimitX96","type":"uint160"}],
    "name":"params","type":"tuple"}],
   "name":"exactInputSingle","outputs":[{"name":"amountOut","type":"uint256"}],
   "stateMutability":"payable","type":"function"}
]`

var (
	factoryABI = mustParseABI(factoryABIJSON)
	poolABI    = mustParseABI(poolABIJSON)
	quoterABI  = mustParseABI(quoterABIJSON)
	routerABI  = mustParseABI(routerABIJSON)
)

func mustParseABI(s string) abi.ABI {
	parsed, err := abi.JSON(strings.NewReader(s))
	if err != nil {
		panic(fmt.Errorf("invalid built-in uniswap v3 abi: %w", err))
	}

	return parsed
}

// PoolABI returns the pool ABI fragment used by this package.
func PoolABI() abi.ABI {
	return poolABI
}

// RouterABI returns the router ABI fragment used by this package.
func RouterABI() abi.ABI {
	return routerABI
}

// quoteParams mirrors the QuoterV2 input tuple. The uint24 fee decodes to a
// *big.Int because it is not one of Go's integer widths.
type quoteParams struct {
	TokenIn           common.Address
	TokenOut          common.Address
	AmountIn          *big.Int
	Fee               *big.Int
	SqrtPriceLimitX96 *big.Int
}

// quoteOutputParams mirrors the quoteExactOutputSingle input tuple, whose
// amount field is named differently from the exact-input one.
type quoteOutputParams struct {
	TokenIn           common.Address
	TokenOut          common.Address
	Amount            *big.Int
	Fee               *big.Int
	SqrtPriceLimitX96 *big.Int
}

// exactInputSingleParams mirrors the SwapRouter02 input tuple.
type exactInputSingleParams struct {
	TokenIn           common.Address
	TokenOut          common.Address
	Fee               *big.Int
	Recipient         common.Address
	AmountIn          *big.Int
	AmountOutMinimum  *big.Int
	SqrtPriceLimitX96 *big.Int
}

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
	case uint32:
		return new(big.Int).SetUint64(uint64(v)), nil
	case int32:
		return big.NewInt(int64(v)), nil
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
