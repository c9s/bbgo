package slacknotifier

import (
	"bytes"
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

type Notifier struct {
	client  *slack.Client
	channel string

	taskC chan notifyTask
}

type NotifyOption func(notifier *Notifier)

func New(client *slack.Client, channel string, options ...NotifyOption) *Notifier {
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

		case types.SlackAttachmentCreator:
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

	case types.SlackAttachmentCreator:
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

func (n *Notifier) SendPhoto(buffer *bytes.Buffer) {
	n.SendPhotoTo(n.channel, buffer)
}

func (n *Notifier) SendPhotoTo(channel string, buffer *bytes.Buffer) {
	// TODO
}
