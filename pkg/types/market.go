package types

import (
	"encoding/json"
	"fmt"
	"github.com/leekchan/accounting"
	"math"
	"strconv"
	"time"
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
	Symbol      string
	LocalSymbol string // LocalSymbol is used for exchange's API

	PricePrecision  int
	VolumePrecision int
	QuoteCurrency   string
	BaseCurrency    string

	// The MIN_NOTIONAL filter defines the minimum notional value allowed for an order on a symbol.
	// An order's notional value is the price * quantity
	MinNotional float64
	MinAmount   float64

	// The LOT_SIZE filter defines the quantity
	MinQuantity float64
	MaxQuantity float64
	StepSize    float64

	MinPrice float64
	MaxPrice float64
	TickSize float64
}

func (m Market) BaseCurrencyFormatter() *accounting.Accounting {
	a := accounting.DefaultAccounting(m.BaseCurrency, m.VolumePrecision)
	a.Format = "%v %s"
	return a
}

func (m Market) QuoteCurrencyFormatter() *accounting.Accounting {
	a := accounting.DefaultAccounting(m.QuoteCurrency, m.PricePrecision)
	a.Format = "%v %s"
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
