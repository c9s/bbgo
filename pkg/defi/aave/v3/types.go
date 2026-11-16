package v3

import (
	"fmt"
	"math"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// Addresses is the resolved layout of one Aave v3 market.
type Addresses struct {
	Provider     common.Address
	Pool         common.Address
	DataProvider common.Address
	Oracle       common.Address

	MarketID string

	// BaseCurrency is the asset the oracle prices everything in. It is the zero
	// address when the base currency is USD, which is the v3 default.
	BaseCurrency common.Address

	// BaseCurrencyUnit is the scale of every "base" amount, 1e8 for USD.
	BaseCurrencyUnit *big.Int
}

// BaseCurrencyDecimals is the number of decimals of the base currency scale.
func (a Addresses) BaseCurrencyDecimals() int {
	if a.BaseCurrencyUnit == nil || a.BaseCurrencyUnit.Sign() <= 0 {
		return 0
	}

	return len(a.BaseCurrencyUnit.String()) - 1
}

func (a Addresses) String() string {
	return fmt.Sprintf("aave v3 market %q: pool=%s dataProvider=%s oracle=%s baseUnit=%s",
		a.MarketID, a.Pool.Hex(), a.DataProvider.Hex(), a.Oracle.Hex(), a.BaseCurrencyUnit)
}

// AccountData is the account level risk view returned by
// Pool.getUserAccountData. Every amount is in the oracle base currency, scaled
// by Addresses.BaseCurrencyUnit, and not in token units.
type AccountData struct {
	TotalCollateralBase  *big.Int
	TotalDebtBase        *big.Int
	AvailableBorrowsBase *big.Int

	LiquidationThresholdBps *big.Int
	LTVBps                  *big.Int

	// HealthFactor is WAD scaled and is max uint256 when there is no debt.
	HealthFactor *big.Int

	baseCurrencyUnit *big.Int
}

// HasDebt reports whether the account currently borrows anything.
func (a *AccountData) HasDebt() bool {
	return a.TotalDebtBase != nil && a.TotalDebtBase.Sign() > 0
}

// HealthFactorFloat returns the health factor as a float, and +Inf for an
// account with no debt, which is how Aave encodes "no liquidation risk".
func (a *AccountData) HealthFactorFloat() float64 {
	if a.HealthFactor == nil {
		return 0
	}

	if a.HealthFactor.Cmp(maxUint256) == 0 {
		return math.Inf(1)
	}

	return WadToFloat(a.HealthFactor)
}

// IsLiquidatable reports whether the health factor is below one, the point at
// which the position can be liquidated.
func (a *AccountData) IsLiquidatable() bool {
	return a.HasDebt() && a.HealthFactorFloat() < 1
}

func (a *AccountData) baseToFloat(value *big.Int) float64 {
	if value == nil || a.baseCurrencyUnit == nil || a.baseCurrencyUnit.Sign() <= 0 {
		return 0
	}

	out, _ := new(big.Float).Quo(new(big.Float).SetInt(value), new(big.Float).SetInt(a.baseCurrencyUnit)).Float64()

	return out
}

// TotalCollateral returns the collateral in base currency units.
func (a *AccountData) TotalCollateral() float64 {
	return a.baseToFloat(a.TotalCollateralBase)
}

// TotalDebt returns the debt in base currency units.
func (a *AccountData) TotalDebt() float64 {
	return a.baseToFloat(a.TotalDebtBase)
}

// AvailableBorrows returns the remaining borrowing power in base currency units.
func (a *AccountData) AvailableBorrows() float64 {
	return a.baseToFloat(a.AvailableBorrowsBase)
}

func (a *AccountData) String() string {
	return fmt.Sprintf(
		"collateral=%.2f debt=%.2f available=%.2f ltv=%.2f%% liquidationThreshold=%.2f%% healthFactor=%.4f",
		a.TotalCollateral(), a.TotalDebt(), a.AvailableBorrows(),
		BpsToFloat(a.LTVBps)*100, BpsToFloat(a.LiquidationThresholdBps)*100, a.HealthFactorFloat())
}

// Reserve is the market wide state of one asset.
type Reserve struct {
	Asset common.Address

	Unbacked                *big.Int
	AccruedToTreasuryScaled *big.Int
	TotalAToken             *big.Int
	TotalStableDebt         *big.Int
	TotalVariableDebt       *big.Int

	// LiquidityRate and VariableBorrowRate are RAY scaled per-second rates.
	LiquidityRate           *big.Int
	VariableBorrowRate      *big.Int
	StableBorrowRate        *big.Int
	AverageStableBorrowRate *big.Int

	LiquidityIndex      *big.Int
	VariableBorrowIndex *big.Int

	LastUpdated time.Time
}

// SupplyAPY is the effective annual yield of supplying this asset.
func (r *Reserve) SupplyAPY() float64 {
	return APY(r.LiquidityRate)
}

// BorrowAPY is the effective annual cost of borrowing this asset at the
// variable rate.
func (r *Reserve) BorrowAPY() float64 {
	return APY(r.VariableBorrowRate)
}

// TotalDebt is the sum of the variable and the legacy stable debt.
func (r *Reserve) TotalDebt() *big.Int {
	total := new(big.Int)

	if r.TotalVariableDebt != nil {
		total.Add(total, r.TotalVariableDebt)
	}

	if r.TotalStableDebt != nil {
		total.Add(total, r.TotalStableDebt)
	}

	return total
}

// UserReserve is one account's position in one reserve. The balances are in
// token units of the asset.
type UserReserve struct {
	Asset common.Address

	// ATokenBalance is the rebased balance, which already includes the accrued
	// interest.
	ATokenBalance *big.Int

	StableDebt          *big.Int
	VariableDebt        *big.Int
	PrincipalStableDebt *big.Int
	ScaledVariableDebt  *big.Int

	StableBorrowRate *big.Int
	LiquidityRate    *big.Int

	UsageAsCollateralEnabled bool
}

// TotalDebt is the sum of the variable and the legacy stable debt.
func (u *UserReserve) TotalDebt() *big.Int {
	total := new(big.Int)

	if u.VariableDebt != nil {
		total.Add(total, u.VariableDebt)
	}

	if u.StableDebt != nil {
		total.Add(total, u.StableDebt)
	}

	return total
}

// IsEmpty reports whether the account has neither a balance nor a debt here.
func (u *UserReserve) IsEmpty() bool {
	return (u.ATokenBalance == nil || u.ATokenBalance.Sign() == 0) && u.TotalDebt().Sign() == 0
}

// ReserveConfig is the risk configuration of one reserve.
type ReserveConfig struct {
	Asset common.Address

	Decimals int

	LTVBps                  *big.Int
	LiquidationThresholdBps *big.Int
	LiquidationBonusBps     *big.Int
	ReserveFactorBps        *big.Int

	UsageAsCollateralEnabled bool
	BorrowingEnabled         bool
	StableBorrowRateEnabled  bool
	IsActive                 bool
	IsFrozen                 bool
}

// ReserveTokens are the three token contracts Aave deploys per reserve.
type ReserveTokens struct {
	AToken            common.Address
	StableDebtToken   common.Address
	VariableDebtToken common.Address
}
