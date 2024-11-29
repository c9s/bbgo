package bitget

import (
	"github.com/c9s/bbgo/pkg/envvar"
)

type LogFunction func(msg string, args ...interface{})

var debugf LogFunction

func getDebugFunction() LogFunction {
	if v, ok := envvar.Bool("DEBUG_BITGET"); ok && v {
		return log.Infof
	}

	return func(msg string, args ...interface{}) {}
}

func init() {
	debugf = getDebugFunction()
}
