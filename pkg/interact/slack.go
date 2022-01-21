package interact

import (
	"context"
	"encoding/json"
	"fmt"
	stdlog "log"
	"os"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
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

	buttons []Button

	textInputModalViewRequest *slack.ModalViewRequest
}

func (reply *SlackReply) Send(message string) {
	cID, tsID, err := reply.client.PostMessage(
		reply.session.ChannelID,
		slack.MsgOptionText(message, false),
		slack.MsgOptionAsUser(false), // Add this if you want that the bot would post message as a user, otherwise it will send response using the default slackbot
	)
	if err != nil {
		log.WithError(err).Errorf("slack post message error: channel=%s thread=%s", cID, tsID)
		return
	}
}

func (reply *SlackReply) Choose(prompt string, options ...Option) {
}

func (reply *SlackReply) Message(message string) {
	reply.message = message
}

func (reply *SlackReply) InputText(prompt string, textFields ...TextField) {
	reply.message = prompt
	reply.textInputModalViewRequest = generateTextInputModalRequest(prompt, prompt, textFields...)
}

// RemoveKeyboard is not supported by Slack
func (reply *SlackReply) RemoveKeyboard() {}

func (reply *SlackReply) AddButton(text string, name string, value string) {
	reply.buttons = append(reply.buttons, Button{
		Text:  text,
		Name:  name,
		Value: value,
	})
}

func (reply *SlackReply) build() interface{} {
	if reply.textInputModalViewRequest != nil {
		return reply.textInputModalViewRequest
	}

	var blocks slack.Blocks

	if len(reply.message) > 0 {
		blocks.BlockSet = append(blocks.BlockSet, slack.NewSectionBlock(
			&slack.TextBlockObject{
				Type: slack.MarkdownType,
				Text: reply.message,
			},
			nil, // fields
			nil, // accessory
			// slack.SectionBlockOptionBlockID(reply.uuid),
		))

		return blocks
	}

	blocks.BlockSet = append(blocks.BlockSet, slack.NewSectionBlock(
		&slack.TextBlockObject{
			Type: slack.MarkdownType,
			Text: reply.message,
		},
		nil, // fields
		nil, // accessory
		slack.SectionBlockOptionBlockID(reply.uuid),
	))

	if len(reply.buttons) > 0 {
		var buttons []slack.BlockElement
		for _, btn := range reply.buttons {
			actionID := reply.uuid + ":" + btn.Value
			buttons = append(buttons,
				slack.NewButtonBlockElement(
					// action id should be unique
					actionID,
					btn.Value,
					&slack.TextBlockObject{
						Type: slack.PlainTextType,
						Text: btn.Text,
					},
				),
			)
		}
		blocks.BlockSet = append(blocks.BlockSet, slack.NewActionBlock(reply.uuid, buttons...))
	}

	return blocks
}

type SlackSession struct {
	BaseSession

	ChannelID string
	UserID    string

	// questions is used to store the questions that we added in the reply
	// the key is the client generated callback id
	questions map[string]interface{}
}

func NewSlackSession(userID, channelID string) *SlackSession {
	return &SlackSession{
		UserID:    userID,
		ChannelID: channelID,
		questions: make(map[string]interface{}),
	}
}

func (s *SlackSession) ID() string {
	return s.UserID
	// return fmt.Sprintf("%s-%s", s.UserID, s.ChannelID)
}

type SlackSessionMap map[string]*SlackSession

//go:generate callbackgen -type Slack
type Slack struct {
	client *slack.Client
	socket *socketmode.Client

	sessions SlackSessionMap

	commands          map[string]*Command
	commandResponders map[string]Responder

	// textMessageResponder is used for interact to register its message handler
	textMessageResponder Responder

	authorizedCallbacks []func(userSession *SlackSession)

	eventsApiCallbacks []func(evt slackevents.EventsAPIEvent)
}

func NewSlack(client *slack.Client) *Slack {
	socket := socketmode.New(
		client,
		socketmode.OptionDebug(true),
		socketmode.OptionLog(
			stdlog.New(os.Stdout, "socketmode: ",
				stdlog.Lshortfile|stdlog.LstdFlags)),
	)

	return &Slack{
		client:            client,
		socket:            socket,
		sessions:          make(SlackSessionMap),
		commands:          make(map[string]*Command),
		commandResponders: make(map[string]Responder),
	}
}

func (s *Slack) SetTextMessageResponder(responder Responder) {
	s.textMessageResponder = responder
}

func (s *Slack) AddCommand(command *Command, responder Responder) {
	if _, exists := s.commands[command.Name]; exists {
		panic(fmt.Errorf("command %s already exists, can not be re-defined", command.Name))
	}

	s.commands[command.Name] = command
	s.commandResponders[command.Name] = responder
}

func (s *Slack) listen() {
	for evt := range s.socket.Events {
		log.Debugf("event: %+v", evt)

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
				log.Debugf("ignored %+v", evt)
				continue
			}

			log.Debugf("event received: %+v", eventsAPIEvent)
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
				log.Debugf("ignored %+v", evt)
				continue
			}

			log.Debugf("interaction received: %+v", callback)

			var payload interface{}

			switch callback.Type {
			case slack.InteractionTypeBlockActions:
				// See https://api.slack.com/apis/connections/socket-implement#button
				log.Debugf("InteractionTypeBlockActions: %+v", callback)

			case slack.InteractionTypeShortcut:
				log.Debugf("InteractionTypeShortcut: %+v", callback)

			case slack.InteractionTypeViewSubmission:
				// See https://api.slack.com/apis/connections/socket-implement#modal
				log.Debugf("[slack] InteractionTypeViewSubmission: %+v", callback)
				var values = simplifyStateValues(callback.View.State)

				if len(values) > 1 {
					log.Warnf("[slack] more than 1 values received from the modal view submission, the value choosen from the state values might be incorrect")
				}

				log.Debugln(toJson(values))
				if inputValue, ok := takeOneValue(values); ok {
					session := s.loadSession(evt, callback.User.ID, callback.Channel.ID)

					if !session.authorizing && !session.Authorized {
						log.Warn("[slack] telegram is set to private mode, skipping message")
						return
					}

					reply := s.newReply(session)
					if s.textMessageResponder != nil {
						if err := s.textMessageResponder(session, inputValue, reply); err != nil {
							log.WithError(err).Errorf("[slack] response handling error")
						}
					}

					// close the modal view by sending a null payload
					s.socket.Ack(*evt.Request)

					// build the response
					response := reply.build()

					log.Debugln("response payload", toJson(response))
					switch response := response.(type) {
					case slack.Blocks:
						payload = map[string]interface{}{
							"response_action": "clear",
							// "errors": { "ticket-due-date": "You may not select a due date in the past" },
							"blocks":          response.BlockSet,
						}
					}
				}

			case slack.InteractionTypeDialogSubmission:
				log.Debugf("[slack] InteractionTypeDialogSubmission: %+v", callback)

			default:
				log.Debugf("[slack] unexpected callback type: %+v", callback)

			}

			s.socket.Ack(*evt.Request, payload)

		case socketmode.EventTypeHello:
			log.Debugf("[slack] hello command received: %+v", evt)

		case socketmode.EventTypeSlashCommand:
			slashCmd, ok := evt.Data.(slack.SlashCommand)
			if !ok {
				log.Debugf("[slack] ignored %+v", evt)
				continue
			}

			log.Debugf("[slack] slash command received: %+v", slashCmd)
			responder, exists := s.commandResponders[slashCmd.Command]
			if !exists {
				log.Errorf("[slack] command %s does not exist", slashCmd.Command)
				s.socket.Ack(*evt.Request)
				continue
			}

			session := s.loadSession(evt, slashCmd.UserID, slashCmd.ChannelID)
			reply := s.newReply(session)
			if err := responder(session, slashCmd.Text, reply); err != nil {
				log.WithError(err).Errorf("[slack] responder returns error")
				s.socket.Ack(*evt.Request)
				continue
			}

			payload := reply.build()
			if payload == nil {
				log.Warnf("[slack] reply returns nil payload")
				// ack with empty payload
				s.socket.Ack(*evt.Request)
				continue
			}

			switch o := payload.(type) {
			case *slack.ModalViewRequest:
				if resp, err := s.socket.OpenView(slashCmd.TriggerID, *o); err != nil {
					log.WithError(err).Error("[slack] view open error, resp: %+v", resp)
				}
				s.socket.Ack(*evt.Request)
			default:
				s.socket.Ack(*evt.Request, o)
			}

		default:
			log.Debugf("[slack] unexpected event type received: %s", evt.Type)
		}
	}
}

func (s *Slack) loadSession(evt socketmode.Event, userID, channelID string) *SlackSession {
	if session, ok := s.sessions[userID]; ok {
		return session
	}

	session := NewSlackSession(userID, channelID)
	s.sessions[userID] = session
	return session
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
		log.WithError(err).Errorf("slack socketmode error")
	}
}

// generateTextInputModalRequest generates a general slack modal view request with the given text fields
// see also https://api.slack.com/surfaces/modals/using#opening
func generateTextInputModalRequest(title string, prompt string, textFields ...TextField) *slack.ModalViewRequest {
	// create a ModalViewRequest with a header and two inputs
	titleText := slack.NewTextBlockObject("plain_text", title, false, false)
	closeText := slack.NewTextBlockObject("plain_text", "Close", false, false)
	submitText := slack.NewTextBlockObject("plain_text", "Submit", false, false)

	headerText := slack.NewTextBlockObject("mrkdwn", prompt, false, false)
	headerSection := slack.NewSectionBlock(headerText, nil, nil)

	blocks := slack.Blocks{
		BlockSet: []slack.Block{
			headerSection,
		},
	}

	for _, textField := range textFields {
		labelObject := slack.NewTextBlockObject("plain_text", textField.Label, false, false)
		placeHolderObject := slack.NewTextBlockObject("plain_text", textField.PlaceHolder, false, false)
		textInputObject := slack.NewPlainTextInputBlockElement(placeHolderObject, textField.Name)

		// Notice that blockID is a unique identifier for a block
		inputBlock := slack.NewInputBlock("block-"+textField.Name+"-"+uuid.NewString(), labelObject, textInputObject)
		blocks.BlockSet = append(blocks.BlockSet, inputBlock)
	}

	var modalRequest slack.ModalViewRequest
	modalRequest.Type = slack.ViewType("modal")
	modalRequest.Title = titleText
	modalRequest.Close = closeText
	modalRequest.Submit = submitText
	modalRequest.Blocks = blocks
	return &modalRequest
}

// simplifyStateValues simplifies the multi-layer structured values into just name=value mapping
func simplifyStateValues(state *slack.ViewState) map[string]string {
	var values = make(map[string]string)

	if state == nil {
		return values
	}

	for blockID, fields := range state.Values {
		_ = blockID
		for fieldName, fieldValues := range fields {
			values[fieldName] = fieldValues.Value
		}
	}
	return values
}

func takeOneValue(values map[string]string) (string, bool) {
	for _, v := range values {
		return v, true
	}
	return "", false
}

func toJson(v interface{}) string {
	o, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		log.WithError(err).Errorf("json marshal error")
		return ""
	}
	return string(o)
}
