package util

import (
	"os"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
)

func GetEnvVarDuration(n string) (time.Duration, bool) {
	str, ok := os.LookupEnv(n)
	if !ok {
		return 0, false
	}

	du, err := time.ParseDuration(str)
	if err != nil {
		logrus.WithError(err).Errorf("can not parse env var %q as time.Duration, incorrect format", str)
		return 0, false
	}

	return du, true
}

func GetEnvVarInt(n string) (int, bool) {
	str, ok := os.LookupEnv(n)
	if !ok {
		return 0, false
	}

	num, err := strconv.Atoi(str)
	if err != nil {
		logrus.WithError(err).Errorf("can not parse env var %q as int, incorrect format", str)
		return 0, false
	}

	return num, true
}

func SetEnvVarBool(n string, v *bool) bool {
	b, ok := GetEnvVarBool(n)
	if ok {
		*v = b
	}

	return ok
}

func GetEnvVarBool(n string) (bool, bool) {
	str, ok := os.LookupEnv(n)
	if !ok {
		return false, false
	}

	num, err := strconv.ParseBool(str)
	if err != nil {
		logrus.WithError(err).Errorf("can not parse env var %q as bool, incorrect format", str)
		return false, false
	}

	return num, true
}
