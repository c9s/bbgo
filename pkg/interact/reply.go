package interact

type Button struct {
	Text string
	Name string
	Value string
}

type Reply interface {
	// Send sends the message directly to the client's session
	Send(message string)

	// Message sets the message to the reply
	Message(message string)

	// AddButton adds the button to the reply
	AddButton(text string, name, value string)

	RequireTextInput(title, message string, textFields ...TextField)

	// RemoveKeyboard hides the keyboard from the client user interface
	RemoveKeyboard()
}

// ButtonReply can be used if your reply needs button user interface.
type ButtonReply interface {
	// AddButton adds the button to the reply
	AddButton(text string)
}

// DialogReply can be used if your reply needs Dialog user interface
type DialogReply interface {
	// AddButton adds the button to the reply
	Dialog(title, text string, buttons []string)
}
