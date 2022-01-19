package interact

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)


type SlackReply struct {
	// uuid is the unique id of this question
	// can be used as the callback id
	uuid string

	session *SlackSession

	client *slack.Client

	message string

	accessories []*slack.Accessory
}

func (reply *SlackReply) Send(message string) {
	cID, tsID, err := reply.client.PostMessage(
		reply.session.ChannelID,
		slack.MsgOptionText(message, false),
		slack.MsgOptionAsUser(false), // Add this if you want that the bot would post message as a user, otherwise it will send response using the default slackbot
	)
	if err != nil {
		logrus.WithError(err).Errorf("slack post message error: channel=%s thread=%s", cID, tsID)
		return
	}
}

func (reply *SlackReply) Message(message string) {
	reply.message = message
}

// RemoveKeyboard is not supported by Slack
func (reply *SlackReply) RemoveKeyboard() {}

func (reply *SlackReply) AddButton(text string, name string, value string) {
	actionID := reply.uuid + ":" + value
	reply.accessories = append(reply.accessories, slack.NewAccessory(
		slack.NewButtonBlockElement(
			// action id should be unique
			actionID,
			value,
			&slack.TextBlockObject{
				Type: slack.PlainTextType,
				Text: text,
			},
		),
	))
}

func (reply *SlackReply) build() map[string]interface{} {
	var blocks []slack.Block

	blocks = append(blocks, slack.NewSectionBlock(
		&slack.TextBlockObject{
			Type: slack.MarkdownType,
			Text: reply.message,
		},
		nil, // fields
		nil, // accessory
		nil, // options
	))

	blocks = append(blocks, slack.NewActionBlock("",
		slack.NewButtonBlockElement(
			"actionID",
			"value",
			slack.NewTextBlockObject(
				slack.PlainTextType, "text", true, false),
		)))

	var payload = map[string]interface{}{
		"blocks": blocks,
	}
	return payload
}

type SlackSession struct {
	BaseSession

	ChannelID string
	UserID    string

	// questions is used to store the questions that we added in the reply
	// the key is the client generated callback id
	questions map[string]interface{}
}

func NewSlackSession() *SlackSession {
	return &SlackSession{
		questions: make(map[string]interface{}),
	}
}

func (s *SlackSession) ID() string {
	return fmt.Sprintf("%s-%s", s.UserID, s.ChannelID)
}

type SlackSessionMap map[int64]*SlackSession

//go:generate callbackgen -type Slack
type Slack struct {
	client *slack.Client
	socket *socketmode.Client

	sessions SlackSessionMap

	commands []*Command

	// textMessageResponder is used for interact to register its message handler
	textMessageResponder Responder

	authorizedCallbacks []func(userSession *SlackSession)

	eventsApiCallbacks []func(slackevents.EventsAPIEvent)
}

func NewSlack(client *slack.Client) *Slack {
	socket := socketmode.New(
		client,
		socketmode.OptionDebug(true),
		socketmode.OptionLog(
			log.New(os.Stdout, "socketmode: ",
				log.Lshortfile|log.LstdFlags)),
	)

	return &Slack{
		client: client,
		socket: socket,
	}
}

func (s *Slack) SetTextMessageResponder(responder Responder) {
	s.textMessageResponder = responder
}

func (s *Slack) AddCommand(command *Command, responder Responder) {
	s.commands = append(s.commands, command)
}

func (s *Slack) listen() {
	for evt := range s.socket.Events {
		switch evt.Type {
		case socketmode.EventTypeConnecting:
			fmt.Println("Connecting to Slack with Socket Mode...")
		case socketmode.EventTypeConnectionError:
			fmt.Println("Connection failed. Retrying later...")
		case socketmode.EventTypeConnected:
			fmt.Println("Connected to Slack with Socket Mode.")
		case socketmode.EventTypeEventsAPI:
			eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
			if !ok {
				logrus.Debugf("ignored %+v", evt)
				continue
			}

			logrus.Debugf("event received: %+v", eventsAPIEvent)
			s.socket.Ack(*evt.Request)

			s.EmitEventsApi(eventsAPIEvent)

			switch eventsAPIEvent.Type {
			case slackevents.CallbackEvent:
				innerEvent := eventsAPIEvent.InnerEvent
				switch ev := innerEvent.Data.(type) {
				case *slackevents.AppMentionEvent:
					_, _, err := s.client.PostMessage(ev.Channel, slack.MsgOptionText("Yes, hello.", false))
					if err != nil {
						fmt.Printf("failed posting message: %v", err)
					}
				case *slackevents.MemberJoinedChannelEvent:
					fmt.Printf("user %q joined to channel %q", ev.User, ev.Channel)
				}
			default:
				s.socket.Debugf("unsupported Events API event received")
			}
		case socketmode.EventTypeInteractive:
			callback, ok := evt.Data.(slack.InteractionCallback)
			if !ok {
				logrus.Debugf("ignored %+v", evt)
				continue
			}

			logrus.Debugf("interaction received: %+v", callback)

			var payload interface{}

			switch callback.Type {
			case slack.InteractionTypeBlockActions:
				// See https://api.slack.com/apis/connections/socket-implement#button
				logrus.Debugf("button clicked!")

			case slack.InteractionTypeShortcut:
			case slack.InteractionTypeViewSubmission:
				// See https://api.slack.com/apis/connections/socket-implement#modal
			case slack.InteractionTypeDialogSubmission:
			default:

			}

			s.socket.Ack(*evt.Request, payload)

		case socketmode.EventTypeSlashCommand:
			cmd, ok := evt.Data.(slack.SlashCommand)
			if !ok {
				logrus.Debugf("ignored %+v", evt)
				continue
			}

			logrus.Debugf("slash command received: %+v", cmd)

			session := s.newSession(evt)
			reply := s.newReply(session)
			if err := s.textMessageResponder(session, "", reply); err != nil {
				continue
			}

			payload := reply.build()
			s.socket.Ack(*evt.Request, payload)
		default:
			logrus.Debugf("unexpected event type received: %s", evt.Type)
		}
	}
}

func (s *Slack) newSession(evt socketmode.Event) *SlackSession {
	return NewSlackSession()
}

func (s *Slack) newReply(session *SlackSession) *SlackReply {
	return &SlackReply{
		uuid:    uuid.New().String(),
		session: session,
	}
}

func (s *Slack) Start(ctx context.Context) {
	go s.listen()
	if err := s.socket.Run(); err != nil {
		logrus.WithError(err).Errorf("slack socketmode error")
	}
}
