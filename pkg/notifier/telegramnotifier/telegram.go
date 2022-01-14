package telegramnotifier

import (
	"fmt"
	"strconv"
	"time"

	"gopkg.in/tucnak/telebot.v2"

	"github.com/c9s/bbgo/pkg/types"
)

type Notifier struct {
	Bot *telebot.Bot

	// Chats stores the Chat objects for broadcasting
	Chats map[int64]time.Time `json:"chats"`

	// Owner
	// when owner and owner chat is not nil, notification will send to the owner chat
	Owner     *telebot.User `json:"owner"`
	OwnerChat *telebot.Chat `json:"chat"`

	broadcast bool
}

type Option func(notifier *Notifier)

func UseBroadcast() Option {
	return func(notifier *Notifier) {
		notifier.broadcast = true
	}
}

// New
func New(options ...Option) *Notifier {
	notifier := &Notifier{}

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
		n.Broadcast(message)
		for _, text := range texts {
			n.Broadcast(text)
		}
	} else if n.OwnerChat != nil {
		n.SendToOwner(message)
		for _, text := range texts {
			n.SendToOwner(text)
		}
	}
}

func (n *Notifier) AddSubscriber(m *telebot.Message) {
	if n.Chats == nil {
		n.Chats = make(map[int64]time.Time)
	}

	n.Chats[m.Chat.ID] = m.Time()
}

func (n *Notifier) SetOwner(owner *telebot.User, chat *telebot.Chat) {
	n.Owner = owner
	n.OwnerChat = chat
}

func (n *Notifier) SendToOwner(message string) {
	if _, err := n.Bot.Send(n.OwnerChat, message); err != nil {
		log.WithError(err).Error("telegram send error")
	}
}

func (n *Notifier) Broadcast(message string) {
	for chatID := range n.Chats {
		chat, err := n.Bot.ChatByID(strconv.FormatInt(chatID, 10))
		if err != nil {
			log.WithError(err).Error("can not get chat by ID")
			continue
		}

		if _, err := n.Bot.Send(chat, message); err != nil {
			log.WithError(err).Error("failed to send message")
		}
	}
}
