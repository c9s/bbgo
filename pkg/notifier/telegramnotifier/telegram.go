package telegramnotifier

import (
	"fmt"
)

type Notifier struct {
	interaction *Interaction
}

type NotifyOption func(notifier *Notifier)

// start bot daemon
func New(interaction *Interaction, options ...NotifyOption) *Notifier {
	notifier := &Notifier{
		interaction: interaction,
	}

	for _, o := range options {
		o(notifier)
	}

	return notifier
}

func (n *Notifier) Notify(format string, args ...interface{}) {
	n.NotifyTo("", format, args...)
}

func (n *Notifier) NotifyTo(_, format string, args ...interface{}) {
	var telegramArgsOffset = -1
	var nonTelegramArgs = args

	if telegramArgsOffset > -1 {
		nonTelegramArgs = args[:telegramArgsOffset]
	}

	log.Infof(format, nonTelegramArgs...)

	message := fmt.Sprintf(format, nonTelegramArgs...)
	n.interaction.SendToOwner(message)
}
