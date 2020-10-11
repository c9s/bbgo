package slacknotifier

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/accounting"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

type SlackAttachmentCreator interface {
	SlackAttachment() slack.Attachment
}

type Notifier struct {
	client *slack.Client

	Channel string

	TradeChannel string
	PnlChannel   string
}

type NotifyOption func(notifier *Notifier)

func TradeChannel(channel string) NotifyOption {
	return func(notifier *Notifier) {
		notifier.TradeChannel = channel
	}
}

func PnlChannel(channel string) NotifyOption {
	return func(notifier *Notifier) {
		notifier.PnlChannel = channel
	}
}

func New(token string, channel string, options ...NotifyOption) *Notifier {
	var client = slack.New(token, slack.OptionDebug(true))
	notifier := &Notifier{
		client:       client,
		Channel:      channel,
		TradeChannel: channel,
		PnlChannel:   channel,
	}

	for _, o := range options {
		o(notifier)
	}

	return notifier
}

func (n *Notifier) Notify(format string, args ...interface{}) {
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

	_, _, err := n.client.PostMessageContext(context.Background(), n.Channel,
		slack.MsgOptionText(fmt.Sprintf(format, nonSlackArgs...), true),
		slack.MsgOptionAttachments(slackAttachments...))
	if err != nil {
		logrus.WithError(err).Errorf("slack error: %s", err.Error())
	}
}

func (n *Notifier) NotifyTrade(trade *types.Trade) {
	_, _, err := n.client.PostMessageContext(context.Background(), n.TradeChannel,
		slack.MsgOptionText(util.Render(`:handshake: {{ .Symbol }} {{ .Side }} Trade Execution @ {{ .Price  }}`, trade), true),
		slack.MsgOptionAttachments(trade.SlackAttachment()))

	if err != nil {
		logrus.WithError(err).Error("slack send error")
	}
}

func (n *Notifier) NotifyPnL(report *accounting.ProfitAndLossReport) {
	attachment := report.SlackAttachment()

	_, _, err := n.client.PostMessageContext(context.Background(), n.PnlChannel,
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
