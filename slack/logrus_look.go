package slack

import (
	"context"
	"github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
)

type SlackLogHook struct {
	Slack        *slack.Client
	ErrorChannel string
}

func (t *SlackLogHook) Levels() []logrus.Level {
	return []logrus.Level{
		// log.InfoLevel,
		logrus.ErrorLevel,
		logrus.PanicLevel,
		// log.WarnLevel,
	}
}

func (t *SlackLogHook) Fire(e *logrus.Entry) error {
	var color = "#F0F0F0"

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

	logerr, ok := e.Data["err"]
	if ok {
		slackAttachments = append(slackAttachments, slack.Attachment{
			Color: color,
			Title: "Error",
			Fields: []slack.AttachmentField{
				{Title: "Error", Value: logerr.(error).Error()},
			},
		})
	}

	_, _, err := t.Slack.PostMessageContext(context.Background(), t.ErrorChannel,
		slack.MsgOptionText(e.Message, true),
		slack.MsgOptionAttachments(slackAttachments...))

	return err
}


