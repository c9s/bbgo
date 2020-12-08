package telegramnotifier

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"

	"github.com/c9s/bbgo/pkg/bbgo"
)

type Notifier struct {
	Bot      *tb.Bot
	chatUser *tb.User
	channel  string

	redis *bbgo.RedisPersistenceService
}

type NotifyOption func(notifier *Notifier)

func WithRedisPersistence(redis *bbgo.RedisPersistenceService) NotifyOption {
	return func(notifier *Notifier) {
		notifier.redis = redis
	}
}

// start bot daemon
func New(bot *tb.Bot, authToken string, options ...NotifyOption) *Notifier {
	notifier := &Notifier{
		chatUser: &tb.User{},
		Bot:      bot,
	}

	for _, o := range options {
		o(notifier)
	}

	// use token prefix as the redis namespace
	tt := strings.Split(bot.Token, ":")
	store := notifier.redis.NewStore("bbgo", "telegram", tt[0])
	if err := store.Load(notifier.chatUser) ; err == nil {
		bot.Send(notifier.chatUser, fmt.Sprintf("Hi %s, I'm back", notifier.chatUser.Username))
	}

	bot.Handle("/help", func(m *tb.Message) {
		helpMsg := `
help	- print help message
auth	- authorize current telegram user to access telegram bot with authToken. ex. /auth my-token
info	- print information about current chat
`
		bot.Send(m.Sender, helpMsg)
	})

	// auth check authToken and then set sender id
	bot.Handle("/auth", func(m *tb.Message) {
		log.Info("receive message: ", m) //debug
		if m.Payload == authToken {
			notifier.chatUser = m.Sender
			if err := store.Save(notifier.chatUser); err != nil {
				log.WithError(err).Error("can not persist telegram chat user")
			}

			if _, err := bot.Send(m.Sender, fmt.Sprintf("Hi %s, I know you, I will send you the notifications!", m.Sender.Username)) ; err != nil {
				log.WithError(err).Error("telegram send error")
			}
		} else {
			if _, err := bot.Send(m.Sender, "Authorization failed. please check your auth token") ; err != nil {
				log.WithError(err).Error("telegram send error")
			}
		}
	})

	bot.Handle("/info", func(m *tb.Message) {
		if m.Sender.ID == notifier.chatUser.ID {
			bot.Send(notifier.chatUser,
				fmt.Sprintf("Welcome! your username: %s, user ID: %d",
					notifier.chatUser.Username,
					notifier.chatUser.ID,
				))
		} else {
			log.Warningf("Incorrect user tried to access bot! sender username: %s id: %d", m.Sender.Username, m.Sender.ID)
		}
	})

	go bot.Start()

	notifier.Bot = bot
	return notifier
}

func (n *Notifier) Notify(format string, args ...interface{}) {
	n.NotifyTo(n.channel, format, args...)
}

func (n *Notifier) NotifyTo(channel, format string, args ...interface{}) {
	if n.chatUser.ID == 0 {
		log.Warningf("Telegram bot has no authenticated user. Skip notification")
		return
	}

	var telegramArgsOffset = -1

	var nonTelegramArgs = args
	if telegramArgsOffset > -1 {
		nonTelegramArgs = args[:telegramArgsOffset]
	}

	log.Infof(format, nonTelegramArgs...)

	_, err := n.Bot.Send(n.chatUser, fmt.Sprintf(format, nonTelegramArgs...))
	if err != nil {
		log.WithError(err).
			WithField("chatUser", n.chatUser).
			Errorf("telegram error: %s", err.Error())
	}

	return
}
