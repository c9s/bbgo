package asset

import (
	"fmt"
	"sort"
	"time"

	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types/currency"
)

type Asset struct {
	Currency string `json:"currency" db:"currency"`

	Total     fixedpoint.Value `json:"total" db:"total"`
	Locked    fixedpoint.Value `json:"lock" db:"lock" `
	Available fixedpoint.Value `json:"available"  db:"available"`
	Borrowed  fixedpoint.Value `json:"borrowed" db:"borrowed"`
	Interest  fixedpoint.Value `json:"interest" db:"interest"`

	NetAsset fixedpoint.Value `json:"netAsset" db:"net_asset"`

	// NetAssetInUSD is net asset in USD
	NetAssetInUSD fixedpoint.Value `json:"netAssetInUSD" db:"net_asset_in_usd"`

	// NetAssetInBTC is net asset in BTC
	NetAssetInBTC fixedpoint.Value `json:"netAssetInBTC" db:"net_asset_in_btc"`

	Debt          fixedpoint.Value `json:"debt" db:"debt"`
	DebtInUSD     fixedpoint.Value `json:"debtInUSD" db:"debt_in_usd"`
	InterestInUSD fixedpoint.Value `json:"interestInUSD" db:"interest_in_usd"`

	Time       time.Time        `json:"time" db:"time"`
	PriceInUSD fixedpoint.Value `json:"priceInUSD" db:"price_in_usd"`
}

type Map map[string]Asset

func (m Map) Merge(other Map) Map {
	newMap := make(Map)
	for cu, asset := range other {
		if existing, ok := m[cu]; ok {
			asset.Total = asset.Total.Add(existing.Total)
			asset.Locked = asset.Locked.Add(existing.Locked)
			asset.Available = asset.Available.Add(existing.Available)
			asset.Borrowed = asset.Borrowed.Add(existing.Borrowed)

			asset.NetAsset = asset.NetAsset.Add(existing.NetAsset)
			asset.NetAssetInUSD = asset.NetAssetInUSD.Add(existing.NetAssetInUSD)
			asset.NetAssetInBTC = asset.NetAssetInBTC.Add(existing.NetAssetInBTC)

			asset.Debt = asset.Debt.Add(existing.Debt)
			asset.DebtInUSD = asset.DebtInUSD.Add(existing.DebtInUSD)
			asset.Interest = asset.Interest.Add(existing.Interest)
			asset.InterestInUSD = asset.InterestInUSD.Add(existing.InterestInUSD)

			if asset.PriceInUSD.IsZero() && existing.PriceInUSD.Sign() > 0 {
				asset.PriceInUSD = existing.PriceInUSD
			}
		}

		newMap[cu] = asset
	}

	return newMap
}

func (m Map) Filter(f func(asset *Asset) bool) Map {
	newMap := make(Map)

	for cu, asset := range m {
		if f(&asset) {
			newMap[cu] = asset
		}
	}

	return newMap
}

func (m Map) InUSD() (total fixedpoint.Value) {
	for _, a := range m {
		if a.NetAssetInUSD.IsZero() {
			continue
		}

		total = total.Add(a.NetAssetInUSD)
	}

	return total
}

func (m Map) PlainText() (o string) {
	var assets = m.Slice()

	// sort assets
	sort.Slice(assets, func(i, j int) bool {
		return assets[i].NetAssetInUSD.Compare(assets[j].NetAssetInUSD) > 0
	})

	sumUsd := fixedpoint.Zero
	sumBTC := fixedpoint.Zero
	for _, a := range assets {
		usd := a.NetAssetInUSD
		btc := a.NetAssetInBTC
		if !a.NetAssetInUSD.IsZero() {
			o += fmt.Sprintf(" %s: %s (≈ %s) (≈ %s)",
				a.Currency,
				a.NetAsset.String(),
				currency.USD.FormatMoney(usd),
				currency.BTC.FormatMoney(btc),
			) + "\n"
			sumUsd = sumUsd.Add(usd)
			sumBTC = sumBTC.Add(btc)
		} else {
			o += fmt.Sprintf(" %s: %s",
				a.Currency,
				a.NetAsset.String(),
			) + "\n"
		}
	}

	o += fmt.Sprintf("Net Asset Value: (≈ %s) (≈ %s)",
		currency.USD.FormatMoney(sumUsd),
		currency.BTC.FormatMoney(sumBTC),
	)
	return o
}

func (m Map) Slice() (assets []Asset) {
	for _, a := range m {
		assets = append(assets, a)
	}
	return assets
}

func (m Map) SlackAttachment() slack.Attachment {
	var fields []slack.AttachmentField
	var netAssetInBTC, netAssetInUSD fixedpoint.Value

	var assets = m.Slice()

	// sort assets
	sort.Slice(assets, func(i, j int) bool {
		return assets[i].NetAssetInUSD.Compare(assets[j].NetAssetInUSD) > 0
	})

	for _, a := range assets {
		netAssetInUSD = netAssetInUSD.Add(a.NetAssetInUSD)
		netAssetInBTC = netAssetInBTC.Add(a.NetAssetInBTC)
	}

	for _, a := range assets {
		if !a.NetAssetInUSD.IsZero() {
			text := fmt.Sprintf("%s (≈ %s) (≈ %s) (%s)",
				a.NetAsset.String(),
				currency.USD.FormatMoney(a.NetAssetInUSD),
				currency.BTC.FormatMoney(a.NetAssetInBTC),
				a.NetAssetInUSD.Div(netAssetInUSD).FormatPercentage(2),
			)

			if !a.Borrowed.IsZero() {
				text += fmt.Sprintf(" Borrowed: %s", a.Borrowed.String())
			}

			if !a.Interest.IsZero() {
				text += fmt.Sprintf(" Interest: %s", a.Interest.String())
			}

			fields = append(fields, slack.AttachmentField{
				Title: a.Currency,
				Value: text,
				Short: false,
			})
		} else {
			text := a.NetAsset.String()

			if !a.Borrowed.IsZero() {
				text += fmt.Sprintf(" Borrowed: %s", a.Borrowed.String())
			}

			fields = append(fields, slack.AttachmentField{
				Title: a.Currency,
				Value: text,
				Short: false,
			})
		}
	}

	return slack.Attachment{
		Title: fmt.Sprintf("Net Asset Value %s (≈ %s)",
			currency.USD.FormatMoney(netAssetInUSD),
			currency.BTC.FormatMoney(netAssetInBTC),
		),
		Fields: fields,
	}
}
