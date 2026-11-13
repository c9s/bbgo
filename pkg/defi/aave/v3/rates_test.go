package v3

import (
	"math"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
)

func ray(f float64) *big.Int {
	out, _ := new(big.Float).Mul(big.NewFloat(f), new(big.Float).SetInt(RAY)).Int(nil)
	return out
}

func wad(f float64) *big.Int {
	out, _ := new(big.Float).Mul(big.NewFloat(f), new(big.Float).SetInt(WAD)).Int(nil)
	return out
}

func TestRayToFloat(t *testing.T) {
	assert.Equal(t, 0.0, RayToFloat(nil))
	assert.Equal(t, 1.0, RayToFloat(RAY))
	assert.InDelta(t, 0.05, RayToFloat(ray(0.05)), 1e-12)
}

func TestWadToFloat(t *testing.T) {
	assert.Equal(t, 1.0, WadToFloat(WAD))
	assert.InDelta(t, 1.2345, WadToFloat(wad(1.2345)), 1e-12)
}

func TestBpsToFloat(t *testing.T) {
	assert.Equal(t, 0.0, BpsToFloat(nil))
	assert.Equal(t, 0.8, BpsToFloat(big.NewInt(8000)))
	assert.Equal(t, 1.0, BpsToFloat(big.NewInt(10000)))
}

func TestAPY(t *testing.T) {
	assert.Equal(t, 0.0, APY(big.NewInt(0)))
	assert.Equal(t, 0.0, APY(nil))

	// 5% nominal, compounded every second, is slightly above 5% effective
	apr := ray(0.05)
	assert.InDelta(t, 0.05, APR(apr), 1e-12)

	apy := APY(apr)
	assert.Greater(t, apy, APR(apr))
	assert.InDelta(t, math.Exp(0.05)-1, apy, 1e-6, "per-second compounding approaches the continuous limit")
}

func TestAPYIsMonotonic(t *testing.T) {
	previous := 0.0
	for _, rate := range []float64{0.001, 0.01, 0.05, 0.2, 1.0} {
		apy := APY(ray(rate))
		assert.Greater(t, apy, previous)
		previous = apy
	}
}

func TestAccountDataHealthFactor(t *testing.T) {
	// no debt: the health factor is max uint256, which reads as infinite
	account := &AccountData{
		TotalDebtBase:    big.NewInt(0),
		HealthFactor:     new(big.Int).Set(maxUint256),
		baseCurrencyUnit: big.NewInt(1e8),
	}

	assert.False(t, account.HasDebt())
	assert.True(t, math.IsInf(account.HealthFactorFloat(), 1))
	assert.False(t, account.IsLiquidatable())

	// a healthy position
	account = &AccountData{
		TotalCollateralBase: big.NewInt(200_00000000),
		TotalDebtBase:       big.NewInt(100_00000000),
		HealthFactor:        wad(1.6),
		LTVBps:              big.NewInt(7500),
		baseCurrencyUnit:    big.NewInt(1e8),
	}

	assert.True(t, account.HasDebt())
	assert.InDelta(t, 1.6, account.HealthFactorFloat(), 1e-12)
	assert.False(t, account.IsLiquidatable())
	assert.Equal(t, 200.0, account.TotalCollateral())
	assert.Equal(t, 100.0, account.TotalDebt())

	// below one, the position can be liquidated
	account.HealthFactor = wad(0.98)
	assert.True(t, account.IsLiquidatable())
}

func TestAddressesBaseCurrencyDecimals(t *testing.T) {
	assert.Equal(t, 8, Addresses{BaseCurrencyUnit: big.NewInt(1e8)}.BaseCurrencyDecimals())
	assert.Equal(t, 18, Addresses{BaseCurrencyUnit: WAD}.BaseCurrencyDecimals())
	assert.Equal(t, 0, Addresses{}.BaseCurrencyDecimals())
}

func TestReserveAPY(t *testing.T) {
	reserve := &Reserve{
		LiquidityRate:      ray(0.03),
		VariableBorrowRate: ray(0.05),
		TotalVariableDebt:  big.NewInt(100),
		TotalStableDebt:    big.NewInt(5),
	}

	assert.Greater(t, reserve.BorrowAPY(), reserve.SupplyAPY())
	assert.Equal(t, big.NewInt(105), reserve.TotalDebt())
}

func TestUserReserveIsEmpty(t *testing.T) {
	assert.True(t, (&UserReserve{}).IsEmpty())
	assert.True(t, (&UserReserve{ATokenBalance: big.NewInt(0)}).IsEmpty())
	assert.False(t, (&UserReserve{ATokenBalance: big.NewInt(1)}).IsEmpty())
	assert.False(t, (&UserReserve{VariableDebt: big.NewInt(1)}).IsEmpty())
}
