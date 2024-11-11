package livenote

import (
	"sync"
	"time"
)

type Object interface {
	ObjectID() string
}

type LiveNote struct {
	// MessageID is the unique identifier of the message
	// for slack, it's the timestamp of the message
	MessageID string `json:"messageId"`

	ChannelID string `json:"channelId"`

	Pin bool `json:"pin"`

	TimeToLive time.Duration `json:"timeToLive"`

	Object Object

	cachedObjID string

	postedTime time.Time
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

func (n *LiveNote) SetTimeToLive(du time.Duration) {
	n.TimeToLive = du
}

func (n *LiveNote) SetPostedTime(tt time.Time) {
	n.postedTime = tt
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

func (n *LiveNote) SetPin(enabled bool) {
	n.Pin = enabled
}

func (n *LiveNote) IsExpired(now time.Time) bool {
	if n.postedTime.IsZero() || n.TimeToLive == 0 {
		return false
	}

	expiryTime := n.postedTime.Add(n.TimeToLive)
	return now.After(expiryTime)
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
			if note.IsExpired(time.Now()) {
				return nil
			}

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
			if note.IsExpired(time.Now()) {
				break
			}

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
