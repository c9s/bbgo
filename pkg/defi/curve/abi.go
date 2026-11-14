// Package curve is the Curve adapter. It covers the pools that Curve's own
// MetaRegistry indexes: StableSwap and StableSwap-NG, their metapools, and the
// CryptoSwap pools (twocrypto, tricrypto and their NG versions).
//
// Curve is not one interface. Where Uniswap v3 has a single pool ABI and a
// router, Curve has a family of pool implementations that disagree on how a
// coin is addressed and on how the pool is entered:
//
//   - StableSwap types a coin index as int128, CryptoSwap types it as uint256.
//     The two produce different selectors for the same call, and nothing in the
//     registry reports which one a pool speaks, so this package detects it by
//     calling both and keeping the one that answers. IndexKind carries the
//     result.
//   - A metapool holds an LP token of another pool, so a pair that looks
//     tradable is reached through exchange_underlying rather than exchange.
//     The registry does report this, as the third return value of
//     get_coin_indices.
//   - A swap is sent to the pool itself. There is no router to approve, so the
//     allowance goes to the pool.
//
// Only one address is hardcoded, the AddressProvider, which Curve deploys at
// the same address on every chain. Everything else is resolved from it, which
// is also the configuration check: a chain without Curve fails at construction.
//
// Units: every amount is raw base units of the coin. A fee is Curve's own 1e10
// fixed point (FeeDenominator).
package curve

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

const addressProviderABIJSON = `[
  {"inputs":[{"name":"id","type":"uint256"}],"name":"get_address",
   "outputs":[{"type":"address"}],"stateMutability":"view","type":"function"}
]`

// The MetaRegistry flattens every Curve registry and factory of a chain behind
// one interface. MaxCoins is 8 and the fee array is 10 wide in every
// deployment, including the ones that hold fewer values.
const metaRegistryABIJSON = `[
  {"inputs":[],"name":"pool_count","outputs":[{"type":"uint256"}],"stateMutability":"view","type":"function"},
  {"inputs":[{"name":"index","type":"uint256"}],"name":"pool_list",
   "outputs":[{"type":"address"}],"stateMutability":"view","type":"function"},
  {"inputs":[{"name":"from","type":"address"},{"name":"to","type":"address"}],
   "name":"find_pools_for_coins","outputs":[{"type":"address[]"}],
   "stateMutability":"view","type":"function"},
  {"inputs":[{"name":"pool","type":"address"},{"name":"from","type":"address"},{"name":"to","type":"address"}],
   "name":"get_coin_indices","outputs":[
     {"name":"i","type":"int128"},
     {"name":"j","type":"int128"},
     {"name":"is_underlying","type":"bool"}],"stateMutability":"view","type":"function"},
  {"inputs":[{"name":"pool","type":"address"}],"name":"get_coins",
   "outputs":[{"type":"address[8]"}],"stateMutability":"view","type":"function"},
  {"inputs":[{"name":"pool","type":"address"}],"name":"get_decimals",
   "outputs":[{"type":"uint256[8]"}],"stateMutability":"view","type":"function"},
  {"inputs":[{"name":"pool","type":"address"}],"name":"get_balances",
   "outputs":[{"type":"uint256[8]"}],"stateMutability":"view","type":"function"},
  {"inputs":[{"name":"pool","type":"address"}],"name":"get_underlying_coins",
   "outputs":[{"type":"address[8]"}],"stateMutability":"view","type":"function"},
  {"inputs":[{"name":"pool","type":"address"}],"name":"get_underlying_decimals",
   "outputs":[{"type":"uint256[8]"}],"stateMutability":"view","type":"function"},
  {"inputs":[{"name":"pool","type":"address"}],"name":"get_underlying_balances",
   "outputs":[{"type":"uint256[8]"}],"stateMutability":"view","type":"function"},
  {"inputs":[{"name":"pool","type":"address"}],"name":"get_n_coins",
   "outputs":[{"type":"uint256"}],"stateMutability":"view","type":"function"},
  {"inputs":[{"name":"pool","type":"address"}],"name":"get_fees",
   "outputs":[{"type":"uint256[10]"}],"stateMutability":"view","type":"function"},
  {"inputs":[{"name":"pool","type":"address"}],"name":"is_meta",
   "outputs":[{"type":"bool"}],"stateMutability":"view","type":"function"},
  {"inputs":[{"name":"pool","type":"address"}],"name":"get_pool_name",
   "outputs":[{"type":"string"}],"stateMutability":"view","type":"function"},
  {"inputs":[{"name":"pool","type":"address"}],"name":"get_lp_token",
   "outputs":[{"type":"address"}],"stateMutability":"view","type":"function"},
  {"inputs":[{"name":"pool","type":"address"}],"name":"get_base_pool",
   "outputs":[{"type":"address"}],"stateMutability":"view","type":"function"}
]`

// The StableSwap pool ABI. Coin indices are int128.
//
// exchange is declared with four arguments although the NG implementations take
// a fifth, a receiver with a default value. Vyper compiles a default argument
// into one selector per arity, so the four argument form exists on every
// implementation and the output always goes to the caller.
const stablePoolABIJSON = `[
  {"inputs":[
    {"name":"i","type":"int128"},{"name":"j","type":"int128"},{"name":"dx","type":"uint256"}],
   "name":"get_dy","outputs":[{"type":"uint256"}],"stateMutability":"view","type":"function"},
  {"inputs":[],"name":"A","outputs":[{"type":"uint256"}],"stateMutability":"view","type":"function"},
  {"inputs":[
    {"name":"i","type":"int128"},{"name":"j","type":"int128"},{"name":"dx","type":"uint256"}],
   "name":"get_dy_underlying","outputs":[{"type":"uint256"}],"stateMutability":"view","type":"function"},
  {"inputs":[
    {"name":"i","type":"int128"},{"name":"j","type":"int128"},
    {"name":"dx","type":"uint256"},{"name":"min_dy","type":"uint256"}],
   "name":"exchange","outputs":[],"stateMutability":"nonpayable","type":"function"},
  {"inputs":[
    {"name":"i","type":"int128"},{"name":"j","type":"int128"},
    {"name":"dx","type":"uint256"},{"name":"min_dy","type":"uint256"}],
   "name":"exchange_underlying","outputs":[],"stateMutability":"nonpayable","type":"function"},
  {"anonymous":false,"inputs":[
    {"indexed":true,"name":"buyer","type":"address"},
    {"indexed":false,"name":"sold_id","type":"int128"},
    {"indexed":false,"name":"tokens_sold","type":"uint256"},
    {"indexed":false,"name":"bought_id","type":"int128"},
    {"indexed":false,"name":"tokens_bought","type":"uint256"}],"name":"TokenExchange","type":"event"},
  {"anonymous":false,"inputs":[
    {"indexed":true,"name":"buyer","type":"address"},
    {"indexed":false,"name":"sold_id","type":"int128"},
    {"indexed":false,"name":"tokens_sold","type":"uint256"},
    {"indexed":false,"name":"bought_id","type":"int128"},
    {"indexed":false,"name":"tokens_bought","type":"uint256"}],
   "name":"TokenExchangeUnderlying","type":"event"}
]`

// The CryptoSwap pool ABI, identical in shape to the StableSwap one except that
// the coin indices are uint256. The two therefore produce different selectors:
// get_dy is 0x5e0d443f on a StableSwap pool and 0x556d6e9f on a CryptoSwap one.
const cryptoPoolABIJSON = `[
  {"inputs":[
    {"name":"i","type":"uint256"},{"name":"j","type":"uint256"},{"name":"dx","type":"uint256"}],
   "name":"get_dy","outputs":[{"type":"uint256"}],"stateMutability":"view","type":"function"},
  {"inputs":[],"name":"A","outputs":[{"type":"uint256"}],"stateMutability":"view","type":"function"},
  {"inputs":[
    {"name":"i","type":"uint256"},{"name":"j","type":"uint256"},{"name":"dx","type":"uint256"}],
   "name":"get_dy_underlying","outputs":[{"type":"uint256"}],"stateMutability":"view","type":"function"},
  {"inputs":[
    {"name":"i","type":"uint256"},{"name":"j","type":"uint256"},
    {"name":"dx","type":"uint256"},{"name":"min_dy","type":"uint256"}],
   "name":"exchange","outputs":[{"type":"uint256"}],"stateMutability":"payable","type":"function"},
  {"inputs":[
    {"name":"i","type":"uint256"},{"name":"j","type":"uint256"},
    {"name":"dx","type":"uint256"},{"name":"min_dy","type":"uint256"}],
   "name":"exchange_underlying","outputs":[{"type":"uint256"}],
   "stateMutability":"payable","type":"function"},
  {"anonymous":false,"inputs":[
    {"indexed":true,"name":"buyer","type":"address"},
    {"indexed":false,"name":"sold_id","type":"uint256"},
    {"indexed":false,"name":"tokens_sold","type":"uint256"},
    {"indexed":false,"name":"bought_id","type":"uint256"},
    {"indexed":false,"name":"tokens_bought","type":"uint256"}],"name":"TokenExchange","type":"event"},
  {"anonymous":false,"inputs":[
    {"indexed":true,"name":"buyer","type":"address"},
    {"indexed":false,"name":"sold_id","type":"uint256"},
    {"indexed":false,"name":"tokens_sold","type":"uint256"},
    {"indexed":false,"name":"bought_id","type":"uint256"},
    {"indexed":false,"name":"tokens_bought","type":"uint256"}],
   "name":"TokenExchangeUnderlying","type":"event"}
]`

// The NG CryptoSwap pools append the realized fee and the packed price scale to
// TokenExchange, which changes the topic of an otherwise identical event. It
// lives in its own ABI because go-ethereum keys events by name.
const cryptoNGEventABIJSON = `[
  {"anonymous":false,"inputs":[
    {"indexed":true,"name":"buyer","type":"address"},
    {"indexed":false,"name":"sold_id","type":"uint256"},
    {"indexed":false,"name":"tokens_sold","type":"uint256"},
    {"indexed":false,"name":"bought_id","type":"uint256"},
    {"indexed":false,"name":"tokens_bought","type":"uint256"},
    {"indexed":false,"name":"fee","type":"uint256"},
    {"indexed":false,"name":"packed_price_scale","type":"uint256"}],
   "name":"TokenExchange","type":"event"}
]`

var (
	addressProviderABI = mustParseABI(addressProviderABIJSON)
	metaRegistryABI    = mustParseABI(metaRegistryABIJSON)
	stablePoolABI      = mustParseABI(stablePoolABIJSON)
	cryptoPoolABI      = mustParseABI(cryptoPoolABIJSON)
	cryptoNGEventABI   = mustParseABI(cryptoNGEventABIJSON)
)

func mustParseABI(s string) abi.ABI {
	parsed, err := abi.JSON(strings.NewReader(s))
	if err != nil {
		panic(fmt.Errorf("invalid built-in curve abi: %w", err))
	}

	return parsed
}

// PoolABI returns the pool ABI fragment of one index kind.
func PoolABI(kind IndexKind) abi.ABI {
	if kind == IndexUint256 {
		return cryptoPoolABI
	}

	return stablePoolABI
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
	case int64:
		return big.NewInt(v), nil
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

	v, ok := values[index].(bool)
	if !ok {
		return false, fmt.Errorf("unexpected output type %T at index %d", values[index], index)
	}

	return v, nil
}

// decodeAddressArray flattens a fixed [8]address output, dropping the zero
// padding that a pool with fewer coins leaves behind.
func decodeAddressArray(value any) []common.Address {
	array, ok := value.([MaxCoins]common.Address)
	if !ok {
		return nil
	}

	out := make([]common.Address, 0, MaxCoins)
	for _, address := range array {
		if address == (common.Address{}) {
			break
		}

		out = append(out, address)
	}

	return out
}

// decodeUintArray flattens a fixed [8]uint256 output to the given length.
func decodeUintArray(value any, length int) []*big.Int {
	array, ok := value.([MaxCoins]*big.Int)
	if !ok {
		return nil
	}

	if length > MaxCoins {
		length = MaxCoins
	}

	out := make([]*big.Int, 0, length)
	for i := 0; i < length; i++ {
		v := array[i]
		if v == nil {
			v = new(big.Int)
		}

		out = append(out, v)
	}

	return out
}

// decodeFeeArray flattens the fixed [10]uint256 fee output.
func decodeFeeArray(value any) []*big.Int {
	array, ok := value.([MaxFees]*big.Int)
	if !ok {
		return nil
	}

	out := make([]*big.Int, 0, MaxFees)
	for _, v := range array {
		if v == nil {
			v = new(big.Int)
		}

		out = append(out, v)
	}

	return out
}
