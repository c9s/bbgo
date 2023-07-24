package types

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type PriceMap map[string]fixedpoint.Value

type Balance struct {
	Currency  string           `json:"currency"`
	Available fixedpoint.Value `json:"available"`
	Locked    fixedpoint.Value `json:"locked,omitempty"`

	// margin related fields
	Borrowed fixedpoint.Value `json:"borrowed,omitempty"`
	Interest fixedpoint.Value `json:"interest,omitempty"`

	// NetAsset = (Available + Locked) - Borrowed - Interest
	NetAsset fixedpoint.Value `json:"net,omitempty"`

	MaxWithdrawAmount fixedpoint.Value `json:"maxWithdrawAmount,omitempty"`
}

func (b Balance) Add(b2 Balance) Balance {
	var newB = b
	newB.Available = b.Available.Add(b2.Available)
	newB.Locked = b.Locked.Add(b2.Locked)
	newB.Borrowed = b.Borrowed.Add(b2.Borrowed)
	newB.NetAsset = b.NetAsset.Add(b2.NetAsset)
	newB.Interest = b.Interest.Add(b2.Interest)
	return newB
}

func (b Balance) Total() fixedpoint.Value {
	return b.Available.Add(b.Locked)
}

// Net returns the net asset value (total - debt)
func (b Balance) Net() fixedpoint.Value {
	total := b.Total()
	return total.Sub(b.Debt())
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
func (m BalanceMap) Assets(prices PriceMap, priceTime time.Time) AssetMap {
	assets := make(AssetMap)

	_, btcInUSD, hasBtcPrice := findUSDMarketPrice("BTC", prices)

	for currency, b := range m {
		total := b.Total()
		netAsset := b.Net()

		if total.IsZero() && netAsset.IsZero() {
			continue
		}

		asset := Asset{
			Currency:  currency,
			Total:     total,
			Time:      priceTime,
			Locked:    b.Locked,
			Available: b.Available,
			Borrowed:  b.Borrowed,
			Interest:  b.Interest,
			NetAsset:  netAsset,
		}

		if IsUSDFiatCurrency(currency) { // for usd
			asset.InUSD = netAsset
			asset.PriceInUSD = fixedpoint.One
			if hasBtcPrice && !asset.InUSD.IsZero() {
				asset.InBTC = asset.InUSD.Div(btcInUSD)
			}
		} else { // for crypto
			if market, usdPrice, ok := findUSDMarketPrice(currency, prices); ok {
				// this includes USDT, USD, USDC and so on
				if strings.HasPrefix(market, "USD") || strings.HasPrefix(market, "BUSD") { // for prices like USDT/TWD, BUSD/USDT
					if !asset.NetAsset.IsZero() {
						asset.InUSD = asset.NetAsset.Div(usdPrice)
					}
					asset.PriceInUSD = fixedpoint.One.Div(usdPrice)
				} else { // for prices like BTC/USDT
					if !asset.NetAsset.IsZero() {
						asset.InUSD = asset.NetAsset.Mul(usdPrice)
					}
					asset.PriceInUSD = usdPrice
				}

				if hasBtcPrice && !asset.InUSD.IsZero() {
					asset.InBTC = asset.InUSD.Div(btcInUSD)
				}
			}
		}

		assets[currency] = asset
	}

	return assets
}

func (m BalanceMap) Print() {
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

		log.Infoln(o)
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
