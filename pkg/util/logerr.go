package util

import (
	log "github.com/sirupsen/logrus"
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
