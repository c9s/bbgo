package interact

import (
	"context"
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

type TelegramAuthorizer struct {
	Telegram *Telegram
	Message  *telebot.Message
}

func (a *TelegramAuthorizer) Authorize() error {
	a.Telegram.Owner = a.Message.Sender
	a.Telegram.OwnerChat = a.Message.Chat

	log.Infof("[interact][telegram] authorized owner %+v and chat %+v", a.Message.Sender, a.Message.Chat)
	return nil
}

type Telegram struct {
	Bot *telebot.Bot `json:"-"`

	// Owner is the authorized bot owner
	// This field is exported in order to be stored in file
	Owner *telebot.User `json:"owner"`

	// OwnerChat is the chat of the authorized bot owner
	// This field is exported in order to be stored in file
	OwnerChat *telebot.Chat `json:"chat"`

	// textMessageResponder is used for interact to register its message handler
	textMessageResponder Responder
}

func (tm *Telegram) newAuthorizer(message *telebot.Message) *TelegramAuthorizer {
	return &TelegramAuthorizer{
		Telegram: tm,
		Message:  message,
	}
}

func (tm *Telegram) SetTextMessageResponder(textMessageResponder Responder) {
	tm.textMessageResponder = textMessageResponder
}

func (tm *Telegram) Start(context.Context) {
	tm.Bot.Handle(telebot.OnText, func(m *telebot.Message) {
		log.Infof("[interact][telegram] onText: %+v", m)

		authorizer := tm.newAuthorizer(m)
		reply := tm.newReply()
		if tm.textMessageResponder != nil {
			if err := tm.textMessageResponder(m.Text, reply, authorizer); err != nil {
				log.WithError(err).Errorf("[interact][telegram] response handling error")
			}
		}

		reply.build()
		if _, err := tm.Bot.Send(m.Sender, reply.message, reply.menu); err != nil {
			log.WithError(err).Errorf("[interact][telegram] message send error")
		}
	})
	go tm.Bot.Start()
}

func (tm *Telegram) AddCommand(command string, responder Responder) {
	tm.Bot.Handle(command, func(m *telebot.Message) {
		reply := tm.newReply()
		if err := responder(m.Payload, reply); err != nil {
			log.WithError(err).Errorf("[interact][telegram] responder error")
			tm.Bot.Send(m.Sender, fmt.Sprintf("error: %v", err))
			return
		}

		// build up the response objects
		reply.build()
		if _, err := tm.Bot.Send(m.Sender, reply.message, reply.menu); err != nil {
			log.WithError(err).Errorf("[interact][telegram] message send error")
		}
	})
}

func (tm *Telegram) newReply() *TelegramReply {
	return &TelegramReply{
		bot:  tm.Bot,
		menu: &telebot.ReplyMarkup{ResizeReplyKeyboard: true},
	}
}
