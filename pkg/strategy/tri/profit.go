package tri

import (
	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/style"
)

type Profit struct {
	Asset       string           `json:"asset"`
	Profit      fixedpoint.Value `json:"profit"`
	ProfitInUSD fixedpoint.Value `json:"profitInUSD"`
}

func (p *Profit) PlainText() string {
	var title = "Arbitrage Profit "
	title += style.PnLEmojiSimple(p.Profit) + " "
	title += style.PnLSignString(p.Profit) + " " + p.Asset

	if !p.ProfitInUSD.IsZero() {
		title += " ~= " + style.PnLSignString(p.ProfitInUSD) + " USD"
	}
	return title
}

func (p *Profit) SlackAttachment() slack.Attachment {
	var color = style.PnLColor(p.Profit)
	var title = "Triangular PnL "
	title += style.PnLEmojiSimple(p.Profit) + " "
	title += style.PnLSignString(p.Profit) + " " + p.Asset

	if !p.ProfitInUSD.IsZero() {
		title += " ~= " + style.PnLSignString(p.ProfitInUSD) + " USD"
	}

	var fields []slack.AttachmentField
	if !p.Profit.IsZero() {
		fields = append(fields, slack.AttachmentField{
			Title: "Profit",
			Value: style.PnLSignString(p.Profit) + " " + p.Asset,
			Short: true,
		})
	}

	if !p.ProfitInUSD.IsZero() {
		fields = append(fields, slack.AttachmentField{
			Title: "Profit (~= USD)",
			Value: style.PnLSignString(p.ProfitInUSD) + " USD",
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
