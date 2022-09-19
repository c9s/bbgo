package types

import (
	"fmt"
	"sort"
	"time"

	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type Asset struct {
	Currency string `json:"currency" db:"currency"`

	Total fixedpoint.Value `json:"total" db:"total"`

	NetAsset fixedpoint.Value `json:"netAsset" db:"net_asset"`

	Interest fixedpoint.Value `json:"interest" db:"interest"`

	// InUSD is net asset in USD
	InUSD fixedpoint.Value `json:"inUSD" db:"net_asset_in_usd"`

	// InBTC is net asset in BTC
	InBTC fixedpoint.Value `json:"inBTC" db:"net_asset_in_btc"`

	Time       time.Time        `json:"time" db:"time"`
	Locked     fixedpoint.Value `json:"lock" db:"lock" `
	Available  fixedpoint.Value `json:"available"  db:"available"`
	Borrowed   fixedpoint.Value `json:"borrowed" db:"borrowed"`
	PriceInUSD fixedpoint.Value `json:"priceInUSD" db:"price_in_usd"`
}

type AssetMap map[string]Asset

func (m AssetMap) InUSD() (total fixedpoint.Value) {
	for _, a := range m {
		if a.InUSD.IsZero() {
			continue
		}

		total = total.Add(a.InUSD)
	}
	return total
}

func (m AssetMap) PlainText() (o string) {
	var assets = m.Slice()

	// sort assets
	sort.Slice(assets, func(i, j int) bool {
		return assets[i].InUSD.Compare(assets[j].InUSD) > 0
	})

	sumUsd := fixedpoint.Zero
	sumBTC := fixedpoint.Zero
	for _, a := range assets {
		usd := a.InUSD
		btc := a.InBTC
		if !a.InUSD.IsZero() {
			o += fmt.Sprintf(" %s: %s (≈ %s) (≈ %s)",
				a.Currency,
				a.NetAsset.String(),
				USD.FormatMoney(usd),
				BTC.FormatMoney(btc),
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
		USD.FormatMoney(sumUsd),
		BTC.FormatMoney(sumBTC),
	)
	return o
}

func (m AssetMap) Slice() (assets []Asset) {
	for _, a := range m {
		assets = append(assets, a)
	}
	return assets
}

func (m AssetMap) SlackAttachment() slack.Attachment {
	var fields []slack.AttachmentField
	var netAssetInBTC, netAssetInUSD fixedpoint.Value

	var assets = m.Slice()

	// sort assets
	sort.Slice(assets, func(i, j int) bool {
		return assets[i].InUSD.Compare(assets[j].InUSD) > 0
	})

	for _, a := range assets {
		netAssetInUSD = netAssetInUSD.Add(a.InUSD)
		netAssetInBTC = netAssetInBTC.Add(a.InBTC)
	}

	for _, a := range assets {
		if !a.InUSD.IsZero() {
			text := fmt.Sprintf("%s (≈ %s) (≈ %s) (%s)",
				a.NetAsset.String(),
				USD.FormatMoney(a.InUSD),
				BTC.FormatMoney(a.InBTC),
				a.InUSD.Div(netAssetInUSD).FormatPercentage(2),
			)

			if !a.Borrowed.IsZero() {
				text += fmt.Sprintf(" Borrowed: %s", a.Borrowed.String())
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
			USD.FormatMoney(netAssetInUSD),
			BTC.FormatMoney(netAssetInBTC),
		),
		Fields: fields,
	}
}
