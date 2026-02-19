package maxapi

import "github.com/sirupsen/logrus"

var logger = logrus.WithFields(logrus.Fields{
	"exchange": "max",
})
