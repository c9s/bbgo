package okex

import (
	"github.com/sirupsen/logrus"
)

var log = logrus.WithFields(logrus.Fields{
	"exchange": "okex",
})

type Exchange struct {
}
