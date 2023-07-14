package types

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/pkg/errors"
)

var simpleDurationRegExp = regexp.MustCompile(`^(\d+)([hdw])$`)

var ErrNotSimpleDuration = errors.New("the given input is not simple duration format, valid format: [1-9][0-9]*[hdw]")

type SimpleDuration struct {
	Num      int
	Unit     string
	Duration Duration
}

func (d *SimpleDuration) String() string {
	return fmt.Sprintf("%d%s", d.Num, d.Unit)
}

func (d *SimpleDuration) Interval() Interval {
	switch d.Unit {

	case "d":
		return Interval1d
	case "h":
		return Interval1h

	case "w":
		return Interval1w

	}

	return ""
}

func (d *SimpleDuration) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	sd, err := ParseSimpleDuration(s)
	if err != nil {
		return err
	}

	if sd != nil {
		*d = *sd
	}
	return nil
}

func ParseSimpleDuration(s string) (*SimpleDuration, error) {
	if s == "" {
		return nil, nil
	}

	if !simpleDurationRegExp.MatchString(s) {
		return nil, errors.Wrapf(ErrNotSimpleDuration, "input %q is not a simple duration", s)
	}

	matches := simpleDurationRegExp.FindStringSubmatch(s)
	numStr := matches[1]
	unit := matches[2]
	num, err := strconv.Atoi(numStr)
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

// Duration
type Duration time.Duration

func (d *Duration) Duration() time.Duration {
	return time.Duration(*d)
}

func (d *Duration) UnmarshalJSON(data []byte) error {
	var o interface{}

	if err := json.Unmarshal(data, &o); err != nil {
		return err
	}

	switch t := o.(type) {
	case string:
		sd, err := ParseSimpleDuration(t)
		if err == nil {
			*d = sd.Duration
			return nil
		}

		dd, err := time.ParseDuration(t)
		if err != nil {
			return err
		}

		*d = Duration(dd)

	case float64:
		*d = Duration(int64(t * float64(time.Second)))

	case int64:
		*d = Duration(t * int64(time.Second))
	case int:
		*d = Duration(t * int(time.Second))

	default:
		return fmt.Errorf("unsupported type %T value: %v", t, t)

	}

	return nil
}
