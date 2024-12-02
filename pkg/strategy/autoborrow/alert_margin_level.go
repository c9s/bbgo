package autoborrow

import (
	"fmt"
	"strings"

	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/slack/slackalert"
	"github.com/c9s/bbgo/pkg/types"
)

type MarginLevelAlertConfig struct {
	slackalert.SlackAlert
	Interval  types.Duration   `json:"interval"`
	MinMargin fixedpoint.Value `json:"minMargin"`
}

// MarginLevelAlert is used to send the slack mention alerts when the current margin is less than the required margin level
type MarginLevelAlert struct {
	CurrentMarginLevel fixedpoint.Value
	MinimalMarginLevel fixedpoint.Value
	SlackMentions      []string
	SessionName        string
}

func (m *MarginLevelAlert) SlackAttachment() slack.Attachment {
	return slack.Attachment{
		Color: "red",
		Title: fmt.Sprintf("Margin Level Alert: %s session - current margin level %f < required margin level %f",
			m.SessionName, m.CurrentMarginLevel.Float64(), m.MinimalMarginLevel.Float64()),
		Text: strings.Join(m.SlackMentions, " "),
		Fields: []slack.AttachmentField{
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
		},
		// Footer:     "",
		// FooterIcon: "",
	}
}
