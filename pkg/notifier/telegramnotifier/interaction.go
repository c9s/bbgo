package telegramnotifier

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/version"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/sirupsen/logrus"
	"gopkg.in/tucnak/telebot.v2"

	"github.com/c9s/bbgo/pkg/service"
)

var log = logrus.WithField("service", "telegram")

type Session struct {
	Owner              *telebot.User `json:"owner"`
	Chat               *telebot.Chat `json:"chat"`
	OneTimePasswordKey *otp.Key      `json:"otpKey"`
}

func NewSession(key *otp.Key) Session {
	return Session{
		Owner:              nil,
		Chat:               nil,
		OneTimePasswordKey: key,
	}
}

//go:generate callbackgen -type Interaction
type Interaction struct {
	store service.Store

	bot *telebot.Bot

	AuthToken string

	session *Session

	StartCallbacks []func()
	AuthCallbacks  []func(user *telebot.User)
}

func NewInteraction(bot *telebot.Bot, store service.Store) *Interaction {
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
	if it.session.Owner == nil || it.session.Chat == nil {
		return
	}

	if m.Sender.ID != it.session.Owner.ID {
		log.Warningf("incorrect user tried to access bot! sender: %+v", m.Sender)
	} else {
		if _, err := it.bot.Send(it.session.Chat,
			fmt.Sprintf("Welcome! your username: %s, user ID: %d",
				it.session.Owner.Username,
				it.session.Owner.ID,
			)); err != nil {
			log.WithError(err).Error("failed to send telegram message")
		}
	}
}

func (it *Interaction) SendToOwner(message string) {
	if it.session.Chat == nil {
		return
	}

	if _, err := it.bot.Send(it.session.Chat, message); err != nil {
		log.WithError(err).Error("failed to send message to the owner")
	}
}

func (it *Interaction) HandleHelp(m *telebot.Message) {
	message := `
help	- show this help message
auth	- authorize current telegram user to access telegram bot with authentication token or one-time password. ex. /auth my-token
info	- show information about current chat
`
	if _, err := it.bot.Send(m.Chat, message); err != nil {
		log.WithError(err).Error("failed to send help message")
	}
}

func (it *Interaction) HandleAuth(m *telebot.Message) {
	if len(it.AuthToken) > 0 && m.Payload == it.AuthToken {
		it.session.Owner = m.Sender
		it.session.Chat = m.Chat

		if _, err := it.bot.Send(m.Chat, fmt.Sprintf("👋 Hi %s, nice to meet you. 🤝 I will send you the notifications!", m.Sender.Username)); err != nil {
			log.WithError(err).Error("telegram send error")
		}

		if err := it.store.Save(it.session); err != nil {
			log.WithError(err).Error("can not persist telegram chat user")
		}

		it.EmitAuth(m.Sender)

	} else if it.session != nil && it.session.OneTimePasswordKey != nil {

		if totp.Validate(m.Payload, it.session.OneTimePasswordKey.Secret()) {
			it.session.Owner = m.Sender
			it.session.Chat = m.Chat

			if _, err := it.bot.Send(m.Chat, fmt.Sprintf("👋 Hi %s, nice to meet you. 🤝 I will send you the notifications!", m.Sender.Username)); err != nil {
				log.WithError(err).Error("telegram send error")
			}

			if err := it.store.Save(it.session); err != nil {
				log.WithError(err).Error("can not persist telegram chat user")
			}

			it.EmitAuth(m.Sender)

		} else {
			if _, err := it.bot.Send(m.Chat, "Authorization failed. please check your auth token"); err != nil {
				log.WithError(err).Error("telegram send error")
			}
		}

	} else {
		if _, err := it.bot.Send(m.Chat, "Authorization failed. please check your auth token"); err != nil {
			log.WithError(err).Error("telegram send error")
		}
	}
}

func (it *Interaction) Start(session Session) {
	it.session = &session

	if it.session.Owner != nil && it.session.Chat != nil {
		if _, err := it.bot.Send(it.session.Chat, fmt.Sprintf("👋 Hi %s, I'm back, this is version %s, good luck! 🖖",
			it.session.Owner.Username,
			version.Version,
		)); err != nil {
			log.WithError(err).Error("failed to send telegram message")
		}
	}

	it.bot.Start()
}
