package types

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types/asset"
	currency2 "github.com/c9s/bbgo/pkg/types/currency"
)

type PriceMap map[string]fixedpoint.Value

type Balance struct {
	Currency  string           `json:"currency"`
	Available fixedpoint.Value `json:"available"`
	Locked    fixedpoint.Value `json:"locked,omitempty"`

	// margin related fields
	Borrowed fixedpoint.Value `json:"borrowed,omitempty"`
	Interest fixedpoint.Value `json:"interest,omitempty"`

	// credit related fields
	// long available in base currency amount for credit account
	LongAvailableCredit fixedpoint.Value `json:"longAvailableCredit,omitempty"`
	// short available in base currency amount for credit account
	ShortAvailableCredit fixedpoint.Value `json:"shortAvailableCredit,omitempty"`

	// NetAsset = (Available + Locked) - Borrowed - Interest
	NetAsset fixedpoint.Value `json:"net,omitempty"`

	MaxWithdrawAmount fixedpoint.Value `json:"maxWithdrawAmount,omitempty"`
}

func NewBalance(currency string, aval fixedpoint.Value) Balance {
	b := NewZeroBalance(currency)
	b.Available = aval
	return b
}

func NewZeroBalance(currency string) Balance {
	return Balance{
		Currency:             currency,
		Available:            fixedpoint.Zero,
		Locked:               fixedpoint.Zero,
		Borrowed:             fixedpoint.Zero,
		Interest:             fixedpoint.Zero,
		LongAvailableCredit:  fixedpoint.Zero,
		ShortAvailableCredit: fixedpoint.Zero,
		NetAsset:             fixedpoint.Zero,
		MaxWithdrawAmount:    fixedpoint.Zero,
	}
}

func (b Balance) Add(b2 Balance) Balance {
	var newB = b
	newB.Available = b.Available.Add(b2.Available)
	newB.Locked = b.Locked.Add(b2.Locked)
	newB.Borrowed = b.Borrowed.Add(b2.Borrowed)
	newB.Interest = b.Interest.Add(b2.Interest)

	if !b.NetAsset.IsZero() && !b2.NetAsset.IsZero() {
		newB.NetAsset = b.NetAsset.Add(b2.NetAsset)
	} else {
		// do not use this field, reset it when any of the balance is zero
		newB.NetAsset = fixedpoint.Zero
	}

	return newB
}

func (b Balance) Total() fixedpoint.Value {
	return b.Available.Add(b.Locked)
}

// Net returns the net asset value (total - debt)
func (b Balance) Net() fixedpoint.Value {
	if !b.NetAsset.IsZero() {
		return b.NetAsset
	}

	return b.Total().Sub(b.Debt())
}

func (b Balance) Debt() fixedpoint.Value {
	return b.Borrowed.Add(b.Interest)
}

func (b Balance) ValueString() (o string) {
	o = b.Net().String()

	if b.Locked.Sign() > 0 {
		o += fmt.Sprintf(" (locked %v)", b.Locked)
	}

	if b.Borrowed.Sign() > 0 {
		o += fmt.Sprintf(" (borrowed: %v)", b.Borrowed)
	}

	return o
}

func (b Balance) String() (o string) {
	o = fmt.Sprintf("%s: %s", b.Currency, b.Net().String())

	if b.Locked.Sign() > 0 {
		o += fmt.Sprintf(" (locked %f)", b.Locked.Float64())
	}

	if b.Borrowed.Sign() > 0 {
		o += fmt.Sprintf(" (borrowed: %f)", b.Borrowed.Float64())
	}

	if b.Interest.Sign() > 0 {
		o += fmt.Sprintf(" (interest: %f)", b.Interest.Float64())
	}

	return o
}

type BalanceSnapshot struct {
	Balances BalanceMap `json:"balances"`
	Session  string     `json:"session"`
	Time     time.Time  `json:"time"`
}

func (m BalanceSnapshot) CsvHeader() []string {
	return []string{"time", "session", "currency", "available", "locked", "borrowed"}
}

func (m BalanceSnapshot) CsvRecords() [][]string {
	var records [][]string

	for cur, b := range m.Balances {
		records = append(records, []string{
			strconv.FormatInt(m.Time.Unix(), 10),
			m.Session,
			cur,
			b.Available.String(),
			b.Locked.String(),
			b.Borrowed.String(),
		})
	}

	return records
}

type BalanceMap map[string]Balance

func (m BalanceMap) NotZero() BalanceMap {
	bm := make(BalanceMap)
	for c, b := range m {
		if b.Total().IsZero() && b.Debt().IsZero() && b.Net().IsZero() {
			continue
		}

		bm[c] = b
	}
	return bm
}

func (m BalanceMap) Debts() BalanceMap {
	bm := make(BalanceMap)
	for c, b := range m {
		if b.Borrowed.Sign() > 0 || b.Interest.Sign() > 0 {
			bm[c] = b
		}
	}
	return bm
}

func (m BalanceMap) Currencies() (currencies []string) {
	for _, b := range m {
		currencies = append(currencies, b.Currency)
	}
	return currencies
}

func (m BalanceMap) Add(bm BalanceMap) BalanceMap {
	var total = m.Copy()
	for _, b := range bm {
		tb, ok := total[b.Currency]
		if ok {
			tb = tb.Add(b)
		} else {
			tb = b
		}
		total[b.Currency] = tb
	}
	return total
}

func (m BalanceMap) String() string {
	var ss []string
	for _, b := range m {
		ss = append(ss, b.String())
	}

	return "BalanceMap[" + strings.Join(ss, ", ") + "]"
}

func (m BalanceMap) Copy() (d BalanceMap) {
	d = make(BalanceMap)
	for c, b := range m {
		d[c] = b
	}
	return d
}

// Assets converts balances into assets with the given prices
func (m BalanceMap) Assets(prices PriceMap, priceTime time.Time) asset.Map {
	assets := make(asset.Map)

	_, btcInUSD, hasBtcPrice := findUSDMarketPrice("BTC", prices)

	for cu, b := range m {
		total := b.Total()
		netAsset := b.Net()
		debt := b.Debt()

		if total.IsZero() && netAsset.IsZero() && debt.IsZero() {
			continue
		}

		as := asset.Asset{
			Currency:  cu,
			Total:     total,
			Time:      priceTime,
			Locked:    b.Locked,
			Available: b.Available,
			Borrowed:  b.Borrowed,
			Interest:  b.Interest,
			NetAsset:  netAsset,
		}

		if currency2.IsUSDFiatCurrency(cu) { // for usd
			as.NetAssetInUSD = netAsset
			as.PriceInUSD = fixedpoint.One
			if hasBtcPrice && !as.NetAssetInUSD.IsZero() {
				as.NetAssetInBTC = as.NetAssetInUSD.Div(btcInUSD)
			}
		} else { // for crypto
			if market, usdPrice, ok := findUSDMarketPrice(cu, prices); ok {
				// this includes USDT, USD, USDC and so on
				if strings.HasPrefix(market, "USD") || strings.HasPrefix(market, "BUSD") { // for prices like USDT/TWD, BUSD/USDT
					if !as.NetAsset.IsZero() {
						as.NetAssetInUSD = as.NetAsset.Div(usdPrice)
					}
					as.PriceInUSD = fixedpoint.One.Div(usdPrice)
				} else { // for prices like BTC/USDT
					if !as.NetAsset.IsZero() {
						as.NetAssetInUSD = as.NetAsset.Mul(usdPrice)
					}
					as.PriceInUSD = usdPrice
				}

				if hasBtcPrice && !as.NetAssetInUSD.IsZero() {
					as.NetAssetInBTC = as.NetAssetInUSD.Div(btcInUSD)
				}
			}
		}

		assets[cu] = as
	}

	return assets
}

func (m BalanceMap) Print(log LogFunc) {
	for _, balance := range m {
		if balance.Net().IsZero() {
			continue
		}

		o := fmt.Sprintf(" %s: %v", balance.Currency, balance.Available)
		if balance.Locked.Sign() > 0 {
			o += fmt.Sprintf(" (locked %v)", balance.Locked)
		}

		if balance.Borrowed.Sign() > 0 {
			o += fmt.Sprintf(" (borrowed %v)", balance.Borrowed)
		}

		log(o)
	}
}

func (m BalanceMap) SlackAttachment() slack.Attachment {
	var fields []slack.AttachmentField

	for _, b := range m {
		fields = append(fields, slack.AttachmentField{
			Title: b.Currency,
			Value: b.ValueString(),
			Short: true,
		})
	}

	return slack.Attachment{
		Color:  "#CCA33F",
		Fields: fields,
	}
}

func findUSDMarketPrice(currency string, prices map[string]fixedpoint.Value) (string, fixedpoint.Value, bool) {
	usdMarkets := []string{currency + "USDT", currency + "USDC", currency + "USD", "USDT" + currency}
	for _, market := range usdMarkets {
		if usdPrice, ok := prices[market]; ok {
			return market, usdPrice, ok
		}
	}
	return "", fixedpoint.Zero, false
}
