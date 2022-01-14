package interact

type Reply interface {
	Message(message string)
	AddButton(text string)
	RemoveKeyboard()
}
