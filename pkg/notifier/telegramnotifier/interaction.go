package telegramnotifier

import (
	"fmt"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/sirupsen/logrus"
	"gopkg.in/tucnak/telebot.v2"

	"github.com/c9s/bbgo/pkg/bbgo"
)

var log = logrus.WithField("service", "telegram")

type Session struct {
	Owner              *telebot.User `json:"owner"`
	OneTimePasswordKey *otp.Key      `json:"otpKey"`
}

func NewSession(key *otp.Key) Session {
	return Session{
		Owner:              nil,
		OneTimePasswordKey: key,
	}
}

//go:generate callbackgen -type Interaction
type Interaction struct {
	store bbgo.Store

	bot *telebot.Bot

	AuthToken string

	session *Session

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

func (it *Interaction) SetAuthToken(token string) {
	it.AuthToken = token
}

func (it *Interaction) Session() *Session {
	return it.session
}

func (it *Interaction) HandleInfo(m *telebot.Message) {
	if it.session.Owner == nil {
		return
	}

	if m.Sender.ID != it.session.Owner.ID {
		log.Warningf("incorrect user tried to access bot! sender: %+v", m.Sender)
	} else {
		if _, err := it.bot.Send(it.session.Owner,
			fmt.Sprintf("Welcome! your username: %s, user ID: %d",
				it.session.Owner.Username,
				it.session.Owner.ID,
			)); err != nil {
			log.WithError(err).Error("failed to send telegram message")
		}
	}
}

func (it *Interaction) SendToOwner(message string) {
	if it.session.Owner == nil {
		return
	}

	if _, err := it.bot.Send(it.session.Owner, message); err != nil {
		log.WithError(err).Error("failed to send message to the owner")
	}
}

func (it *Interaction) HandleHelp(m *telebot.Message) {
	message := `
help	- show this help message
auth	- authorize current telegram user to access telegram bot with authentication token or one-time password. ex. /auth my-token
info	- show information about current chat
`
	if _, err := it.bot.Send(m.Sender, message); err != nil {
		log.WithError(err).Error("failed to send help message")
	}
}

func (it *Interaction) HandleAuth(m *telebot.Message) {
	if len(it.AuthToken) > 0 && m.Payload == it.AuthToken {
		it.session.Owner = m.Sender
		if _, err := it.bot.Send(m.Sender, fmt.Sprintf("Hi %s, I know you, I will send you the notifications!", m.Sender.Username)); err != nil {
			log.WithError(err).Error("telegram send error")
		}

		if err := it.store.Save(it.session); err != nil {
			log.WithError(err).Error("can not persist telegram chat user")
		}

		it.EmitAuth(m.Sender)

	} else if it.session != nil && it.session.OneTimePasswordKey != nil {

		if totp.Validate(m.Payload, it.session.OneTimePasswordKey.Secret()) {
			it.session.Owner = m.Sender

			if _, err := it.bot.Send(m.Sender, fmt.Sprintf("Hi %s, I know you, I will send you the notifications!", m.Sender.Username)); err != nil {
				log.WithError(err).Error("telegram send error")
			}

			if err := it.store.Save(it.session); err != nil {
				log.WithError(err).Error("can not persist telegram chat user")
			}

			it.EmitAuth(m.Sender)

		} else {
			if _, err := it.bot.Send(m.Sender, "Authorization failed. please check your auth token"); err != nil {
				log.WithError(err).Error("telegram send error")
			}
		}

	} else {
		if _, err := it.bot.Send(m.Sender, "Authorization failed. please check your auth token"); err != nil {
			log.WithError(err).Error("telegram send error")
		}
	}
}

func (it *Interaction) Start(session Session) {
	it.session = &session

	if it.session.Owner != nil {
		if _, err := it.bot.Send(it.session.Owner, fmt.Sprintf("Hi %s, I'm back", it.session.Owner.Username)); err != nil {
			log.WithError(err).Error("failed to send telegram message")
		}
	}

	it.bot.Start()
}
