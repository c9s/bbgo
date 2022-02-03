package types

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/leekchan/accounting"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type Duration time.Duration

func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

func (d *Duration) UnmarshalJSON(data []byte) error {
	var o interface{}

	if err := json.Unmarshal(data, &o); err != nil {
		return err
	}

	switch t := o.(type) {
	case string:
		dd, err := time.ParseDuration(t)
		if err != nil {
			return err
		}

		*d = Duration(dd)

	case float64:
		*d = Duration(int64(t * float64(time.Second)))

	case int64:
		*d = Duration(t * int64(time.Second))
	case int:
		*d = Duration(t * int(time.Second))

	default:
		return fmt.Errorf("unsupported type %T value: %v", t, t)

	}

	return nil
}

type Market struct {
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

	MinPrice fixedpoint.Value `json:"minPrice,omitempty"`
	MaxPrice fixedpoint.Value `json:"maxPrice,omitempty"`

	// TickSize is the step size of price
	TickSize fixedpoint.Value `json:"tickSize,omitempty"`
}

// TruncateQuantity uses the step size to truncate floating number, in order to avoid the rounding issue
func (m Market) TruncateQuantity(quantity fixedpoint.Value) fixedpoint.Value {
	stepRound := math.Pow10(-int(math.Log10(m.StepSize.Float64())))
	return fixedpoint.NewFromFloat(math.Trunc(quantity.Float64()*stepRound) / stepRound)
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
	return formatPrice(val, m.TickSize)
}

func formatPrice(price fixedpoint.Value, tickSize fixedpoint.Value) string {
	// TODO Round
	prec := int(math.Round(math.Abs(math.Log10(tickSize.Float64()))))
	p := math.Pow10(prec)
	pp := math.Trunc(price.Float64()*p) / p
	return strconv.FormatFloat(pp, 'f', prec, 64)
}

func (m Market) FormatQuantity(val fixedpoint.Value) string {
	return formatQuantity(val, m.StepSize)
}

func formatQuantity(quantity fixedpoint.Value, lot fixedpoint.Value) string {
	// TODO Round
	prec := int(math.Round(math.Abs(math.Log10(lot.Float64()))))
	p := math.Pow10(prec)
	q := math.Trunc(quantity.Float64() * p) / p
	return strconv.FormatFloat(q, 'f', prec, 64)
}

func (m Market) FormatVolume(val fixedpoint.Value) string {
	// TODO Round
	p := math.Pow10(m.VolumePrecision)
	v := math.Trunc(val.Float64()*p) / p
	return strconv.FormatFloat(v, 'f', m.VolumePrecision, 64)
}

func (m Market) CanonicalizeVolume(val fixedpoint.Value) float64 {
	// TODO Round
	p := math.Pow10(m.VolumePrecision)
	return math.Trunc(p*val.Float64()) / p
}

type MarketMap map[string]Market

func (m MarketMap) Add(market Market) {
	m[market.Symbol] = market
}
