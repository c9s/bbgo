package v3

import (
	"math/big"
)

// Q96 is the 2^96 scale of sqrtPriceX96.
var Q96 = new(big.Int).Lsh(big.NewInt(1), 96)

var q96Float = new(big.Float).SetInt(Q96)

// SqrtPriceX96ToPrice converts a pool's sqrtPriceX96 into the price of token0
// denominated in token1, in human units.
//
// The raw ratio is (sqrtPriceX96 / 2^96)^2, which is token1 base units per
// token0 base unit; the decimals of the two tokens then scale it by
// 10^(decimals0 - decimals1). Skipping that adjustment is the classic Uniswap
// v3 bug: a WBTC/USDC pool reads as 1e-12 off, because WBTC has 8 decimals and
// USDC has 6.
func SqrtPriceX96ToPrice(sqrtPriceX96 *big.Int, decimals0, decimals1 int) *big.Float {
	if sqrtPriceX96 == nil || sqrtPriceX96.Sign() == 0 {
		return new(big.Float)
	}

	ratio := new(big.Float).Quo(new(big.Float).SetInt(sqrtPriceX96), q96Float)
	price := new(big.Float).Mul(ratio, ratio)

	if exponent := decimals0 - decimals1; exponent > 0 {
		price.Mul(price, pow10Float(exponent))
	} else if exponent < 0 {
		price.Quo(price, pow10Float(-exponent))
	}

	return price
}

func pow10Float(exponent int) *big.Float {
	return new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(exponent)), nil))
}

// PriceRatio converts an input and an output amount, both in raw base units,
// into the realized price of the input token denominated in the output token.
func PriceRatio(amountIn, amountOut *big.Int, decimalsIn, decimalsOut int) *big.Float {
	if amountIn == nil || amountIn.Sign() == 0 || amountOut == nil {
		return new(big.Float)
	}

	price := new(big.Float).Quo(new(big.Float).SetInt(amountOut), new(big.Float).SetInt(amountIn))

	if exponent := decimalsIn - decimalsOut; exponent > 0 {
		price.Mul(price, pow10Float(exponent))
	} else if exponent < 0 {
		price.Quo(price, pow10Float(-exponent))
	}

	return price
}

// AmountOutMinimum applies a slippage tolerance in basis points to a quoted
// output amount, rounding down, which is the value the router enforces.
func AmountOutMinimum(amountOut *big.Int, slippageBps int) *big.Int {
	if amountOut == nil {
		return new(big.Int)
	}

	if slippageBps <= 0 {
		return new(big.Int).Set(amountOut)
	}

	if slippageBps >= 10000 {
		return new(big.Int)
	}

	out := new(big.Int).Mul(amountOut, big.NewInt(int64(10000-slippageBps)))

	return out.Div(out, big.NewInt(10000))
}

// FeeTierFraction converts a fee tier into a fraction, so 3000 becomes 0.003.
func FeeTierFraction(fee uint32) float64 {
	return float64(fee) / 1e6
}
