package autoborrow

import (
	"fmt"
	"strings"

	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

// RepaidAlert
type RepaidAlert struct {
	SessionName   string
	Asset         string
	Amount        fixedpoint.Value
	SlackMentions []string
}

func (m *RepaidAlert) SlackAttachment() slack.Attachment {
	return slack.Attachment{
		Color: "red",
		Title: fmt.Sprintf("Margin Repaid on %s session", m.SessionName),
		Text:  strings.Join(m.SlackMentions, " "),
		Fields: []slack.AttachmentField{
			{
				Title: "Session",
				Value: m.SessionName,
				Short: true,
			},
			{
				Title: "Asset",
				Value: m.Amount.String() + " " + m.Asset,
				Short: true,
			},
		},
		// Footer:     "",
		// FooterIcon: "",
	}
}
