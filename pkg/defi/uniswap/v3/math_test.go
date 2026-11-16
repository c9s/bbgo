package v3

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
)

// sqrtPriceX96Of builds a sqrtPriceX96 for a raw price, which is token1 base
// units per token0 base unit.
func sqrtPriceX96Of(rawPrice float64) *big.Int {
	root := new(big.Float).Sqrt(big.NewFloat(rawPrice))
	out, _ := new(big.Float).Mul(root, new(big.Float).SetInt(Q96)).Int(nil)

	return out
}

func TestSqrtPriceX96ToPrice(t *testing.T) {
	// equal decimals: the raw ratio is the human price
	price, _ := SqrtPriceX96ToPrice(Q96, 18, 18).Float64()
	assert.InDelta(t, 1.0, price, 1e-9)

	price, _ = SqrtPriceX96ToPrice(sqrtPriceX96Of(4), 18, 18).Float64()
	assert.InDelta(t, 4.0, price, 1e-9)

	// a zero price is not a panic
	assert.Equal(t, 0.0, mustFloat(SqrtPriceX96ToPrice(nil, 18, 18)))
	assert.Equal(t, 0.0, mustFloat(SqrtPriceX96ToPrice(big.NewInt(0), 18, 18)))
}

func TestSqrtPriceX96ToPriceAppliesDecimals(t *testing.T) {
	// a USDC/WETH pool: token0 has 6 decimals, token1 has 18. The raw ratio of
	// 1 means 1e-12 in human terms, and skipping this adjustment is the classic
	// v3 decimals bug.
	price, _ := SqrtPriceX96ToPrice(Q96, 6, 18).Float64()
	assert.InDelta(t, 1e-12, price, 1e-24)

	// and the other way around
	price, _ = SqrtPriceX96ToPrice(Q96, 18, 6).Float64()
	assert.InDelta(t, 1e12, price, 1e0)

	// a realistic WBTC(8)/USDC(6) pool at 70000 USDC per WBTC: the raw ratio is
	// 70000 * 10^(6-8)
	raw := sqrtPriceX96Of(70000 * 1e-2)
	price, _ = SqrtPriceX96ToPrice(raw, 8, 6).Float64()
	assert.InEpsilon(t, 70000.0, price, 1e-6)
}

func mustFloat(f *big.Float) float64 {
	out, _ := f.Float64()
	return out
}

func TestPriceRatio(t *testing.T) {
	// 1 WETH in, 2000 USDC out
	amountIn := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	amountOut := big.NewInt(2000_000000)

	price, _ := PriceRatio(amountIn, amountOut, 18, 6).Float64()
	assert.InEpsilon(t, 2000.0, price, 1e-9)

	// and the inverse direction
	price, _ = PriceRatio(amountOut, amountIn, 6, 18).Float64()
	assert.InEpsilon(t, 1.0/2000.0, price, 1e-9)

	assert.Equal(t, 0.0, mustFloat(PriceRatio(nil, amountOut, 18, 6)))
	assert.Equal(t, 0.0, mustFloat(PriceRatio(big.NewInt(0), amountOut, 18, 6)))
}

func TestAmountOutMinimum(t *testing.T) {
	amountOut := big.NewInt(1_000_000)

	assert.Equal(t, big.NewInt(995_000), AmountOutMinimum(amountOut, 50))
	assert.Equal(t, big.NewInt(900_000), AmountOutMinimum(amountOut, 1000))

	// no tolerance means the full quote
	assert.Equal(t, amountOut, AmountOutMinimum(amountOut, 0))
	assert.Equal(t, amountOut, AmountOutMinimum(amountOut, -1))

	// a tolerance of 100% or more leaves no floor at all
	assert.Equal(t, 0, AmountOutMinimum(amountOut, 10000).Sign())

	// rounding is down, so the floor is never above the tolerance: half of 3
	// floors to 1
	assert.Equal(t, big.NewInt(1), AmountOutMinimum(big.NewInt(3), 5000))

	assert.Equal(t, 0, AmountOutMinimum(nil, 50).Sign())
}

func TestFeeTierFraction(t *testing.T) {
	assert.Equal(t, 0.0001, FeeTierFraction(FeeTier100))
	assert.Equal(t, 0.0005, FeeTierFraction(FeeTier500))
	assert.Equal(t, 0.0025, FeeTierFraction(FeeTier2500))
	assert.Equal(t, 0.003, FeeTierFraction(FeeTier3000))
	assert.Equal(t, 0.01, FeeTierFraction(FeeTier10000))
}

func TestQuoteHelpers(t *testing.T) {
	quote := &Quote{
		AmountIn:  new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil),
		AmountOut: big.NewInt(2000_000000),
		Fee:       FeeTier500,
	}

	price, _ := quote.Price(18, 6).Float64()
	assert.InEpsilon(t, 2000.0, price, 1e-9)
	assert.Equal(t, big.NewInt(1990_000000), quote.AmountOutMinimum(50))
}

func TestPoolHelpers(t *testing.T) {
	pool := &Pool{
		Token0:       testUSDC,
		Token1:       testWETH,
		SqrtPriceX96: Q96,
		Fee:          FeeTier500,
		Liquidity:    big.NewInt(1),
	}

	assert.True(t, pool.Contains(testUSDC, testWETH))
	assert.True(t, pool.Contains(testWETH, testUSDC))
	assert.False(t, pool.Contains(testUSDC, testDAI))

	price, _ := pool.Price(6, 18).Float64()
	assert.InDelta(t, 1e-12, price, 1e-24)
}
