package interact

// Responder defines the logic of responding the message
type Responder func(message string, reply Reply, ctxObjects ...interface{}) error

type TextMessageResponder interface {
	SetTextMessageResponder(responder Responder)
}

type CommandResponder interface {
	AddCommand(command *Command, responder Responder)
}
