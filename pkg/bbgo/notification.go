package bbgo

import (
	"bytes"
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/envvar"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

// notificationFlushTimeout is the time budget given to Flush to wait for
// queued notifications (e.g. Slack/Telegram) to actually be sent before giving up.
// Overridable via the BBGO_FLUSH_TIMEOUT environment variable (e.g. "10s").
var notificationFlushTimeout = 5 * time.Second

func init() {
	if du, ok := envvar.Duration("BBGO_FLUSH_TIMEOUT", notificationFlushTimeout); ok && du > 0 {
		notificationFlushTimeout = du
	}
}

var Notification = &Notifiability{}

func Notify(obj interface{}, args ...interface{}) {
	Notification.Notify(obj, args...)
}

// Flush blocks until all notifiers have sent their queued notifications, up
// to notificationFlushTimeout.
func Flush() {
	Notification.Flush()
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

	// Flush blocks until all notifications queued prior to the call have been
	// sent, or ctx is done, whichever happens first.
	Flush(ctx context.Context)
}

type NullNotifier struct{}

func (n *NullNotifier) Notify(_ interface{}, args ...interface{}) {}

func (n *NullNotifier) Upload(_ *types.UploadFile) {}

func (n *NullNotifier) Flush(_ context.Context) {}

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

// Flush waits for all registered notifiers to send their queued notifications
// concurrently, giving them up to notificationFlushTimeout to finish.
func (m *Notifiability) Flush() {
	ctx, cancel := context.WithTimeout(context.Background(), notificationFlushTimeout)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(len(m.notifiers))

	for _, n := range m.notifiers {
		go func(n Notifier) {
			defer wg.Done()
			n.Flush(ctx)
		}(n)
	}

	wg.Wait()
}
