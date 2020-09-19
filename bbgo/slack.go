package bbgo

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/bbgo/types"
	"github.com/c9s/bbgo/pkg/util"
)

type Notifier interface {
	Notify(format string, args ...interface{})
}

type NullNotifier struct{}

func (n *NullNotifier) Notify(format string, args ...interface{}) {
}

type SlackAttachmentCreator interface {
	SlackAttachment() slack.Attachment
}

type SlackNotifier struct {
	Slack *slack.Client

	TradeChannel string
	ErrorChannel string
	InfoChannel  string
}

func (n *SlackNotifier) Notify(format string, args ...interface{}) {
	var slackAttachments []slack.Attachment
	var slackArgsOffset = -1

	for idx, arg := range args {
		switch a := arg.(type) {

		// concrete type assert first
		case slack.Attachment:
			if slackArgsOffset == -1 {
				slackArgsOffset = idx
			}

			slackAttachments = append(slackAttachments, a)

		case SlackAttachmentCreator:
			if slackArgsOffset == -1 {
				slackArgsOffset = idx
			}

			slackAttachments = append(slackAttachments, a.SlackAttachment())

		}
	}

	var nonSlackArgs = args
	if slackArgsOffset > -1 {
		nonSlackArgs = args[:slackArgsOffset]
	}

	logrus.Infof(format, nonSlackArgs...)

	_, _, err := n.Slack.PostMessageContext(context.Background(), n.InfoChannel,
		slack.MsgOptionText(fmt.Sprintf(format, nonSlackArgs...), true),
		slack.MsgOptionAttachments(slackAttachments...))
	if err != nil {
		logrus.WithError(err).Errorf("slack error: %s", err.Error())
	}
}

func (n *SlackNotifier) ReportTrade(trade *types.Trade) {
	_, _, err := n.Slack.PostMessageContext(context.Background(), n.TradeChannel,
		slack.MsgOptionText(util.Render(`:handshake: {{ .Symbol }} {{ .Side }} Trade Execution @ {{ .Price  }}`, trade), true),
		slack.MsgOptionAttachments(trade.SlackAttachment()))

	if err != nil {
		logrus.WithError(err).Error("slack send error")
	}
}

func (n *SlackNotifier) ReportPnL(report *ProfitAndLossReport) {
	attachment := report.SlackAttachment()

	_, _, err := n.Slack.PostMessageContext(context.Background(), n.TradeChannel,
		slack.MsgOptionText(util.Render(
			`:heavy_dollar_sign: Here is your *{{ .symbol }}* PnL report collected since *{{ .startTime }}*`,
			map[string]interface{}{
				"symbol":    report.Symbol,
				"startTime": report.StartTime.Format(time.RFC822),
			}), true),
		slack.MsgOptionAttachments(attachment))

	if err != nil {
		logrus.WithError(err).Errorf("slack send error")
	}
}

