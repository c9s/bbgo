package telegramnotifier

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

var limiter = rate.NewLimiter(rate.Every(time.Minute), 3)

type LogHook struct {
	notifier *Notifier
}

func NewLogHook(notifier *Notifier) *LogHook {
	return &LogHook{
		notifier: notifier,
	}
}

func (t *LogHook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.ErrorLevel,
		logrus.FatalLevel,
		logrus.PanicLevel,
	}
}

func (t *LogHook) Fire(e *logrus.Entry) error {
	if !limiter.Allow() {
		return nil
	}

	var message = fmt.Sprintf("[%s] %s", e.Level.String(), e.Message)
	if errData, ok := e.Data[logrus.ErrorKey]; ok && errData != nil {
		if err, isErr := errData.(error); isErr {
			message += " Error: " + err.Error()
		}
	}

	t.notifier.Notify(message)
	return nil
}
