package util

import (
	"time"

	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

// LogErr logs the error with the message and arguments if the error is not nil.
// It returns true if the error is not nil.
// Examples:
// LogErr(err)
// LogErr(err, "error message")
// LogErr(err, "error message %s", "with argument")
func LogErr(err error, msgAndArgs ...interface{}) bool {
	if err == nil {
		return false
	}

	if len(msgAndArgs) == 0 {
		log.WithError(err).Error(err.Error())
	} else if len(msgAndArgs) == 1 {
		msg := msgAndArgs[0].(string)
		log.WithError(err).Error(msg)
	} else if len(msgAndArgs) > 1 {
		msg := msgAndArgs[0].(string)
		log.WithError(err).Errorf(msg, msgAndArgs[1:]...)
	}

	return true
}

type WarnFirstLogger struct {
	logger      logrus.FieldLogger
	warnLimiter *rate.Limiter
}

func NewWarnFirstLogger(threshold int, window time.Duration, logger logrus.FieldLogger) *WarnFirstLogger {
	return &WarnFirstLogger{
		logger:      logger,
		warnLimiter: rate.NewLimiter(rate.Every(window), threshold),
	}
}

func (w *WarnFirstLogger) WarnOrError(err error, msg string, args ...interface{}) {
	log := w.logger
	if err != nil {
		log = log.WithError(err)
	}

	if w.warnLimiter.Allow() {
		log.Warnf(msg, args...)
	} else {
		log.Errorf(msg, args...)
	}
}
