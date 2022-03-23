package interact

type Button struct {
	Text  string
	Name  string
	Value string
}

type TextField struct {
	// Name is the form field name
	Name string

	// Label is the field label
	Label string

	// PlaceHolder is the sample text in the text input
	PlaceHolder string
}

type Option struct {
	// Name is the form field name
	Name string

	// Label is the option label for display
	Label string

	// Value is the option value
	Value string
}

type Reply interface {
	// Send sends the message directly to the client's session
	Send(message string)

	// Message sets the message to the reply
	Message(message string)

	// AddButton adds the button to the reply
	AddButton(text string, name, value string)

	// AddMultipleButtons adds multiple buttons to the reply
	AddMultipleButtons(buttonsForm [][3]string)

	// Choose(prompt string, options ...Option)
	// Confirm shows the confirm dialog or confirm button in the user interface
	// Confirm(prompt string)
}

// KeyboardController is used when messenger supports keyboard controls
type KeyboardController interface {
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
