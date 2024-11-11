package livenote

import "sync"

type Object interface {
	ObjectID() string
}

type LiveNote struct {
	// MessageID is the unique identifier of the message
	// for slack, it's the timestamp of the message
	MessageID string `json:"messageId"`

	ChannelID string `json:"channelId"`

	Object Object

	cachedObjID string
}

func NewLiveNote(object Object) *LiveNote {
	return &LiveNote{
		Object: object,
	}
}

func (n *LiveNote) ObjectID() string {
	if n.cachedObjID != "" {
		return n.cachedObjID
	}

	n.cachedObjID = n.Object.ObjectID()
	return n.cachedObjID
}

func (n *LiveNote) SetObject(object Object) {
	n.Object = object
}

func (n *LiveNote) SetMessageID(messageID string) {
	n.MessageID = messageID
}

func (n *LiveNote) SetChannelID(channelID string) {
	n.ChannelID = channelID
}

type Pool struct {
	notes map[string]*LiveNote
	mu    sync.Mutex
}

func NewPool(size int64) *Pool {
	return &Pool{
		notes: make(map[string]*LiveNote, size),
	}
}

func (p *Pool) Get(obj Object) *LiveNote {
	objID := obj.ObjectID()

	p.mu.Lock()
	defer p.mu.Unlock()

	for _, note := range p.notes {
		if note.ObjectID() == objID {
			return note
		}
	}

	return nil
}

func (p *Pool) Update(obj Object) *LiveNote {
	objID := obj.ObjectID()

	p.mu.Lock()
	defer p.mu.Unlock()

	for _, note := range p.notes {
		if note.ObjectID() == objID {
			// update the object inside the note
			note.SetObject(obj)
			return note
		}
	}

	note := NewLiveNote(obj)
	p.add(objID, note)
	return note
}

func (p *Pool) add(id string, note *LiveNote) {
	p.notes[id] = note
}

func (p *Pool) Add(note *LiveNote) {
	p.mu.Lock()
	p.add(note.ObjectID(), note)
	p.mu.Unlock()
}
