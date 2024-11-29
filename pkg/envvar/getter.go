package envvar

import (
	"os"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
)

func Duration(n string, args ...time.Duration) (time.Duration, bool) {
	defaultValue := time.Duration(0)
	if len(args) > 0 {
		defaultValue = args[0]
	}

	str, ok := os.LookupEnv(n)
	if !ok {
		return defaultValue, false
	}

	du, err := time.ParseDuration(str)
	if err != nil {
		logrus.WithError(err).Errorf("can not parse env var %q as time.Duration, incorrect format", str)
		return defaultValue, false
	}

	return du, true
}

func Int(n string, args ...int) (int, bool) {
	defaultValue := 0
	if len(args) > 0 {
		defaultValue = args[0]
	}

	str, ok := os.LookupEnv(n)
	if !ok {
		return defaultValue, false
	}

	num, err := strconv.Atoi(str)
	if err != nil {
		logrus.WithError(err).Errorf("can not parse env var %q as int, incorrect format", str)
		return defaultValue, false
	}

	return num, true
}

func SetBool(n string, v *bool) bool {
	b, ok := Bool(n)
	if ok {
		*v = b
	}

	return ok
}

func Bool(n string, args ...bool) (bool, bool) {
	defaultValue := false
	if len(args) > 0 {
		defaultValue = args[0]
	}

	str, ok := os.LookupEnv(n)
	if !ok {
		return defaultValue, false
	}

	num, err := strconv.ParseBool(str)
	if err != nil {
		logrus.WithError(err).Errorf("can not parse env var %q as bool, incorrect format", str)
		return defaultValue, false
	}

	return num, true
}
