package types

import (
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type Balance struct {
	Currency  string           `json:"currency"`
	Available fixedpoint.Value `json:"available"`
	Locked    fixedpoint.Value `json:"locked,omitempty"`

	// margin related fields
	Borrowed fixedpoint.Value `json:"borrowed,omitempty"`
	Interest fixedpoint.Value `json:"interest,omitempty"`

	// NetAsset = (Available + Locked) - Borrowed - Interest
	NetAsset fixedpoint.Value `json:"net,omitempty"`
}

func (b Balance) Total() fixedpoint.Value {
	return b.Available.Add(b.Locked)
}

func (b Balance) String() (o string) {
	o = fmt.Sprintf("%s: %s", b.Currency, b.Available.String())

	if b.Locked.Sign() > 0 {
		o += fmt.Sprintf(" (locked %v)", b.Locked)
	}

	if b.Borrowed.Sign() > 0 {
		o += fmt.Sprintf(" (borrowed: %v)", b.Borrowed)
	}

	return o
}

type BalanceMap map[string]Balance

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
			tb.Available = tb.Available.Add(b.Available)
			tb.Locked = tb.Locked.Add(b.Locked)
			tb.Borrowed = tb.Borrowed.Add(b.Borrowed)
			tb.NetAsset = tb.NetAsset.Add(b.NetAsset)
			tb.Interest = tb.Interest.Add(b.Interest)
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
func (m BalanceMap) Assets(prices map[string]fixedpoint.Value, priceTime time.Time) AssetMap {
	assets := make(AssetMap)
	btcusdt, hasBtcPrice := prices["BTCUSDT"]
	for currency, b := range m {
		if b.Locked.IsZero() && b.Available.IsZero() && b.Borrowed.IsZero() {
			continue
		}

		total := b.Available.Add(b.Locked)
		netAsset := b.NetAsset
		if netAsset.IsZero() {
			netAsset = total.Sub(b.Borrowed)
		}

		asset := Asset{
			Currency:  currency,
			Total:     total,
			Time:      priceTime,
			Locked:    b.Locked,
			Available: b.Available,
			Borrowed:  b.Borrowed,
			NetAsset:  netAsset,
		}

		if strings.HasPrefix(currency, "USD") { // for usd
			asset.InUSD = netAsset
			asset.PriceInUSD = fixedpoint.One
			if hasBtcPrice && !asset.InUSD.IsZero() {
				asset.InBTC = asset.InUSD.Div(btcusdt)
			}
		} else { // for crypto
			if market, usdPrice, ok := findUSDMarketPrice(currency, prices); ok {
				// this includes USDT, USD, USDC and so on
				if strings.HasPrefix(market, "USD") { // for prices like USDT/TWD
					if !asset.Total.IsZero() {
						asset.InUSD = asset.Total.Div(usdPrice)
					}
					asset.PriceInUSD = fixedpoint.One.Div(usdPrice)
				} else { // for prices like BTC/USDT
					if !asset.Total.IsZero() {
						asset.InUSD = asset.Total.Mul(usdPrice)
					}
					asset.PriceInUSD = usdPrice
				}

				if hasBtcPrice && !asset.InUSD.IsZero() {
					asset.InBTC = asset.InUSD.Div(btcusdt)
				}
			}
		}

		assets[currency] = asset
	}

	return assets
}

func (m BalanceMap) Print() {
	for _, balance := range m {
		if balance.Available.IsZero() && balance.Locked.IsZero() {
			continue
		}

		if balance.Locked.Sign() > 0 {
			logrus.Infof(" %s: %v (locked %v)", balance.Currency, balance.Available, balance.Locked)
		} else {
			logrus.Infof(" %s: %v", balance.Currency, balance.Available)
		}
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
