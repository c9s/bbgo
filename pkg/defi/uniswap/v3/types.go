package v3

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// Pool is the state of one v3 pool.
type Pool struct {
	Address common.Address

	Token0 common.Address
	Token1 common.Address

	// Fee is the tier in hundredths of a basis point.
	Fee uint32

	TickSpacing int32

	SqrtPriceX96 *big.Int
	Tick         int32

	// Liquidity is the in-range liquidity, which is not a token amount and is
	// only comparable between pools of the same pair.
	Liquidity *big.Int

	Unlocked bool
}

// Price returns the price of token0 denominated in token1, in human units.
func (p *Pool) Price(decimals0, decimals1 int) *big.Float {
	return SqrtPriceX96ToPrice(p.SqrtPriceX96, decimals0, decimals1)
}

// Contains reports whether the pool trades the given pair, in either order.
func (p *Pool) Contains(tokenA, tokenB common.Address) bool {
	return (p.Token0 == tokenA && p.Token1 == tokenB) || (p.Token0 == tokenB && p.Token1 == tokenA)
}

func (p *Pool) String() string {
	return fmt.Sprintf("pool %s fee=%.2f%% tick=%d liquidity=%s",
		p.Address.Hex(), FeeTierFraction(p.Fee)*100, p.Tick, p.Liquidity)
}

// Quote is the result of a QuoterV2 call. Amounts are raw base units.
type Quote struct {
	TokenIn  common.Address
	TokenOut common.Address

	Fee uint32

	AmountIn  *big.Int
	AmountOut *big.Int

	SqrtPriceX96After       *big.Int
	InitializedTicksCrossed uint32

	// GasEstimate is the quoter's own estimate of the swap gas, which is a
	// rough number and not what the transaction layer uses.
	GasEstimate *big.Int
}

// Price returns the realized price of the input token denominated in the output
// token, which already includes the fee and the price impact of this size.
func (q *Quote) Price(decimalsIn, decimalsOut int) *big.Float {
	return PriceRatio(q.AmountIn, q.AmountOut, decimalsIn, decimalsOut)
}

// AmountOutMinimum applies a slippage tolerance to this quote.
func (q *Quote) AmountOutMinimum(slippageBps int) *big.Int {
	return AmountOutMinimum(q.AmountOut, slippageBps)
}

func (q *Quote) String() string {
	return fmt.Sprintf("quote %s -> %s fee=%.2f%% in=%s out=%s ticksCrossed=%d",
		q.TokenIn.Hex(), q.TokenOut.Hex(), FeeTierFraction(q.Fee)*100,
		q.AmountIn, q.AmountOut, q.InitializedTicksCrossed)
}

// SwapRequest describes one exact-input single-hop swap.
type SwapRequest struct {
	TokenIn  common.Address
	TokenOut common.Address

	// Fee selects the pool. Zero asks the client to pick the deepest pool.
	Fee uint32

	// AmountIn is in raw base units of TokenIn.
	AmountIn *big.Int

	// SlippageBps bounds the shortfall against the quote. It is required: a
	// swap without a minimum output is a swap that can be sandwiched for
	// everything it is worth.
	SlippageBps int

	// AmountOutMinimum overrides the slippage calculation when non-nil.
	AmountOutMinimum *big.Int

	// Recipient defaults to the sender.
	Recipient common.Address
}
