package signal

import "github.com/sirupsen/logrus"

type Logger struct {
	logger logrus.FieldLogger
}

func (l *Logger) SetLogger(logger logrus.FieldLogger) {
	l.logger = logger
}
