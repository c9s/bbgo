package autoborrow

import (
	"fmt"
	"time"

	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/slack/slackalert"
	"github.com/c9s/bbgo/pkg/types"
)

type MarginLevelAlertConfig struct {
	Slack     *slackalert.SlackAlert `json:"slack"`
	Interval  types.Duration         `json:"interval"`
	MinMargin fixedpoint.Value       `json:"minMargin"`
}

// MarginLevelAlert is used to send the slack mention alerts when the current margin is less than the required margin level
type MarginLevelAlert struct {
	AccountLabel       string
	CurrentMarginLevel fixedpoint.Value
	MinimalMarginLevel fixedpoint.Value
	SessionName        string
	Exchange           types.ExchangeName
	Debts              types.BalanceMap
}

func (m *MarginLevelAlert) ObjectID() string {
	return m.AccountLabel
}

func (m *MarginLevelAlert) SlackAttachment() slack.Attachment {
	var fields []slack.AttachmentField

	if len(m.AccountLabel) > 0 {
		fields = append(fields, slack.AttachmentField{
			Title: "Account",
			Value: m.AccountLabel,
		})
	}

	if len(m.Exchange) > 0 {
		fields = append(fields, slack.AttachmentField{
			Title: "Exchange",
			Value: string(m.Exchange),
		})
	}

	fields = append(fields, []slack.AttachmentField{
		{
			Title: "Session",
			Value: m.SessionName,
			Short: true,
		},
		{
			Title: "Current Margin Level",
			Value: m.CurrentMarginLevel.String(),
			Short: true,
		},
		{
			Title: "Minimal Margin Level",
			Value: m.MinimalMarginLevel.String(),
			Short: true,
		},
	}...)

	// collect the current debts into the alert fields
	if m.Debts != nil && len(m.Debts) > 0 {
		fields = append(fields, m.Debts.SlackAttachment().Fields...)
	}

	color := "good"
	isDanger := m.CurrentMarginLevel.Compare(m.MinimalMarginLevel) <= 0
	if isDanger {
		color = "danger"
	}

	isSolved := !isDanger
	if isSolved {
		fields = append(fields, slack.AttachmentField{
			Title: "Status",
			Value: "✅ Solved",
		})
	}

	footer := fmt.Sprintf("%s - %s", m.Exchange, time.Now().String())

	return slack.Attachment{
		Color: color,
		Title: fmt.Sprintf("Margin Level Alert: %s session",
			m.SessionName,
		),
		Text: fmt.Sprintf("The current margin level %f is less than required margin level %f",
			m.CurrentMarginLevel.Float64(),
			m.MinimalMarginLevel.Float64(),
		),
		Fields:     fields,
		Footer:     footer,
		FooterIcon: types.ExchangeFooterIcon(m.Exchange),
	}
}
