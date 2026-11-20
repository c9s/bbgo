package curve

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// IndexKind is how a pool types its coin indices. It is the one piece of Curve
// ABI variation that the registry does not report, so it is detected by calling
// get_dy both ways and keeping the one that answers.
type IndexKind int

const (
	// IndexUnknown means the kind has not been detected yet.
	IndexUnknown IndexKind = iota

	// IndexInt128 is the StableSwap family, including StableSwap-NG.
	IndexInt128

	// IndexUint256 is the CryptoSwap family: twocrypto, tricrypto and their NG
	// versions.
	IndexUint256
)

func (k IndexKind) String() string {
	switch k {
	case IndexInt128:
		return "stableswap"
	case IndexUint256:
		return "cryptoswap"
	default:
		return "unknown"
	}
}

// Pool is the metadata of one Curve pool as the MetaRegistry reports it.
//
// A field the registry's handler for that pool cannot answer is left at its
// zero value rather than failing the read: get_pool_name reverts for the NG
// crypto pools and get_A reverts for every crypto pool, since a CryptoSwap pool
// has no single amplification coefficient.
type Pool struct {
	Address common.Address

	// Name is the registry's name for the pool, which is empty when the
	// registry has none and is sometimes a raw hex string for a
	// factory-deployed pool.
	Name string

	// IsMeta reports whether the pool holds the LP token of another pool.
	IsMeta   bool
	BasePool common.Address
	LPToken  common.Address

	// Coins are the pool's own coins, which for a metapool include the base
	// pool's LP token.
	Coins    []common.Address
	Decimals []int
	Balances []*big.Int

	// UnderlyingCoins expands a metapool's LP token into the base pool's coins.
	// It is empty for a plain pool.
	UnderlyingCoins    []common.Address
	UnderlyingDecimals []int
	UnderlyingBalances []*big.Int

	// Fees is Curve's fee array, 1e10 denominated. The first entry is the swap
	// fee and the second the admin fee; a CryptoSwap pool adds mid_fee and
	// out_fee, between which its fee moves with the pool's balance.
	Fees []*big.Int

	// A is the amplification coefficient as the pool reports it, nil for a
	// CryptoSwap pool.
	A *big.Int

	// Index is the pool's coin index type, IndexUnknown until a quote has
	// detected it.
	Index IndexKind
}

// Fee returns the swap fee as a fraction, so 0.0004 for a 0.04% pool.
func (p *Pool) Fee() float64 {
	if len(p.Fees) == 0 {
		return 0
	}

	return FeeFraction(p.Fees[0])
}

func (p *Pool) String() string {
	name := p.Name
	if name == "" {
		name = p.Address.Hex()
	}

	return fmt.Sprintf("curve pool %s (%s, %d coins, fee %.4f%%)",
		name, p.Address.Hex(), len(p.Coins), p.Fee()*100)
}

// Route is one pool and the way to trade one ordered pair through it: the two
// coin indices, whether the pair is reached through the metapool's underlying
// coins, and which index type the pool's ABI uses.
//
// It is what a quote and a swap are both addressed to, because on Curve the
// pool and the index pair together are the venue.
type Route struct {
	Pool common.Address

	TokenIn  common.Address
	TokenOut common.Address

	I int64
	J int64

	// Underlying selects exchange_underlying over exchange, which is how a
	// metapool trades a coin of its base pool.
	Underlying bool

	Index IndexKind

	// OutBalance is the pool's balance of the output coin, in that coin's own
	// base units, and OutBalanceExact says whether it can be read as such.
	//
	// For a direct route it comes from get_balances and is exact: a pool cannot
	// pay out more of a coin than it holds, so it is a hard upper bound on the
	// swap and therefore a sound way to order the pools worth quoting.
	//
	// For an underlying route it comes from get_underlying_balances, which the
	// lending pool handlers do not convert: the busd pool returns exactly the
	// same array as get_balances, so the number is the wrapped coin's balance
	// in the wrapped coin's own scale. Read with the underlying coin's
	// decimals, that lands orders of magnitude out, so such a route is only
	// ordered by it, never bounded by it.
	OutBalance      *big.Int
	OutBalanceExact bool
}

// Method is the name of the pool function this route swaps through.
func (r *Route) Method() string {
	if r.Underlying {
		return "exchange_underlying"
	}

	return "exchange"
}

// QuoteMethod is the name of the pool function this route quotes with.
func (r *Route) QuoteMethod() string {
	if r.Underlying {
		return "get_dy_underlying"
	}

	return "get_dy"
}

func (r *Route) String() string {
	return fmt.Sprintf("%s(%d, %d) on %s [%s]",
		r.Method(), r.I, r.J, r.Pool.Hex(), r.Index)
}

// Quote is what a pool reports it would return for one exact-input swap. It is
// the pool's own get_dy, so it already includes the fee and the price impact of
// this size.
type Quote struct {
	Route *Route

	AmountIn  *big.Int
	AmountOut *big.Int
}

// Price returns the realized price of the input coin denominated in the output
// coin.
func (q *Quote) Price(decimalsIn, decimalsOut int) *big.Float {
	return PriceRatio(q.AmountIn, q.AmountOut, decimalsIn, decimalsOut)
}

// AmountOutMinimum applies a slippage tolerance to this quote, which is the
// min_dy the pool will enforce.
func (q *Quote) AmountOutMinimum(slippageBps int) *big.Int {
	return AmountOutMinimum(q.AmountOut, slippageBps)
}

func (q *Quote) String() string {
	return fmt.Sprintf("quote in=%s out=%s via %s", q.AmountIn, q.AmountOut, q.Route)
}

// SwapRequest describes one exact-input swap through one pool.
//
// There is no recipient: the four argument form of exchange, the only one every
// pool implementation has, always pays out to the caller.
type SwapRequest struct {
	TokenIn  common.Address
	TokenOut common.Address

	// AmountIn is in raw base units of TokenIn.
	AmountIn *big.Int

	// Pool pins the swap to one pool. The zero address asks the client to pick
	// the pool with the best quote for this size.
	Pool common.Address

	// SlippageBps bounds the shortfall against the quote. It is required: a
	// swap without a minimum output is a swap that can be sandwiched for
	// everything it is worth.
	SlippageBps int

	// AmountOutMinimum overrides the slippage calculation when non-nil.
	AmountOutMinimum *big.Int
}
