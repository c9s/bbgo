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
	MinNotional float64 `json:"minNotional,omitempty"`
	MinAmount   float64 `json:"minAmount,omitempty"`

	// The LOT_SIZE filter defines the quantity
	MinQuantity float64 `json:"minQuantity,omitempty"`

	// MaxQuantity is currently not used in the code
	MaxQuantity float64 `json:"maxQuantity,omitempty"`

	// StepSize is the step size of quantity
	// can be converted from precision, e.g.
	//    1.0 / math.Pow10(m.BaseUnitPrecision)
	StepSize float64 `json:"stepSize,omitempty"`

	MinPrice float64 `json:"minPrice,omitempty"`
	MaxPrice float64 `json:"maxPrice,omitempty"`

	// TickSize is the step size of price
	TickSize float64 `json:"tickSize,omitempty"`
}

// TruncateQuantity uses the step size to truncate floating number, in order to avoid the rounding issue
func (m Market) TruncateQuantity(quantity fixedpoint.Value) fixedpoint.Value {
	stepRound := math.Pow10(-int(math.Log10(m.StepSize)))
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

func (m Market) FormatPriceCurrency(val float64) string {
	switch m.QuoteCurrency {

	case "USD", "USDT":
		return USD.FormatMoneyFloat64(val)

	case "BTC":
		return BTC.FormatMoneyFloat64(val)

	case "BNB":
		return BNB.FormatMoneyFloat64(val)

	}

	return m.FormatPrice(val)
}

func (m Market) FormatPrice(val float64) string {
	// p := math.Pow10(m.PricePrecision)
	return formatPrice(val, m.TickSize)
}

func formatPrice(price float64, tickSize float64) string {
	prec := int(math.Round(math.Abs(math.Log10(tickSize))))
	p := math.Pow10(prec)
	price = math.Trunc(price*p) / p
	return strconv.FormatFloat(price, 'f', prec, 64)
}

func (m Market) FormatQuantity(val float64) string {
	return formatQuantity(val, m.StepSize)
}

func formatQuantity(quantity float64, lot float64) string {
	prec := int(math.Round(math.Abs(math.Log10(lot))))
	p := math.Pow10(prec)
	quantity = math.Trunc(quantity*p) / p
	return strconv.FormatFloat(quantity, 'f', prec, 64)
}

func (m Market) FormatVolume(val float64) string {
	p := math.Pow10(m.VolumePrecision)
	val = math.Trunc(val*p) / p
	return strconv.FormatFloat(val, 'f', m.VolumePrecision, 64)
}

func (m Market) CanonicalizeVolume(val float64) float64 {
	p := math.Pow10(m.VolumePrecision)
	return math.Trunc(p*val) / p
}

type MarketMap map[string]Market

func (m MarketMap) Add(market Market) {
	m[market.Symbol] = market
}
