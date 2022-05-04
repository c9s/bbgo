package types

import (
	"fmt"
	"sort"
	"time"

	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type Asset struct {
	Currency   string           `json:"currency" db:"currency"`
	Total      fixedpoint.Value `json:"total" db:"total"`
	InUSD      fixedpoint.Value `json:"inUSD" db:"in_usd"`
	InBTC      fixedpoint.Value `json:"inBTC" db:"in_btc"`
	Time       time.Time        `json:"time" db:"time"`
	Locked     fixedpoint.Value `json:"lock" db:"lock" `
	Available  fixedpoint.Value `json:"available"  db:"available"`
	Borrowed   fixedpoint.Value `json:"borrowed" db:"borrowed"`
	NetAsset   fixedpoint.Value `json:"netAsset" db:"net_asset"`
	PriceInUSD fixedpoint.Value `json:"priceInUSD" db:"price_in_usd"`
}

type AssetMap map[string]Asset

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
			o += fmt.Sprintf("  %s: %s (≈ %s) (≈ %s)",
				a.Currency,
				a.Total.String(),
				USD.FormatMoney(usd),
				BTC.FormatMoney(btc),
			) + "\n"
			sumUsd = sumUsd.Add(usd)
			sumBTC = sumBTC.Add(btc)
		} else {
			o += fmt.Sprintf("  %s: %s",
				a.Currency,
				a.Total.String(),
			) + "\n"
		}
	}
	o += fmt.Sprintf(" Summary: (≈ %s) (≈ %s)",
		USD.FormatMoney(sumUsd),
		BTC.FormatMoney(sumBTC),
	) + "\n"
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
	var totalBTC, totalUSD fixedpoint.Value

	var assets = m.Slice()

	// sort assets
	sort.Slice(assets, func(i, j int) bool {
		return assets[i].InUSD.Compare(assets[j].InUSD) > 0
	})

	for _, a := range assets {
		totalUSD = totalUSD.Add(a.InUSD)
		totalBTC = totalBTC.Add(a.InBTC)
	}

	for _, a := range assets {
		if !a.InUSD.IsZero() {
			fields = append(fields, slack.AttachmentField{
				Title: a.Currency,
				Value: fmt.Sprintf("%s (≈ %s) (≈ %s) (%s)",
					a.Total.String(),
					USD.FormatMoney(a.InUSD),
					BTC.FormatMoney(a.InBTC),
					a.InUSD.Div(totalUSD).FormatPercentage(2),
				),
				Short: false,
			})
		} else {
			fields = append(fields, slack.AttachmentField{
				Title: a.Currency,
				Value: fmt.Sprintf("%s", a.Total.String()),
				Short: false,
			})
		}
	}

	return slack.Attachment{
		Title: fmt.Sprintf("Net Asset Value %s (≈ %s)",
			USD.FormatMoney(totalUSD),
			BTC.FormatMoney(totalBTC),
		),
		Fields: fields,
	}
}
