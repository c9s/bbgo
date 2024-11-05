package bbgo

import (
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/livenote"
)

// PostLiveNote posts a live note to slack or other services
// The MessageID will be set after the message is posted if it's not set.
func PostLiveNote(obj livenote.Object) {
	if len(Notification.liveNotePosters) == 0 {
		logrus.Warn("no live note poster is registered")
		return
	}

	for _, poster := range Notification.liveNotePosters {
		if err := poster.PostLiveNote(obj); err != nil {
			logrus.WithError(err).Errorf("unable to post live note: %+v", obj)
		}
	}
}

type LiveNotePoster interface {
	PostLiveNote(note livenote.Object) error
}
