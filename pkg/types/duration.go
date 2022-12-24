package types

import (
	"regexp"
	"strconv"
	"time"

	"github.com/pkg/errors"
)

var simpleDurationRegExp = regexp.MustCompile("^(\\d+)[hdw]$")

var ErrNotSimpleDuration = errors.New("the given input is not simple duration format")

type SimpleDuration struct {
	Num      int64
	Unit     string
	Duration Duration
}

func ParseSimpleDuration(s string) (*SimpleDuration, error) {
	if !simpleDurationRegExp.MatchString(s) {
		return nil, errors.Wrapf(ErrNotSimpleDuration, "input %q is not a simple duration", s)
	}

	matches := simpleDurationRegExp.FindStringSubmatch(s)
	numStr := matches[1]
	unit := matches[2]
	num, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return nil, err
	}

	switch unit {
	case "d":
		d := Duration(time.Duration(num) * 24 * time.Hour)
		return &SimpleDuration{num, unit, d}, nil
	case "w":
		d := Duration(time.Duration(num) * 7 * 24 * time.Hour)
		return &SimpleDuration{num, unit, d}, nil
	case "h":
		d := Duration(time.Duration(num) * time.Hour)
		return &SimpleDuration{num, unit, d}, nil
	}

	return nil, errors.Wrapf(ErrNotSimpleDuration, "input %q is not a simple duration", s)
}
