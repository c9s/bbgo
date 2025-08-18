package bbgo

import (
	"bytes"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

var Notification = &Notifiability{}

func Notify(obj interface{}, args ...interface{}) {
	Notification.Notify(obj, args...)
}

func SendPhoto(buffer *bytes.Buffer) {
	Notification.Upload(&types.UploadFile{
		Caption:  "Image",
		FileType: types.FileTypeImage,
		Data:     buffer,
	})
}

func Upload(file *types.UploadFile) {
	Notification.Upload(file)
}

type Notifier interface {
	Notify(obj any, args ...any)
	Upload(file *types.UploadFile)
}

type NullNotifier struct{}

func (n *NullNotifier) Notify(_ interface{}, args ...interface{}) {}

func (n *NullNotifier) Upload(_ *types.UploadFile) {}

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

func (m *Notifiability) Upload(file *types.UploadFile) {
	for _, n := range m.notifiers {
		n.Upload(file)
	}
}
