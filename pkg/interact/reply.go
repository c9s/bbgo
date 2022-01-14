package interact

type Reply interface {
	Send(message string)
	Message(message string)
	AddButton(text string)
	RemoveKeyboard()
}
