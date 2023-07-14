package types

import "github.com/slack-go/slack"

type SlackAttachmentCreator interface {
	SlackAttachment() slack.Attachment
}
