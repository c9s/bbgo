package xalign

import (
	"fmt"
	"strings"
	"time"

	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/slack/slackalert"
	"github.com/c9s/bbgo/pkg/types"
)

type CriticalBalanceDiscrepancyAlert struct {
	SlackAlert *slackalert.SlackAlert

	Warning bool

	BaseCurrency      string
	Delta             fixedpoint.Value
	SustainedDuration time.Duration

	QuoteCurrency string
	AlertAmount   fixedpoint.Value

	Side     types.SideType
	Price    fixedpoint.Value
	Quantity fixedpoint.Value
	Amount   fixedpoint.Value
}

func (m *CriticalBalanceDiscrepancyAlert) SlackAttachment() slack.Attachment {
	color := "red"

	if m.Warning {
		color = "yellow"
	}

	titlePrefix := "Critical Balance Discrepancy Alert:"
	if m.Warning {
		titlePrefix = "Critical Balance Discrepancy Warning:"
	}

	if m.Delta.Sign() > 0 {
		m.Side = types.SideTypeBuy
	} else {
		m.Side = types.SideTypeSell
	}

	title := titlePrefix + fmt.Sprintf(" %f %s",
		m.Delta.Float64(),
		m.BaseCurrency,
	)

	if m.SustainedDuration > 0 {
		title += fmt.Sprintf(" sustained for %s", m.SustainedDuration)
	}

	title += fmt.Sprintf(" (~= %f %s > %f %s)",
		m.Amount.Float64(),
		m.QuoteCurrency,
		m.AlertAmount.Float64(),
		m.QuoteCurrency,
	)

	return slack.Attachment{
		Color:  color,
		Title:  title,
		Text:   strings.Join(m.SlackAlert.Mentions, " ") + " Please check the balances",
		Footer: fmt.Sprintf("strategy: %s", ID),
		Fields: []slack.AttachmentField{
			{
				Title: "Base Currency",
				Value: m.BaseCurrency,
				Short: true,
			},
			{
				Title: "Side",
				Value: m.Side.String(),
				Short: true,
			},
			{
				Title: "Price",
				Value: m.Price.String(),
				Short: true,
			},
			{
				Title: "Quantity",
				Value: m.Quantity.String(),
				Short: true,
			},
		},
	}
}
