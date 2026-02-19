package v1

import "github.com/sirupsen/logrus"

var logger = logrus.WithFields(
	logrus.Fields{
		"component": "chart",
	},
)
