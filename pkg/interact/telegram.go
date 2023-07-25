package interact

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/c9s/bbgo/pkg/util"
	log "github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
	"gopkg.in/tucnak/telebot.v2"
)

func init() {
	// force interface type check
	_ = Reply(&TelegramReply{})
}

var sendLimiter = rate.NewLimiter(10, 2)

const maxMessageSize int = 3000

type TelegramSessionMap map[int64]*TelegramSession

type TelegramSession struct {
	BaseSession

	telegram *Telegram

	User *telebot.User `json:"user"`
	Chat *telebot.Chat `json:"chat"`
}

func (s *TelegramSession) ID() string {
	return fmt.Sprintf("telegram-%d-%d", s.User.ID, s.Chat.ID)
}

func (s *TelegramSession) SetAuthorized() {
	s.BaseSession.SetAuthorized()
	s.telegram.EmitAuthorized(s)
}

func NewTelegramSession(telegram *Telegram, message *telebot.Message) *TelegramSession {
	return &TelegramSession{
		BaseSession: BaseSession{
			OriginState:  StatePublic,
			CurrentState: StatePublic,
			Authorized:   false,
			authorizing:  false,

			StartedTime: time.Now(),
		},
		telegram: telegram,
		User:     message.Sender,
		Chat:     message.Chat,
	}
}

type TelegramReply struct {
	bot     *telebot.Bot
	session *TelegramSession

	message string
	menu    *telebot.ReplyMarkup
	buttons []telebot.Btn
	set     bool
}

func (r *TelegramReply) Send(message string) {
	ctx := context.Background()
	splits := util.StringSplitByLength(message, maxMessageSize)
	for _, split := range splits {
		if err := sendLimiter.Wait(ctx); err != nil {
			log.WithError(err).Errorf("telegram send limit exceeded")
			return
		}
		checkSendErr(r.bot.Send(r.session.Chat, split))
	}
}

func (r *TelegramReply) Message(message string) {
	r.message = message
	r.set = true
}

func (r *TelegramReply) RemoveKeyboard() {
	r.menu.ReplyKeyboardRemove = true
	r.set = true
}

func (r *TelegramReply) AddButton(text string, name string, value string) {
	var button = r.menu.Text(text)
	r.buttons = append(r.buttons, button)
	r.set = true
}

func (r *TelegramReply) AddMultipleButtons(buttonsForm [][3]string) {
	for _, buttonForm := range buttonsForm {
		r.AddButton(buttonForm[0], buttonForm[1], buttonForm[2])
	}
}

func (r *TelegramReply) build() {
	var rows []telebot.Row
	for _, button := range r.buttons {
		rows = append(rows, telebot.Row{
			button,
		})
	}
	r.menu.Reply(rows...)
}

//go:generate callbackgen -type Telegram
type Telegram struct {
	Bot *telebot.Bot `json:"-"`

	// Private is used to protect the telegram bot, users not authenticated can not see messages or sending commands
	Private bool `json:"private,omitempty"`

	sessions TelegramSessionMap

	// textMessageResponder is used for interact to register its message handler
	textMessageResponder Responder

	callbackResponder CallbackResponder

	commands []*Command

	authorizedCallbacks []func(s *TelegramSession)
}

func NewTelegram(bot *telebot.Bot) *Telegram {
	return &Telegram{
		Bot:      bot,
		Private:  true,
		sessions: make(map[int64]*TelegramSession),
	}
}

func (tm *Telegram) SetCallbackResponder(responder CallbackResponder) {
	tm.callbackResponder = responder
}

func (tm *Telegram) SetTextMessageResponder(responder Responder) {
	tm.textMessageResponder = responder
}

func (tm *Telegram) Start(ctx context.Context) {
	tm.Bot.Handle(telebot.OnCallback, func(c *telebot.Callback) {
		log.Infof("[telegram] onCallback: %+v", c)
	})

	tm.Bot.Handle(telebot.OnText, func(m *telebot.Message) {
		log.Infof("[telegram] onText: %+v", m)

		session := tm.loadSession(m)
		if tm.Private {
			if !session.authorizing && !session.Authorized {
				log.Warn("[telegram] telegram is set to private mode, skipping message")
				return
			}
		}

		reply := tm.newReply(session)
		if tm.textMessageResponder != nil {
			if err := tm.textMessageResponder(session, m.Text, reply); err != nil {
				log.WithError(err).Errorf("[telegram] response handling error")
			}
		}

		if reply.set {
			reply.build()
			if len(reply.message) > 0 || reply.menu != nil {
				splits := util.StringSplitByLength(reply.message, maxMessageSize)
				for i, split := range splits {
					if err := sendLimiter.Wait(ctx); err != nil {
						log.WithError(err).Errorf("telegram send limit exceeded")
						return
					}
					if i == len(splits)-1 {
						// only set menu on the last message
						checkSendErr(tm.Bot.Send(m.Chat, split, reply.menu))
					} else {
						checkSendErr(tm.Bot.Send(m.Chat, split))
					}
				}
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

	tm.Bot.Start()
}

func checkSendErr(m *telebot.Message, err error) {
	if err != nil {
		log.WithError(err).Errorf("[telegram] message send error")
	}
}

func (tm *Telegram) loadSession(m *telebot.Message) *TelegramSession {
	if tm.sessions == nil {
		tm.sessions = make(map[int64]*TelegramSession)
	}

	session, ok := tm.sessions[m.Chat.ID]
	if ok {
		log.Infof("[telegram] loaded existing session: %+v", session)
		return session
	}

	session = NewTelegramSession(tm, m)
	tm.sessions[m.Chat.ID] = session

	log.Infof("[telegram] allocated a new session: %+v", session)
	return session
}

func (tm *Telegram) AddCommand(cmd *Command, responder Responder) {
	tm.commands = append(tm.commands, cmd)
	tm.Bot.Handle(cmd.Name, func(m *telebot.Message) {
		session := tm.loadSession(m)
		reply := tm.newReply(session)
		if err := responder(session, m.Payload, reply); err != nil {
			log.WithError(err).Errorf("[telegram] responder error")
			checkSendErr(tm.Bot.Send(m.Chat, fmt.Sprintf("error: %v", err)))
			return
		}

		// build up the response objects
		if reply.set {
			reply.build()
			checkSendErr(tm.Bot.Send(m.Chat, reply.message, reply.menu))
		}
	})
}

func (tm *Telegram) newReply(session *TelegramSession) *TelegramReply {
	return &TelegramReply{
		bot:     tm.Bot,
		session: session,
		menu:    &telebot.ReplyMarkup{ResizeReplyKeyboard: true},
	}
}

func (tm *Telegram) Sessions() TelegramSessionMap {
	return tm.sessions
}

func (tm *Telegram) RestoreSessions(sessions TelegramSessionMap) {
	if len(sessions) == 0 {
		return
	}

	log.Infof("[telegram] restoring telegram %d sessions", len(sessions))
	tm.sessions = sessions
	for _, session := range sessions {
		if session.Chat == nil || session.User == nil {
			continue
		}

		// update telegram context reference
		session.telegram = tm

		if session.IsAuthorized() {
			if _, err := tm.Bot.Send(session.Chat, fmt.Sprintf("Hi %s, I'm back. Your telegram session is restored.", session.User.Username)); err != nil {
				log.WithError(err).Error("[telegram] can not send telegram message")
			}
		}
	}
}
