package types

import (
	"math"
	"strconv"

	"github.com/leekchan/accounting"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type Market struct {
	Exchange ExchangeName `json:"exchange,omitempty"`

	Symbol string `json:"symbol"`

	// LocalSymbol is used for exchange's API (exchange package internal)
	LocalSymbol string `json:"localSymbol,omitempty"`

	// PricePrecision is the precision used for formatting price, 8 = 8 decimals
	// can be converted from price tick step size, e.g.
	//    int(math.Log10(price step size))
	PricePrecision int `json:"pricePrecision"`

	// VolumePrecision is the precision used for formatting quantity and volume, 8 = 8 decimals
	// can be converted from step size, e.g.
	//    int(math.Log10(quantity step size))
	VolumePrecision int `json:"volumePrecision"`

	// QuoteCurrency is the currency name for quote, e.g. USDT in BTC/USDT, USDC in BTC/USDC
	QuoteCurrency string `json:"quoteCurrency"`

	// BaseCurrency is the current name for base, e.g. BTC in BTC/USDT, ETH in ETH/USDC
	BaseCurrency string `json:"baseCurrency"`

	// The MIN_NOTIONAL filter defines the minimum notional value allowed for an order on a symbol.
	// An order's notional value is the price * quantity
	MinNotional fixedpoint.Value `json:"minNotional,omitempty"`
	MinAmount   fixedpoint.Value `json:"minAmount,omitempty"`

	// The LOT_SIZE filter defines the quantity
	MinQuantity fixedpoint.Value `json:"minQuantity,omitempty"`

	// MaxQuantity is currently not used in the code
	MaxQuantity fixedpoint.Value `json:"maxQuantity,omitempty"`

	// StepSize is the step size of quantity
	// can be converted from precision, e.g.
	//    1.0 / math.Pow10(m.BaseUnitPrecision)
	StepSize fixedpoint.Value `json:"stepSize,omitempty"`

	// TickSize is the step size of price
	TickSize fixedpoint.Value `json:"tickSize,omitempty"`

	MinPrice fixedpoint.Value `json:"minPrice,omitempty"`
	MaxPrice fixedpoint.Value `json:"maxPrice,omitempty"`
}

func (m Market) IsDustQuantity(quantity, price fixedpoint.Value) bool {
	return quantity.Compare(m.MinQuantity) <= 0 || quantity.Mul(price).Compare(m.MinNotional) <= 0
}

// TruncateQuantity uses the step size to truncate floating number, in order to avoid the rounding issue
func (m Market) TruncateQuantity(quantity fixedpoint.Value) fixedpoint.Value {
	var ts = m.StepSize.Float64()
	var prec = int(math.Round(math.Log10(ts) * -1.0))
	var pow10 = math.Pow10(prec)

	qf := math.Trunc(quantity.Float64() * pow10)
	qf = qf / pow10

	qs := strconv.FormatFloat(qf, 'f', prec, 64)
	return fixedpoint.MustNewFromString(qs)
}

// TruncateQuoteQuantity uses the tick size to truncate floating number, in order to avoid the rounding issue
// this is usually used for calculating the order size from the quote quantity.
func (m Market) TruncateQuoteQuantity(quantity fixedpoint.Value) fixedpoint.Value {
	var ts = m.TickSize.Float64()
	var prec = int(math.Round(math.Log10(ts) * -1.0))
	var pow10 = math.Pow10(prec)

	qf := math.Trunc(quantity.Float64() * pow10)
	qf = qf / pow10

	qs := strconv.FormatFloat(qf, 'f', prec, 64)
	return fixedpoint.MustNewFromString(qs)
}

// GreaterThanMinimalOrderQuantity ensures that your given balance could fit the minimal order quantity
// when side = sell, then available = base balance
// when side = buy, then available = quote balance
// The balance will be truncated first in order to calculate the minimal notional and minimal quantity
// The adjusted (truncated) order quantity will be returned
func (m Market) GreaterThanMinimalOrderQuantity(
	side SideType, price, available fixedpoint.Value,
) (fixedpoint.Value, bool) {
	switch side {
	case SideTypeSell:
		available = m.TruncateQuantity(available)

		if available.Compare(m.MinQuantity) < 0 {
			return fixedpoint.Zero, false
		}

		quoteAmount := price.Mul(available)
		if quoteAmount.Compare(m.MinNotional) < 0 {
			return fixedpoint.Zero, false
		}

		return available, true

	case SideTypeBuy:
		available = m.TruncateQuoteQuantity(available)

		if available.Compare(m.MinNotional) < 0 {
			return fixedpoint.Zero, false
		}

		quantity := available.Div(price)
		quantity = m.TruncateQuantity(quantity)
		if quantity.Compare(m.MinQuantity) < 0 {
			return fixedpoint.Zero, false
		}

		notional := quantity.Mul(price)
		if notional.Compare(m.MinNotional) < 0 {
			return fixedpoint.Zero, false
		}

		return quantity, true
	}

	return available, true
}

// RoundDownQuantityByPrecision uses the volume precision to round down the quantity
// This is different from the TruncateQuantity, which uses StepSize (it uses fewer fractions to truncate)
func (m Market) RoundDownQuantityByPrecision(quantity fixedpoint.Value) fixedpoint.Value {
	return quantity.Round(m.VolumePrecision, fixedpoint.Down)
}

// RoundUpQuantityByPrecision uses the volume precision to round up the quantity
func (m Market) RoundUpQuantityByPrecision(quantity fixedpoint.Value) fixedpoint.Value {
	return quantity.Round(m.VolumePrecision, fixedpoint.Up)
}

func (m Market) TruncatePrice(price fixedpoint.Value) fixedpoint.Value {
	return fixedpoint.MustNewFromString(m.FormatPrice(price))
}

func (m Market) BaseCurrencyFormatter() *accounting.Accounting {
	a := accounting.DefaultAccounting(m.BaseCurrency, m.VolumePrecision)
	a.Format = "%v %s"
	return a
}

func (m Market) QuoteCurrencyFormatter() *accounting.Accounting {
	var format, symbol string

	switch m.QuoteCurrency {
	case "USDT", "USDC", "USD":
		symbol = "$"
		format = "%s %v"

	default:
		symbol = m.QuoteCurrency
		format = "%v %s"
	}

	a := accounting.DefaultAccounting(symbol, m.PricePrecision)
	a.Format = format
	return a
}

func (m Market) FormatPriceCurrency(val fixedpoint.Value) string {
	switch m.QuoteCurrency {

	case "USD", "USDT":
		return USD.FormatMoney(val)

	case "BTC":
		return BTC.FormatMoney(val)

	case "BNB":
		return BNB.FormatMoney(val)

	}

	return m.FormatPrice(val)
}

func (m Market) FormatPrice(val fixedpoint.Value) string {
	// p := math.Pow10(m.PricePrecision)
	return FormatPrice(val, m.TickSize)
}

func FormatPrice(price fixedpoint.Value, tickSize fixedpoint.Value) string {
	prec := int(math.Round(math.Log10(tickSize.Float64()) * -1.0))
	return price.FormatString(prec)
}

func (m Market) FormatQuantity(val fixedpoint.Value) string {
	return formatQuantity(val, m.StepSize)
}

func formatQuantity(quantity fixedpoint.Value, lot fixedpoint.Value) string {
	prec := int(math.Round(math.Abs(math.Log10(lot.Float64()))))
	return quantity.FormatString(prec)
}

func (m Market) FormatVolume(val fixedpoint.Value) string {
	return val.FormatString(m.VolumePrecision)
}

func (m Market) CanonicalizeVolume(val fixedpoint.Value) float64 {
	// TODO Round
	p := math.Pow10(m.VolumePrecision)
	return math.Trunc(p*val.Float64()) / p
}

func (m Market) AdjustQuantityByMinQuantity(quantity fixedpoint.Value)  fixedpoint.Value {
	return fixedpoint.Max(quantity, m.MinQuantity)
}

// AdjustQuantityByMinNotional adjusts the quantity to make the amount greater than the given minAmount
func (m Market) AdjustQuantityByMinNotional(quantity, currentPrice fixedpoint.Value) fixedpoint.Value {
	// modify quantity for the min amount
	amount := currentPrice.Mul(quantity)
	if amount.Compare(m.MinNotional) < 0 {
		ratio := m.MinNotional.Div(amount)
		quantity = quantity.Mul(ratio)

		ts := m.StepSize.Float64()
		prec := int(math.Round(math.Log10(ts) * -1.0))
		return quantity.Round(prec, fixedpoint.Up)
	}

	return quantity
}

type MarketMap map[string]Market

func (m MarketMap) Add(market Market) {
	m[market.Symbol] = market
}

func (m MarketMap) Has(symbol string) bool {
	_, ok := m[symbol]
	return ok
}
