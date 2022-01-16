package interact

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

type SlackSession struct {
	BaseSession
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
				fmt.Printf("Ignored %+v\n", evt)

				continue
			}

			fmt.Printf("Event received: %+v\n", eventsAPIEvent)

			s.socket.Ack(*evt.Request)

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
				fmt.Printf("Ignored %+v\n", evt)

				continue
			}

			fmt.Printf("Interaction received: %+v\n", callback)

			var payload interface{}

			switch callback.Type {
			case slack.InteractionTypeBlockActions:
				// See https://api.slack.com/apis/connections/socket-implement#button

				s.socket.Debugf("button clicked!")
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
				fmt.Printf("Ignored %+v\n", evt)

				continue
			}

			s.socket.Debugf("Slash command received: %+v", cmd)

			payload := map[string]interface{}{
				"blocks": []slack.Block{
					slack.NewSectionBlock(
						&slack.TextBlockObject{
							Type: slack.MarkdownType,
							Text: "foo",
						},
						nil,
						slack.NewAccessory(
							slack.NewButtonBlockElement(
								"",
								"somevalue",
								&slack.TextBlockObject{
									Type: slack.PlainTextType,
									Text: "bar",
								},
							),
						),
					),
				}}

			s.socket.Ack(*evt.Request, payload)
		default:
			fmt.Fprintf(os.Stderr, "Unexpected event type received: %s\n", evt.Type)
		}
	}
}

func (s *Slack) Start(ctx context.Context) {
	go s.listen()
	if err := s.socket.Run() ; err != nil {
		logrus.WithError(err).Errorf("slack socketmode error")
	}
}
