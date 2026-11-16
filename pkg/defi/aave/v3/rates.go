package v3

import (
	"math"
	"math/big"
)

// Aave fixed-point scales.
var (
	// RAY is 1e27, the scale of the interest rates and the liquidity indexes.
	RAY = new(big.Int).Exp(big.NewInt(10), big.NewInt(27), nil)

	// WAD is 1e18, the scale of the health factor.
	WAD = new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)

	maxUint256 = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
)

// SecondsPerYear is the constant Aave uses to compound the per-second rates.
const SecondsPerYear = 31536000

func scaledToFloat(value, scale *big.Int) float64 {
	if value == nil {
		return 0
	}

	out, _ := new(big.Float).Quo(new(big.Float).SetInt(value), new(big.Float).SetInt(scale)).Float64()

	return out
}

// RayToFloat converts a RAY scaled value to a float fraction.
func RayToFloat(ray *big.Int) float64 {
	return scaledToFloat(ray, RAY)
}

// WadToFloat converts a WAD scaled value to a float fraction.
func WadToFloat(wad *big.Int) float64 {
	return scaledToFloat(wad, WAD)
}

// BpsToFloat converts basis points to a float fraction, so 8000 becomes 0.8.
func BpsToFloat(bps *big.Int) float64 {
	if bps == nil {
		return 0
	}

	out, _ := new(big.Float).Quo(new(big.Float).SetInt(bps), big.NewFloat(10000)).Float64()

	return out
}

// APR converts a RAY scaled per-second rate to its nominal annual rate.
func APR(ray *big.Int) float64 {
	return RayToFloat(ray)
}

// APY converts a RAY scaled per-second rate to the effective annual yield,
// compounding every second the way Aave's own interface reports it.
func APY(ray *big.Int) float64 {
	apr := APR(ray)
	if apr == 0 {
		return 0
	}

	return math.Pow(1+apr/SecondsPerYear, SecondsPerYear) - 1
}
