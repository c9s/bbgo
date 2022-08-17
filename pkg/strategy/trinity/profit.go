package trinity

import (
	"fmt"

	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/util"
)

type Profit struct {
	Asset       string           `json:"asset"`
	Profit      fixedpoint.Value `json:"profit"`
	ProfitInUSD fixedpoint.Value `json:"profitInUSD"`
}

func (p *Profit) PlainText() string {
	var title = fmt.Sprintf("Arbitrage Profit ")
	title += util.PnLEmojiSimple(p.Profit) + " "
	title += util.PnLSignString(p.Profit) + " " + p.Asset

	if !p.ProfitInUSD.IsZero() {
		title += " ~= " + util.PnLSignString(p.ProfitInUSD) + " USD"
	}
	return title
}

func (p *Profit) SlackAttachment() slack.Attachment {
	var color = util.PnLColor(p.Profit)
	var title = fmt.Sprintf("Triangular PnL ")
	title += util.PnLEmojiSimple(p.Profit) + " "
	title += util.PnLSignString(p.Profit) + " " + p.Asset

	if !p.ProfitInUSD.IsZero() {
		title += " ~= " + util.PnLSignString(p.ProfitInUSD) + " USD"
	}

	var fields []slack.AttachmentField
	if !p.Profit.IsZero() {
		fields = append(fields, slack.AttachmentField{
			Title: "Profit",
			Value: util.PnLSignString(p.Profit) + " " + p.Asset,
			Short: true,
		})
	}

	if !p.ProfitInUSD.IsZero() {
		fields = append(fields, slack.AttachmentField{
			Title: "Profit (~= USD)",
			Value: util.PnLSignString(p.ProfitInUSD) + " USD",
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
