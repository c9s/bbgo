package telegramnotifier

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/types"
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
	var textArgsOffset = -1
	var texts []string

	for idx, arg := range args {
		switch a := arg.(type) {

		case types.PlainText:
			texts = append(texts, a.PlainText())
			textArgsOffset = idx

		}
	}

	var simpleArgs = args
	if textArgsOffset > -1 {
		simpleArgs = args[:textArgsOffset]
	}

	log.Infof(format, simpleArgs...)

	message := fmt.Sprintf(format, simpleArgs...)
	n.interaction.SendToOwner(message)

	for _, text := range texts {
		n.interaction.SendToOwner(text)
	}

}
