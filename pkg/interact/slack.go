package interact

import (
	"context"
	"encoding/json"
	"fmt"
	stdlog "log"
	"os"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"

	"github.com/c9s/bbgo/pkg/util"
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

func (reply *SlackReply) InputText(prompt string, textFields ...TextField) {
	reply.message = prompt
	reply.textInputModalViewRequest = generateTextInputModalRequest(prompt, prompt, textFields...)
}

func (reply *SlackReply) Choose(prompt string, options ...Option) {
}

func (reply *SlackReply) Message(message string) {
	reply.message = message
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

func (reply *SlackReply) AddMultipleButtons(buttonsForm [][3]string) {
	for _, buttonForm := range buttonsForm {
		reply.AddButton(buttonForm[0], buttonForm[1], buttonForm[2])
	}
}

func (reply *SlackReply) build() interface{} {
	// you should avoid using this modal view request, because it interrupts the interaction flow
	// once we send the modal view request, we can't go back to the channel.
	// (we don't know which channel the user started the interaction)
	if reply.textInputModalViewRequest != nil {
		return reply.textInputModalViewRequest
	}

	if len(reply.message) > 0 {
		return reply.message
	}

	var blocks slack.Blocks
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

	slack     *Slack
	ChannelID string
	UserID    string
}

func NewSlackSession(slack *Slack, userID, channelID string) *SlackSession {
	return &SlackSession{
		BaseSession: BaseSession{
			OriginState:  StatePublic,
			CurrentState: StatePublic,
			Authorized:   false,
			authorizing:  false,

			StartedTime: time.Now(),
		},
		slack:     slack,
		UserID:    userID,
		ChannelID: channelID,
	}
}

func (s *SlackSession) ID() string {
	return fmt.Sprintf("%s-%s", s.UserID, s.ChannelID)
}

func (s *SlackSession) SetAuthorized() {
	s.BaseSession.SetAuthorized()
	s.slack.EmitAuthorized(s)
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
	var opts = []socketmode.Option{
		socketmode.OptionLog(
			stdlog.New(os.Stdout, "socketmode: ",
				stdlog.Lshortfile|stdlog.LstdFlags)),
	}

	if b, ok := util.GetEnvVarBool("DEBUG_SLACK"); ok {
		opts = append(opts, socketmode.OptionDebug(b))
	}

	socket := socketmode.New(client, opts...)
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

func (s *Slack) listen(ctx context.Context) {
	for evt := range s.socket.Events {
		log.Debugf("event: %+v", evt)

		switch evt.Type {
		case socketmode.EventTypeConnecting:
			log.Infof("connecting to slack with socket mode...")

		case socketmode.EventTypeConnectionError:
			log.Infof("connection failed. retrying later...")

		case socketmode.EventTypeConnected:
			log.Infof("connected to slack with socket mode.")

		case socketmode.EventTypeDisconnect:
			log.Infof("slack socket mode disconnected")

		case socketmode.EventTypeEventsAPI:
			eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
			if !ok {
				log.Debugf("ignored %+v", evt)
				continue
			}

			log.Debugf("event received: %+v", eventsAPIEvent)

			// events api don't have response trigger, we can't set the response
			s.socket.Ack(*evt.Request)

			s.EmitEventsApi(eventsAPIEvent)

			switch eventsAPIEvent.Type {
			case slackevents.CallbackEvent:
				innerEvent := eventsAPIEvent.InnerEvent
				switch ev := innerEvent.Data.(type) {
				case *slackevents.MessageEvent:
					log.Infof("message event: text=%+v", ev.Text)

					if len(ev.BotID) > 0 {
						log.Debug("skip bot message")
						continue
					}

					session := s.loadSession(evt, ev.User, ev.Channel)

					if !session.authorizing && !session.Authorized {
						log.Warn("[slack] session is not authorizing nor authorized, skipping message handler")
						continue
					}

					if s.textMessageResponder != nil {
						reply := s.newReply(session)
						if err := s.textMessageResponder(session, ev.Text, reply); err != nil {
							log.WithError(err).Errorf("[slack] response handling error")
							continue
						}

						// build the response
						response := reply.build()

						log.Debugln("response payload", toJson(response))
						switch response := response.(type) {

						case string:
							_, _, err := s.client.PostMessage(ev.Channel, slack.MsgOptionText(response, false))
							if err != nil {
								log.WithError(err).Error("failed posting plain text message")
							}
						case slack.Blocks:
							_, _, err := s.client.PostMessage(ev.Channel, slack.MsgOptionBlocks(response.BlockSet...))
							if err != nil {
								log.WithError(err).Error("failed posting blocks message")
							}

						default:
							log.Errorf("[slack] unexpected message type %T: %+v", response, response)

						}
					}

				case *slackevents.AppMentionEvent:
					log.Infof("app mention event: %+v", ev)
					s.socket.Ack(*evt.Request)

				case *slackevents.MemberJoinedChannelEvent:
					log.Infof("user %q joined to channel %q", ev.User, ev.Channel)
					s.socket.Ack(*evt.Request)
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
						continue
					}

					reply := s.newReply(session)
					if s.textMessageResponder != nil {
						if err := s.textMessageResponder(session, inputValue, reply); err != nil {
							log.WithError(err).Errorf("[slack] response handling error")
							continue
						}
					}

					// close the modal view by sending a null payload
					s.socket.Ack(*evt.Request)

					// build the response
					response := reply.build()

					log.Debugln("response payload", toJson(response))
					switch response := response.(type) {

					case string:
						payload = map[string]interface{}{
							"blocks": []slack.Block{
								translateMessageToBlock(response),
							},
						}

					case slack.Blocks:
						payload = map[string]interface{}{
							"blocks": response.BlockSet,
						}
					default:
						s.socket.Ack(*evt.Request, response)
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

			case string:
				s.socket.Ack(*evt.Request, map[string]interface{}{
					"blocks": []slack.Block{
						translateMessageToBlock(o),
					},
				})

			case *slack.ModalViewRequest:
				if resp, err := s.socket.OpenView(slashCmd.TriggerID, *o); err != nil {
					log.WithError(err).Errorf("[slack] view open error, resp: %+v", resp)
				}
				s.socket.Ack(*evt.Request)

			case slack.Blocks:
				s.socket.Ack(*evt.Request, map[string]interface{}{
					"blocks": o.BlockSet,
				})
			default:
				s.socket.Ack(*evt.Request, o)
			}

		default:
			log.Debugf("[slack] unexpected event type received: %s", evt.Type)
		}
	}
}

func (s *Slack) loadSession(evt socketmode.Event, userID, channelID string) *SlackSession {
	key := userID + "-" + channelID
	if session, ok := s.sessions[key]; ok {
		log.Infof("[slack] an existing session %q found, session: %+v", key, session)
		return session
	}

	session := NewSlackSession(s, userID, channelID)
	s.sessions[key] = session
	log.Infof("[slack] allocated a new session %q, session: %+v", key, session)
	return session
}

func (s *Slack) newReply(session *SlackSession) *SlackReply {
	return &SlackReply{
		uuid:    uuid.New().String(),
		session: session,
	}
}

func (s *Slack) Start(ctx context.Context) {
	go s.listen(ctx)
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

func translateMessageToBlock(message string) slack.Block {
	return slack.NewSectionBlock(
		&slack.TextBlockObject{
			Type: slack.MarkdownType,
			Text: message,
		},
		nil, // fields
		nil, // accessory
		// slack.SectionBlockOptionBlockID(reply.uuid),
	)
}
