package bbgo

import (
	"bytes"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/util"
)

var Notification = &Notifiability{}

func Notify(obj interface{}, args ...interface{}) {
	Notification.Notify(obj, args...)
}

func SendPhoto(buffer *bytes.Buffer) {
	Notification.Upload(buffer)
}

type UploadFile struct {
	FileType string
	Data     bytes.Buffer
}

func Upload(file *UploadFile) {

}

type Notifier interface {
	Notify(obj any, args ...any)
	Upload(buffer *bytes.Buffer)
}

type NullNotifier struct{}

func (n *NullNotifier) NotifyTo(channel string, obj interface{}, args ...interface{}) {}

func (n *NullNotifier) Notify(obj interface{}, args ...interface{}) {}

func (n *NullNotifier) Upload(buffer *bytes.Buffer) {}

type Notifiability struct {
	notifiers       []Notifier
	liveNotePosters []LiveNotePoster
}

// AddNotifier adds the notifier that implements the Notifier interface.
func (m *Notifiability) AddNotifier(notifier Notifier) {
	m.notifiers = append(m.notifiers, notifier)

	if poster, ok := notifier.(LiveNotePoster); ok {
		m.liveNotePosters = append(m.liveNotePosters, poster)
	}
}

func (m *Notifiability) Notify(obj interface{}, args ...interface{}) {
	if str, ok := obj.(string); ok {
		simpleArgs := util.FilterSimpleArgs(args)
		logrus.Infof(str, simpleArgs...)
	}

	for _, n := range m.notifiers {
		n.Notify(obj, args...)
	}
}

func (m *Notifiability) Upload(buffer *bytes.Buffer) {
	for _, n := range m.notifiers {
		n.Upload(buffer)
	}
}
