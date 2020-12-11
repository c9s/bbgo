package telegramnotifier

import (
	"fmt"

	"github.com/pquerna/otp"
	"github.com/sirupsen/logrus"
	"gopkg.in/tucnak/telebot.v2"

	"github.com/c9s/bbgo/pkg/bbgo"
)

var log = logrus.WithField("service", "telegram")

//go:generate callbackgen -type Interaction
type Interaction struct {
	store bbgo.Store
	bot   *telebot.Bot

	AuthToken          string
	OneTimePasswordKey *otp.Key

	Owner *telebot.User

	StartCallbacks []func()
	AuthCallbacks  []func(user *telebot.User)
}

func NewInteraction(bot *telebot.Bot, store bbgo.Store) *Interaction {
	interaction := &Interaction{
		store: store,
		bot:   bot,
	}

	bot.Handle("/help", interaction.HandleHelp)
	bot.Handle("/auth", interaction.HandleAuth)
	bot.Handle("/info", interaction.HandleInfo)
	return interaction
}

func (it *Interaction) HandleInfo(m *telebot.Message) {
	if it.Owner == nil {
		return
	}

	if m.Sender.ID != it.Owner.ID {
		log.Warningf("incorrect user tried to access bot! sender: %+v", m.Sender)
	} else {
		if _, err := it.bot.Send(it.Owner,
			fmt.Sprintf("Welcome! your username: %s, user ID: %d",
				it.Owner.Username,
				it.Owner.ID,
			)); err != nil {
			log.WithError(err).Error("failed to send telegram message")
		}
	}
}

func (it *Interaction) SendToOwner(message string) {
	if it.Owner == nil {
		log.Warnf("the telegram owner is authorized yet")
		return
	}

	if _, err := it.bot.Send(it.Owner, message); err != nil {
		log.WithError(err).Error("failed to send message to the owner")
	}
}

func (it *Interaction) HandleHelp(m *telebot.Message) {
	message := `
help	- show this help message
auth	- authorize current telegram user to access telegram bot with authToken. ex. /auth my-token
info	- show information about current chat
`
	if _, err := it.bot.Send(m.Sender, message); err != nil {
		log.WithError(err).Error("failed to send help message")
	}
}

func (it *Interaction) HandleAuth(m *telebot.Message) {
	if m.Payload == it.AuthToken {
		it.Owner = m.Sender
		if _, err := it.bot.Send(m.Sender, fmt.Sprintf("Hi %s, I know you, I will send you the notifications!", m.Sender.Username)); err != nil {
			log.WithError(err).Error("telegram send error")
		}

		if err := it.store.Save(it.Owner); err != nil {
			log.WithError(err).Error("can not persist telegram chat user")
		}

		it.EmitAuth(m.Sender)
	} else {
		if _, err := it.bot.Send(m.Sender, "Authorization failed. please check your auth token"); err != nil {
			log.WithError(err).Error("telegram send error")
		}
	}
}

func (it *Interaction) Start() {
	// load user data from persistence layer
	var owner telebot.User

	if err := it.store.Load(&owner); err == nil {
		if _, err := it.bot.Send(it.Owner, fmt.Sprintf("Hi %s, I'm back", it.Owner.Username)); err != nil {
			log.WithError(err).Error("failed to send telegram message")
		}

		it.Owner = &owner
	}

	it.bot.Start()
}
