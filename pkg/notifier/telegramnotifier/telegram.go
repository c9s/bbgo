package telegramnotifier

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
	"gopkg.in/tucnak/telebot.v2"

	"github.com/c9s/bbgo/pkg/types"
)

var apiLimiter = rate.NewLimiter(rate.Every(1*time.Second), 1)

var log = logrus.WithField("service", "telegram")

type notifyTask struct {
	message     string
	texts       []string
	photoBuffer *bytes.Buffer
}

type Notifier struct {
	bot *telebot.Bot

	// Subscribers stores the Chat objects for broadcasting public notification
	Subscribers map[int64]time.Time `json:"subscribers"`

	// Chats are the private chats that we will send private notification
	Chats map[int64]*telebot.Chat `json:"chats"`

	broadcast bool

	taskC chan notifyTask
}

type Option func(notifier *Notifier)

func UseBroadcast() Option {
	return func(notifier *Notifier) {
		notifier.broadcast = true
	}
}

// New returns a telegram notifier instance
func New(bot *telebot.Bot, options ...Option) *Notifier {
	notifier := &Notifier{
		bot:         bot,
		Chats:       make(map[int64]*telebot.Chat),
		Subscribers: make(map[int64]time.Time),
		taskC:       make(chan notifyTask, 100),
	}

	for _, o := range options {
		o(notifier)
	}

	go notifier.worker()

	return notifier
}

func (n *Notifier) worker() {
	ctx := context.Background()
	for {
		select {
		case <-ctx.Done():
			return
		case task := <-n.taskC:
			apiLimiter.Wait(ctx)
			n.consume(task)
		}
	}
}

func (n *Notifier) consume(task notifyTask) {
	if n.broadcast {
		if n.Subscribers == nil {
			return
		}
		if task.message != "" {
			n.Broadcast(task.message)
		}
		for _, text := range task.texts {
			n.Broadcast(text)
		}
		if task.photoBuffer == nil {
			return
		}

		for chatID := range n.Subscribers {
			chat, err := n.bot.ChatByID(strconv.FormatInt(chatID, 10))
			if err != nil {
				log.WithError(err).Error("can not get chat by ID")
				continue
			}
			album := telebot.Album{
				photoFromBuffer(task.photoBuffer),
			}
			if _, err := n.bot.SendAlbum(chat, album); err != nil {
				log.WithError(err).Error("failed to send message")
			}
		}
	} else if n.Chats != nil {
		for _, chat := range n.Chats {
			if task.message != "" {
				if _, err := n.bot.Send(chat, task.message); err != nil {
					log.WithError(err).Error("telegram send error")
				}
			}

			for _, text := range task.texts {
				if _, err := n.bot.Send(chat, text); err != nil {
					log.WithError(err).Error("telegram send error")
				}
			}
			if task.photoBuffer != nil {
				album := telebot.Album{
					photoFromBuffer(task.photoBuffer),
				}
				if _, err := n.bot.SendAlbum(chat, album); err != nil {
					log.WithError(err).Error("telegram send error")
				}
			}
		}
	}
}

func (n *Notifier) Notify(obj interface{}, args ...interface{}) {
	n.NotifyTo("", obj, args...)
}

func filterPlaintextMessages(args []interface{}) (texts []string, pureArgs []interface{}) {
	var firstObjectOffset = -1
	for idx, arg := range args {
		rt := reflect.TypeOf(arg)
		if rt.Kind() == reflect.Ptr {
			switch a := arg.(type) {

			case nil:
				texts = append(texts, "nil")
				if firstObjectOffset == -1 {
					firstObjectOffset = idx
				}

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

	select {
	case n.taskC <- notifyTask{
		texts:   texts,
		message: message,
	}:
	default:
		log.Error("[telegram] cannot send task to notify")
	}
}

func (n *Notifier) SendPhoto(buffer *bytes.Buffer) {
	n.SendPhotoTo("", buffer)
}

func photoFromBuffer(buffer *bytes.Buffer) telebot.InputMedia {
	reader := bytes.NewReader(buffer.Bytes())
	return &telebot.Photo{
		File: telebot.FromReader(reader),
	}
}

func (n *Notifier) SendPhotoTo(channel string, buffer *bytes.Buffer) {
	select {
	case n.taskC <- notifyTask{
		photoBuffer: buffer,
	}:
	case <-time.After(50 * time.Millisecond):
		return
	}
}

func (n *Notifier) AddChat(c *telebot.Chat) {
	if n.Chats == nil {
		n.Chats = make(map[int64]*telebot.Chat)
	}
	n.Chats[c.ID] = c
}

func (n *Notifier) AddSubscriber(m *telebot.Message) {
	if n.Subscribers == nil {
		n.Subscribers = make(map[int64]time.Time)
	}

	n.Subscribers[m.Chat.ID] = m.Time()
}

func (n *Notifier) Broadcast(message string) {
	if n.Subscribers == nil {
		return
	}

	for chatID := range n.Subscribers {
		chat, err := n.bot.ChatByID(strconv.FormatInt(chatID, 10))
		if err != nil {
			log.WithError(err).Error("can not get chat by ID")
			continue
		}

		if _, err := n.bot.Send(chat, message); err != nil {
			log.WithError(err).Error("failed to send message")
		}
	}
}
