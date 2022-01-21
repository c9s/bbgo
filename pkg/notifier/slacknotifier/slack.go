package slacknotifier

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/types"

	log "github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
)

var limiter = rate.NewLimiter(rate.Every(1*time.Second), 1)

type notifyTask struct {
	Channel string
	Opts    []slack.MsgOption
}

type slackAttachmentCreator interface {
	SlackAttachment() slack.Attachment
}

type Notifier struct {
	client  *slack.Client
	channel string

	taskC chan notifyTask
}

type NotifyOption func(notifier *Notifier)

func New(client *slack.Client, channel string, options ...NotifyOption) *Notifier {
	// var client = slack.New(token, slack.OptionDebug(true))
	notifier := &Notifier{
		channel: channel,
		client:  client,
		taskC:   make(chan notifyTask, 100),
	}

	for _, o := range options {
		o(notifier)
	}

	go notifier.worker()

	return notifier
}

func (n *Notifier) worker() {
	ctx := context.Background()
	for {
		select {
		case <-ctx.Done():
			return

		case task := <-n.taskC:
			limiter.Wait(ctx)
			_, _, err := n.client.PostMessageContext(ctx, task.Channel, task.Opts...)
			if err != nil {
				log.WithError(err).
					WithField("channel", task.Channel).
					Errorf("slack api error: %s", err.Error())
			}
		}
	}
}

func (n *Notifier) Notify(obj interface{}, args ...interface{}) {
	n.NotifyTo(n.channel, obj, args...)
}

func filterSlackAttachments(args []interface{}) (slackAttachments []slack.Attachment, pureArgs []interface{}) {
	var firstAttachmentOffset = -1
	for idx, arg := range args {
		switch a := arg.(type) {

		// concrete type assert first
		case slack.Attachment:
			if firstAttachmentOffset == -1 {
				firstAttachmentOffset = idx
			}

			slackAttachments = append(slackAttachments, a)

		case slackAttachmentCreator:
			if firstAttachmentOffset == -1 {
				firstAttachmentOffset = idx
			}

			slackAttachments = append(slackAttachments, a.SlackAttachment())

		case types.PlainText:
			if firstAttachmentOffset == -1 {
				firstAttachmentOffset = idx
			}

			// fallback to PlainText if it's not supported
			// convert plain text to slack attachment
			text := a.PlainText()
			slackAttachments = append(slackAttachments, slack.Attachment{
				Title: text,
			})
		}
	}

	pureArgs = args
	if firstAttachmentOffset > -1 {
		pureArgs = args[:firstAttachmentOffset]
	}

	return slackAttachments, pureArgs
}

func (n *Notifier) NotifyTo(channel string, obj interface{}, args ...interface{}) {
	if len(channel) == 0 {
		channel = n.channel
	}

	slackAttachments, pureArgs := filterSlackAttachments(args)

	var opts []slack.MsgOption

	switch a := obj.(type) {
	case string:
		opts = append(opts, slack.MsgOptionText(fmt.Sprintf(a, pureArgs...), true),
			slack.MsgOptionAttachments(slackAttachments...))

	case slack.Attachment:
		opts = append(opts, slack.MsgOptionAttachments(append([]slack.Attachment{a}, slackAttachments...)...))

	case slackAttachmentCreator:
		// convert object to slack attachment (if supported)
		opts = append(opts, slack.MsgOptionAttachments(append([]slack.Attachment{a.SlackAttachment()}, slackAttachments...)...))

	default:
		log.Errorf("slack message conversion error, unsupported object: %T %+v", a, a)

	}

	select {
	case n.taskC <- notifyTask{
		Channel: channel,
		Opts:    opts,
	}:
	case <-time.After(50 * time.Millisecond):
		return
	}
}

/*
func (n *Notifier) NotifyTrade(trade *types.Trade) {
	_, _, err := n.client.PostMessageContext(context.Background(), n.TradeChannel,
		slack.MsgOptionText(util.Render(`:handshake: {{ .Symbol }} {{ .Side }} Trade Execution @ {{ .Price  }}`, trade), true),
		slack.MsgOptionAttachments(trade.SlackAttachment()))

	if err != nil {
		logrus.WithError(err).Error("slack send error")
	}
}
*/

/*
func (n *Notifier) NotifyPnL(report *pnl.AverageCostPnlReport) {
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
*/
