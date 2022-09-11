package slacklog

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
	"golang.org/x/time/rate"
)

var limiter = rate.NewLimiter(rate.Every(time.Minute), 3)

type LogHook struct {
	Slack        *slack.Client
	ErrorChannel string
}

func NewLogHook(token string, channel string) *LogHook {
	var client = slack.New(token)
	return &LogHook{
		Slack:        client,
		ErrorChannel: channel,
	}
}

func (t *LogHook) Levels() []logrus.Level {
	return []logrus.Level{
		// log.InfoLevel,
		logrus.ErrorLevel,
		logrus.PanicLevel,
		// log.WarnLevel,
	}
}

func (t *LogHook) Fire(e *logrus.Entry) error {
	if !limiter.Allow() {
		return nil
	}

	var color string

	switch e.Level {
	case logrus.DebugLevel:
		color = "#9B30FF"
	case logrus.InfoLevel:
		color = "good"
	case logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel:
		color = "danger"
	default:
		color = "warning"
	}

	var slackAttachments []slack.Attachment = nil

	// error fields
	var fields []slack.AttachmentField
	for k, d := range e.Data {
		fields = append(fields, slack.AttachmentField{
			Title: k, Value: fmt.Sprintf("%v", d),
		})
	}

	slackAttachments = append(slackAttachments, slack.Attachment{
		Color:  color,
		Title:  strings.ToUpper(e.Level.String()),
		Fields: fields,
	})

	_, _, err := t.Slack.PostMessageContext(context.Background(), t.ErrorChannel,
		slack.MsgOptionText(":balloon: "+e.Message, true),
		slack.MsgOptionAttachments(slackAttachments...))

	return err
}
