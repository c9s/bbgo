package autoborrow

import (
	"fmt"
	"sort"
	"time"

	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/currency"
)

// InterestBudgetConfig defines the interest budget risk control.
//
// When the projected monthly interest (the current debts multiplied by the latest
// hourly interest rate of each asset over the whole month) exceeds MonthlyBudget,
// autoborrow stops borrowing. A borrow that would push the projection over the
// budget is reduced to the amount that still fits in the budget.
type InterestBudgetConfig struct {
	// MonthlyBudget is the max interest (in QuoteCurrency) we are willing to pay in a month.
	MonthlyBudget fixedpoint.Value `json:"monthlyBudget"`

	// QuoteCurrency is the currency used for valuing debts and interest, default USDT.
	QuoteCurrency string `json:"quoteCurrency"`
}

func (c *InterestBudgetConfig) quoteCurrency() string {
	if c.QuoteCurrency == "" {
		return currency.USDT
	}
	return c.QuoteCurrency
}

// DebtInterestEstimation is the interest estimation of a single debt asset.
// All the interest and value fields are in the quote currency.
type DebtInterestEstimation struct {
	Asset              string
	Debt               fixedpoint.Value
	Price              fixedpoint.Value
	DebtValue          fixedpoint.Value
	HourlyRate         fixedpoint.Value
	HourlyInterest     fixedpoint.Value
	InterestToMonthEnd fixedpoint.Value
	MonthlyInterest    fixedpoint.Value
}

// InterestEstimation is the interest estimation of all debts at a given time.
type InterestEstimation struct {
	Time          time.Time
	QuoteCurrency string

	// HoursToMonthEnd is the number of hours from Time to the beginning of the next month.
	HoursToMonthEnd fixedpoint.Value

	// HoursInMonth is the total number of hours of the month of Time.
	HoursInMonth fixedpoint.Value

	Assets []DebtInterestEstimation

	TotalDebtValue          fixedpoint.Value
	TotalHourlyInterest     fixedpoint.Value
	TotalInterestToMonthEnd fixedpoint.Value

	// TotalMonthlyInterest is the projected interest of a whole month at the current debts and rates.
	TotalMonthlyInterest fixedpoint.Value

	// MissingRates are the debt assets without an interest rate, they are excluded from the totals.
	MissingRates []string

	// MissingPrices are the debt assets without a price, they are excluded from the totals.
	MissingPrices []string
}

func beginningOfNextMonth(now time.Time) time.Time {
	y, m, _ := now.Date()
	return time.Date(y, m+1, 1, 0, 0, 0, 0, now.Location())
}

// hoursUntilEndOfMonth returns the hours from now to the beginning of the next month.
func hoursUntilEndOfMonth(now time.Time) fixedpoint.Value {
	return fixedpoint.NewFromFloat(beginningOfNextMonth(now).Sub(now).Hours())
}

// hoursInMonth returns the total hours of the month of now.
func hoursInMonth(now time.Time) fixedpoint.Value {
	y, m, _ := now.Date()
	begin := time.Date(y, m, 1, 0, 0, 0, 0, now.Location())
	return fixedpoint.NewFromFloat(beginningOfNextMonth(now).Sub(begin).Hours())
}

// estimateInterest estimates the interest of the given debts with the latest hourly
// interest rates. prices maps asset to its price in the quote currency; the quote
// currency itself is always priced at 1.
func estimateInterest(
	now time.Time,
	quoteCurrency string,
	debts types.BalanceMap,
	rates types.MarginNextHourlyInterestRateMap,
	prices map[string]fixedpoint.Value,
) *InterestEstimation {
	est := &InterestEstimation{
		Time:                    now,
		QuoteCurrency:           quoteCurrency,
		HoursToMonthEnd:         hoursUntilEndOfMonth(now),
		HoursInMonth:            hoursInMonth(now),
		TotalDebtValue:          fixedpoint.Zero,
		TotalHourlyInterest:     fixedpoint.Zero,
		TotalInterestToMonthEnd: fixedpoint.Zero,
		TotalMonthlyInterest:    fixedpoint.Zero,
	}

	assets := make([]string, 0, len(debts))
	for asset := range debts {
		assets = append(assets, asset)
	}
	sort.Strings(assets)

	for _, asset := range assets {
		debt := debts[asset].Debt()
		if debt.Sign() <= 0 {
			continue
		}

		rate, ok := rates[asset]
		if !ok || rate == nil {
			est.MissingRates = append(est.MissingRates, asset)
			continue
		}

		price, ok := lookupPrice(prices, asset, quoteCurrency)
		if !ok {
			est.MissingPrices = append(est.MissingPrices, asset)
			continue
		}

		debtValue := debt.Mul(price)
		hourlyInterest := debtValue.Mul(rate.HourlyRate)
		e := DebtInterestEstimation{
			Asset:              asset,
			Debt:               debt,
			Price:              price,
			DebtValue:          debtValue,
			HourlyRate:         rate.HourlyRate,
			HourlyInterest:     hourlyInterest,
			InterestToMonthEnd: hourlyInterest.Mul(est.HoursToMonthEnd),
			MonthlyInterest:    hourlyInterest.Mul(est.HoursInMonth),
		}

		est.Assets = append(est.Assets, e)
		est.TotalDebtValue = est.TotalDebtValue.Add(e.DebtValue)
		est.TotalHourlyInterest = est.TotalHourlyInterest.Add(e.HourlyInterest)
		est.TotalInterestToMonthEnd = est.TotalInterestToMonthEnd.Add(e.InterestToMonthEnd)
		est.TotalMonthlyInterest = est.TotalMonthlyInterest.Add(e.MonthlyInterest)
	}

	return est
}

func lookupPrice(prices map[string]fixedpoint.Value, asset, quoteCurrency string) (fixedpoint.Value, bool) {
	if asset == quoteCurrency {
		return fixedpoint.One, true
	}

	price, ok := prices[asset]
	if !ok || price.Sign() <= 0 {
		return fixedpoint.Zero, false
	}

	return price, true
}

// maxBorrowWithinBudget returns the max quantity of an asset that can be borrowed
// without making the projected monthly interest exceed the budget.
// It returns zero when the budget is already used up.
func maxBorrowWithinBudget(
	projectedMonthlyInterest, monthlyBudget, price, hourlyRate, hoursInMonth fixedpoint.Value,
) fixedpoint.Value {
	remaining := monthlyBudget.Sub(projectedMonthlyInterest)
	if remaining.Sign() <= 0 {
		return fixedpoint.Zero
	}

	monthlyInterestPerUnit := price.Mul(hourlyRate).Mul(hoursInMonth)
	if monthlyInterestPerUnit.Sign() <= 0 {
		// free to borrow, no interest is charged
		return fixedpoint.PosInf
	}

	return remaining.Div(monthlyInterestPerUnit)
}

// interestBudgetGuard limits the borrow quantity by the monthly interest budget.
// It holds no I/O dependency so that it can be tested with plain data.
type interestBudgetGuard struct {
	budget     fixedpoint.Value
	estimation *InterestEstimation
	rates      types.MarginNextHourlyInterestRateMap
	prices     map[string]fixedpoint.Value
}

func newInterestBudgetGuard(
	budget fixedpoint.Value,
	estimation *InterestEstimation,
	rates types.MarginNextHourlyInterestRateMap,
	prices map[string]fixedpoint.Value,
) *interestBudgetGuard {
	return &interestBudgetGuard{
		budget:     budget,
		estimation: estimation,
		rates:      rates,
		prices:     prices,
	}
}

// Exceeded returns true when the projected monthly interest reaches the budget.
func (g *interestBudgetGuard) Exceeded() bool {
	return g.estimation.TotalMonthlyInterest.Compare(g.budget) >= 0
}

// Limit returns the quantity of the asset that can be borrowed within the budget, capped to the given quantity.
// An asset without a known rate or price can not be borrowed since its cost is unknown.
func (g *interestBudgetGuard) Limit(asset string, quantity fixedpoint.Value) (fixedpoint.Value, error) {
	rate, ok := g.rates[asset]
	if !ok || rate == nil {
		return fixedpoint.Zero, fmt.Errorf("interest rate of %s is unknown", asset)
	}

	price, ok := lookupPrice(g.prices, asset, g.estimation.QuoteCurrency)
	if !ok {
		return fixedpoint.Zero, fmt.Errorf("price of %s is unknown", asset)
	}

	maxQuantity := maxBorrowWithinBudget(
		g.estimation.TotalMonthlyInterest, g.budget, price, rate.HourlyRate, g.estimation.HoursInMonth,
	)

	return fixedpoint.Min(quantity, maxQuantity), nil
}

// AddBorrow adds the borrowed quantity into the projected monthly interest.
func (g *interestBudgetGuard) AddBorrow(asset string, quantity fixedpoint.Value) {
	rate, ok := g.rates[asset]
	if !ok || rate == nil {
		return
	}

	price, ok := lookupPrice(g.prices, asset, g.estimation.QuoteCurrency)
	if !ok {
		return
	}

	monthlyInterest := quantity.Mul(price).Mul(rate.HourlyRate).Mul(g.estimation.HoursInMonth)
	g.estimation.TotalMonthlyInterest = g.estimation.TotalMonthlyInterest.Add(monthlyInterest)
}

// InterestBudgetAlert is sent when the interest budget stops or limits borrowing.
type InterestBudgetAlert struct {
	SessionName string
	Budget      fixedpoint.Value
	Estimation  *InterestEstimation
	Message     string
}

func (a *InterestBudgetAlert) SlackAttachment() slack.Attachment {
	est := a.Estimation
	quote := est.QuoteCurrency

	fields := []slack.AttachmentField{
		{Title: "Session", Value: a.SessionName, Short: true},
		{Title: "Monthly Budget", Value: a.Budget.String() + " " + quote, Short: true},
		{Title: "Total Debt Value", Value: est.TotalDebtValue.String() + " " + quote, Short: true},
		{Title: "Hourly Interest", Value: est.TotalHourlyInterest.String() + " " + quote, Short: true},
		{Title: "Interest To Month End", Value: est.TotalInterestToMonthEnd.String() + " " + quote, Short: true},
		{Title: "Projected Monthly Interest", Value: est.TotalMonthlyInterest.String() + " " + quote, Short: true},
	}

	for _, e := range est.Assets {
		fields = append(fields, slack.AttachmentField{
			Title: e.Asset,
			Value: fmt.Sprintf("Debt: %s Hourly Rate: %s Monthly Interest: %s %s",
				e.Debt.String(), e.HourlyRate.FormatPercentage(5), e.MonthlyInterest.String(), quote),
		})
	}

	return slack.Attachment{
		Color:  "danger",
		Title:  "Interest Budget Alert",
		Text:   a.Message,
		Fields: fields,
	}
}

func (e *InterestEstimation) String() string {
	s := fmt.Sprintf("debt value: %s %s, hourly interest: %s, interest to month end (%.1f hours): %s, projected monthly interest: %s",
		e.TotalDebtValue.String(), e.QuoteCurrency,
		e.TotalHourlyInterest.String(),
		e.HoursToMonthEnd.Float64(),
		e.TotalInterestToMonthEnd.String(),
		e.TotalMonthlyInterest.String(),
	)

	if len(e.MissingRates) > 0 {
		s += fmt.Sprintf(", missing rates: %v", e.MissingRates)
	}

	if len(e.MissingPrices) > 0 {
		s += fmt.Sprintf(", missing prices: %v", e.MissingPrices)
	}

	return s
}
