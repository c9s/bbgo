package telegramnotifier

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/types"
)

type Notifier struct {
	interaction *Interaction
	broadcast   bool
}

type Option func(notifier *Notifier)

func UseBroadcast() Option {
	return func(notifier *Notifier) {
		notifier.broadcast = true
	}
}

// New
// TODO: register interaction with channel, so that we can route message to the specific telegram bot
func New(interaction *Interaction, options ...Option) *Notifier {
	notifier := &Notifier{
		interaction: interaction,
	}

	for _, o := range options {
		o(notifier)
	}

	return notifier
}

func (n *Notifier) Notify(obj interface{}, args ...interface{}) {
	n.NotifyTo("", obj, args...)
}

func filterPlaintextMessages(args []interface{}) (texts []string, pureArgs []interface{}) {
	var firstObjectOffset = -1
	for idx, arg := range args {
		switch a := arg.(type) {

		case types.PlainText:
			texts = append(texts, a.PlainText())
			if firstObjectOffset == -1 {
				firstObjectOffset = idx
			}

		case types.Stringer:
			texts = append(texts, a.String())
			if firstObjectOffset == -1 {
				firstObjectOffset = idx
			}
		}
	}

	pureArgs = args
	if firstObjectOffset > -1 {
		pureArgs = args[:firstObjectOffset]
	}

	return texts, pureArgs
}

func (n *Notifier) NotifyTo(channel string, obj interface{}, args ...interface{}) {
	var texts, pureArgs = filterPlaintextMessages(args)
	var message string

	switch a := obj.(type) {

	case string:
		message = fmt.Sprintf(a, pureArgs...)

	case types.PlainText:
		message = a.PlainText()

	case types.Stringer:
		message = a.String()

	default:
		log.Errorf("unsupported notification format: %T %+v", a, a)

	}

	if n.broadcast {
		n.interaction.Broadcast(message)
		for _, text := range texts {
			n.interaction.Broadcast(text)
		}
	} else {
		n.interaction.SendToOwner(message)
		for _, text := range texts {
			n.interaction.SendToOwner(text)
		}
	}
}
