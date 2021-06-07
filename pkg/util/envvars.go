package util

import (
	"os"
	"strconv"

	"github.com/sirupsen/logrus"
)

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
