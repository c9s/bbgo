package maxapi

import (
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/types"
)

var logger = logrus.WithFields(logrus.Fields{
	"exchange": types.ExchangeMax,
})
