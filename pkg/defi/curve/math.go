package curve

import "math/big"

// PriceRatio converts an input and an output amount, both in raw base units,
// into the realized price of the input coin denominated in the output coin.
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

func pow10Float(exponent int) *big.Float {
	return new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(exponent)), nil))
}

// AmountOutMinimum applies a slippage tolerance in basis points to a quoted
// output amount, rounding down. It is the min_dy the pool enforces, and the
// pool reverts rather than filling worse.
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
