package interact

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"gopkg.in/tucnak/telebot.v2"
)

type TelegramReply struct {
	bot     *telebot.Bot
	message string
	menu    *telebot.ReplyMarkup
	buttons [][]telebot.Btn
}

func (r *TelegramReply) Message(message string) {
	r.message = message
}

func (r *TelegramReply) RemoveKeyboard() {
	r.menu.ReplyKeyboardRemove = true
}

func (r *TelegramReply) AddButton(text string) {
	var button = r.menu.Text(text)
	if len(r.buttons) == 0 {
		r.buttons = append(r.buttons, []telebot.Btn{})
	}
	r.buttons[len(r.buttons)-1] = append(r.buttons[len(r.buttons)-1], button)
}

func (r *TelegramReply) build() {
	var rows []telebot.Row
	for _, buttons := range r.buttons {
		rows = append(rows, telebot.Row(buttons))
	}
	r.menu.Reply(rows...)
}

//go:generate callbackgen -type Telegram
type Telegram struct {
	Interact *Interact
	Bot      *telebot.Bot

	// messageCallbacks is used for interact to register its message handler
	messageCallbacks []func(reply Reply, message string)
}

func (b *Telegram) Start() {
	b.Bot.Handle(telebot.OnText, func(m *telebot.Message) {
		log.Infof("onText: %+v", m)

		reply := b.newReply()
		if err := b.Interact.handleResponse(m.Text, reply); err != nil {
			log.WithError(err).Errorf("response handling error")
		}

		reply.build()
		if _, err := b.Bot.Send(m.Sender, reply.message, reply.menu); err != nil {
			log.WithError(err).Errorf("message send error")
		}
	})
	go b.Bot.Start()
}

func (b *Telegram) AddCommand(command string, responder Responder) {
	b.Bot.Handle(command, func(m *telebot.Message) {
		reply := b.newReply()
		if err := responder(reply, m.Payload); err != nil {
			log.WithError(err).Errorf("responder error")
			b.Bot.Send(m.Sender, fmt.Sprintf("error: %v", err))
			return
		}

		// build up the response objects
		reply.build()
		if _, err := b.Bot.Send(m.Sender, reply.message, reply.menu); err != nil {
			log.WithError(err).Errorf("message send error")
		}
	})
}

func (b *Telegram) newReply() *TelegramReply {
	return &TelegramReply{
		bot:  b.Bot,
		menu: &telebot.ReplyMarkup{ResizeReplyKeyboard: true},
	}
}
