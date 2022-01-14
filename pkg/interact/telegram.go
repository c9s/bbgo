package interact

import (
	"context"
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"gopkg.in/tucnak/telebot.v2"
)

type TelegramReply struct {
	bot  *telebot.Bot
	chat *telebot.Chat

	message string
	menu    *telebot.ReplyMarkup
	buttons [][]telebot.Btn
	set     bool
}

func (r *TelegramReply) Send(message string) {
	checkSendErr(r.bot.Send(r.chat, message))
}

func (r *TelegramReply) Message(message string) {
	r.message = message
	r.set = true
}

func (r *TelegramReply) RemoveKeyboard() {
	r.menu.ReplyKeyboardRemove = true
	r.set = true
}

func (r *TelegramReply) AddButton(text string) {
	var button = r.menu.Text(text)
	if len(r.buttons) == 0 {
		r.buttons = append(r.buttons, []telebot.Btn{})
	}
	r.buttons[len(r.buttons)-1] = append(r.buttons[len(r.buttons)-1], button)
	r.set = true
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
	a.Telegram.authorizing = false
	log.Infof("[interact][telegram] authorized owner %+v and chat %+v", a.Message.Sender, a.Message.Chat)
	a.Telegram.EmitAuthorized(a)
	return nil
}

func (a *TelegramAuthorizer) StartAuthorizing() {
	a.Telegram.authorizing = true
}

//go:generate callbackgen -type Telegram
type Telegram struct {
	Bot *telebot.Bot `json:"-"`

	// Private is used to protect the telegram bot, users not authenticated can not see messages or sending commands
	Private bool `json:"private,omitempty"`

	authorizing bool

	// Owner is the authorized bot owner
	// This field is exported in order to be stored in file
	Owner *telebot.User `json:"owner,omitempty"`

	// OwnerChat is the chat of the authorized bot owner
	// This field is exported in order to be stored in file
	OwnerChat *telebot.Chat `json:"chat,omitempty"`

	// textMessageResponder is used for interact to register its message handler
	textMessageResponder Responder

	commands []*Command

	authorizedCallbacks []func(a *TelegramAuthorizer)
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
		log.Infof("[telegram] onText: %+v", m)

		if tm.Private && !tm.authorizing {
			// ignore the message directly if it's not authorized yet
			if tm.Owner == nil {
				log.Warn("[telegram] telegram is set to private mode, skipping message")
				return
			} else if tm.Owner != nil && tm.Owner.ID != m.Sender.ID {
				log.Warnf("[telegram] telegram is set to private mode, owner does not match: %d != %d", tm.Owner.ID, m.Sender.ID)
				return
			}
		}

		authorizer := tm.newAuthorizer(m)
		reply := tm.newReply(m)
		if tm.textMessageResponder != nil {
			if err := tm.textMessageResponder(m.Text, reply, authorizer); err != nil {
				log.WithError(err).Errorf("[telegram] response handling error")
			}
		}

		if reply.set {
			reply.build()
			if _, err := tm.Bot.Send(m.Sender, reply.message, reply.menu); err != nil {
				log.WithError(err).Errorf("[telegram] message send error")
			}
		}
	})

	var cmdList []telebot.Command
	for _, cmd := range tm.commands {
		if len(cmd.Desc) == 0 {
			continue
		}

		cmdList = append(cmdList, telebot.Command{
			Text:        strings.ToLower(strings.TrimLeft(cmd.Name, "/")),
			Description: cmd.Desc,
		})
	}
	if err := tm.Bot.SetCommands(cmdList); err != nil {
		log.WithError(err).Errorf("[telegram] set commands error")
	}

	go tm.Bot.Start()
}

func checkSendErr(m *telebot.Message, err error) {
	if err != nil {
		log.WithError(err).Errorf("[telegram] message send error")
	}
}

func (tm *Telegram) AddCommand(cmd *Command, responder Responder) {
	tm.commands = append(tm.commands, cmd)
	tm.Bot.Handle(cmd.Name, func(m *telebot.Message) {
		authorizer := tm.newAuthorizer(m)
		reply := tm.newReply(m)
		if err := responder(m.Payload, reply, authorizer); err != nil {
			log.WithError(err).Errorf("[telegram] responder error")
			checkSendErr(tm.Bot.Send(m.Sender, fmt.Sprintf("error: %v", err)))
			return
		}

		// build up the response objects
		if reply.set {
			reply.build()
			checkSendErr(tm.Bot.Send(m.Sender, reply.message, reply.menu))
		}
	})
}

func (tm *Telegram) newReply(m *telebot.Message) *TelegramReply {
	return &TelegramReply{
		bot:  tm.Bot,
		chat: m.Chat,
		menu: &telebot.ReplyMarkup{ResizeReplyKeyboard: true},
	}
}

func (tm *Telegram) RestoreSession(session *TelegramSession) {
	log.Infof("[telegram] restoring telegram session: %+v", session)
	if session.OwnerChat != nil {
		tm.OwnerChat = session.OwnerChat
		tm.Owner = session.Owner
		if _, err := tm.Bot.Send(tm.OwnerChat, fmt.Sprintf("Hi %s, I'm back. Your telegram session is restored.", tm.Owner.Username)); err != nil {
			log.WithError(err).Error("[telegram] can not send telegram message")
		}
	}
}

type TelegramSession struct {
	Owner     *telebot.User `json:"owner"`
	OwnerChat *telebot.Chat `json:"chat"`

	// Subscribers stores the Chat objects
	Subscribers map[int64]time.Time `json:"chats"`
}

func NewTelegramSession() *TelegramSession {
	return &TelegramSession{
		Owner:       nil,
		OwnerChat:   nil,
		Subscribers: make(map[int64]time.Time),
	}
}
