package bbgo

import (
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/types"
)

// PostLiveNote posts a live note to slack or other services
// The MessageID will be set after the message is posted if it's not set.
func PostLiveNote(note *types.LiveNote) {
	for _, poster := range Notification.liveNotePosters {
		if err := poster.PostLiveNote(note); err != nil {
			logrus.WithError(err).Errorf("unable to post live note: %+v", note)
		}
	}
}

type LiveNotePoster interface {
	PostLiveNote(note *types.LiveNote) error
}
