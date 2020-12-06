package telegramnotifier

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
)

type Notifier struct {
	Bot      *tb.Bot
	chatUser *tb.User
	channel  string
}

type NotifyOption func(notifier *Notifier)

// start bot daemon
func New(botToken, authToken string, options ...NotifyOption) *Notifier {

	notifier := &Notifier{
		chatUser: &tb.User{},
		Bot:      &tb.Bot{},
	}

	for _, o := range options {
		o(notifier)
	}

	bot, err := tb.NewBot(tb.Settings{
		// You can also set custom API URL.
		// If field is empty it equals to "https://api.telegram.org".
		// URL: "http://195.129.111.17:8012",

		Token:  botToken,
		Poller: &tb.LongPoller{Timeout: 10 * time.Second},
	})

	if err != nil {
		panic(err)
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
		log.Info("Receive message: ", m) //debug
		if m.Payload == authToken {
			notifier.chatUser = m.Sender
			bot.Send(m.Sender, "User authorized")
		} else {
			bot.Send(m.Sender, "Error: User authorization failed. Auth token does not match!")
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
