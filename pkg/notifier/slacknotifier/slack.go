package slacknotifier

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/livenote"
	"github.com/c9s/bbgo/pkg/types"

	log "github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
)

var limiter = rate.NewLimiter(rate.Every(1*time.Second), 3)

type notifyTask struct {
	Channel string
	Opts    []slack.MsgOption
}

type Notifier struct {
	client  *slack.Client
	channel string

	taskC chan notifyTask

	liveNotePool *livenote.Pool
}

type NotifyOption func(notifier *Notifier)

func New(client *slack.Client, channel string, options ...NotifyOption) *Notifier {
	notifier := &Notifier{
		channel:      channel,
		client:       client,
		taskC:        make(chan notifyTask, 100),
		liveNotePool: livenote.NewPool(100),
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
			// ignore the wait error
			_ = limiter.Wait(ctx)

			_, _, err := n.client.PostMessageContext(ctx, task.Channel, task.Opts...)
			if err != nil {
				log.WithError(err).
					WithField("channel", task.Channel).
					Errorf("slack api error: %s", err.Error())
			}
		}
	}
}

type LiveNoteOption interface{}

type Mention struct {
	User string
}

type Comment struct {
	Text string
}

func (n *Notifier) PostLiveNote(obj livenote.Object, opts ...LiveNoteOption) error {
	note := n.liveNotePool.Update(obj)
	ctx := context.Background()

	channel := note.ChannelID
	if channel == "" {
		channel = n.channel
	}

	var attachment slack.Attachment
	if creator, ok := note.Object.(types.SlackAttachmentCreator); ok {
		attachment = creator.SlackAttachment()
	} else {
		return fmt.Errorf("livenote object does not support types.SlackAttachmentCreator interface")
	}

	var slackOpts []slack.MsgOption
	slackOpts = append(slackOpts, slack.MsgOptionAttachments(attachment))

	var mentions []*Mention
	var comments []*Comment
	for _, opt := range opts {
		switch val := opt.(type) {
		case *Mention:
			mentions = append(mentions, val)
		case *Comment:
			comments = append(comments, val)

		}
	}

	if note.MessageID != "" {
		// UpdateMessageContext returns channel, timestamp, text, err
		_, _, _, err := n.client.UpdateMessageContext(ctx, channel, note.MessageID, slackOpts...)
		if err != nil {
			return err
		}

	} else {
		respCh, respTs, err := n.client.PostMessageContext(ctx, channel, slackOpts...)
		if err != nil {
			log.WithError(err).
				WithField("channel", n.channel).
				Errorf("slack api error: %s", err.Error())
			return err
		}

		note.SetChannelID(respCh)
		note.SetMessageID(respTs)
	}

	return nil
}

func (n *Notifier) Notify(obj interface{}, args ...interface{}) {
	// TODO: filter args for the channel option
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

		case *slack.Attachment:
			if firstAttachmentOffset == -1 {
				firstAttachmentOffset = idx
			}

			slackAttachments = append(slackAttachments, *a)

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
