package types

type LiveNoteObject interface {
	ObjectID() string
}

type LiveNote struct {
	// MessageID is the unique identifier of the message
	// for slack, it's the timestamp of the message
	MessageID string `json:"messageId"`

	ChannelID string `json:"channelId"`

	Object LiveNoteObject
}

func NewLiveNote(object LiveNoteObject) *LiveNote {
	return &LiveNote{
		Object: object,
	}
}

func (n *LiveNote) ObjectID() string {
	return n.Object.ObjectID()
}

func (n *LiveNote) SetMessageID(messageID string) {
	n.MessageID = messageID
}

func (n *LiveNote) SetChannelID(channelID string) {
	n.ChannelID = channelID
}
