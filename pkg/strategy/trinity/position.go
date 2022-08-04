package trinity

import (
	"fmt"

	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

type MultiCurrencyPosition struct {
	Currencies map[string]fixedpoint.Value
	Markets    map[string]types.Market
}

func (p *MultiCurrencyPosition) handleTrade(trade types.Trade) {
	market := p.Markets[trade.Symbol]
	switch trade.Side {
	case types.SideTypeBuy:
		p.Currencies[market.BaseCurrency] = p.Currencies[market.BaseCurrency].Add(trade.Quantity)
		p.Currencies[market.QuoteCurrency] = p.Currencies[market.QuoteCurrency].Sub(trade.QuoteQuantity)

	case types.SideTypeSell:
		p.Currencies[market.BaseCurrency] = p.Currencies[market.BaseCurrency].Sub(trade.Quantity)
		p.Currencies[market.QuoteCurrency] = p.Currencies[market.QuoteCurrency].Add(trade.QuoteQuantity)
	}
}

type Profit struct {
	Asset  string           `json:"asset"`
	Profit fixedpoint.Value `json:"profit"`
}

func (p *Profit) PlainText() string {
	var title = fmt.Sprintf("Triangular PnL ")
	title += util.PnLEmojiSimple(p.Profit) + " "
	title += util.PnLSignString(p.Profit) + " " + p.Asset
	return title
}

func (p *Profit) SlackAttachment() slack.Attachment {
	var color = util.PnLColor(p.Profit)
	var title = fmt.Sprintf("Triangular PnL ")
	title += util.PnLEmojiSimple(p.Profit) + " "
	title += util.PnLSignString(p.Profit) + " " + p.Asset

	var fields []slack.AttachmentField
	if !p.Profit.IsZero() {
		fields = append(fields, slack.AttachmentField{
			Title: "Profit",
			Value: util.PnLSignString(p.Profit) + " " + p.Asset,
			Short: true,
		})
	}

	return slack.Attachment{
		Color:  color,
		Title:  title,
		Fields: fields,
		// Footer:        "",
	}
}

func (p *MultiCurrencyPosition) CollectProfits() []Profit {
	var profits []Profit
	for currency, base := range p.Currencies {
		if base.IsZero() {
			continue
		}

		profits = append(profits, Profit{
			Asset:  currency,
			Profit: base,
		})
	}

	return profits
}

func (p *MultiCurrencyPosition) Reset() {
	for currency := range p.Currencies {
		p.Currencies[currency] = fixedpoint.Zero
	}
}

func (p *MultiCurrencyPosition) String() (o string) {
	for currency, base := range p.Currencies {
		if base.IsZero() {
			continue
		}

		o += fmt.Sprintf("base %s: %f\n", currency, base.Float64())
	}

	return o
}
