package strint

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// Int64 is a string type for int64
type Int64 int64

func NewFromString(ss string) (Int64, error) {
	i, err := strconv.ParseInt(ss, 10, 64)
	if err != nil {
		return 0, err
	}

	return Int64(i), nil
}

func (s *Int64) UnmarshalYAML(unmarshal func(a interface{}) error) (err error) {
	var a int64
	if err = unmarshal(&a); err == nil {
		*s = Int64(a)
		return
	}

	var ss string
	if err = unmarshal(&ss); err == nil {
		s2, err2 := NewFromString(ss)
		if err2 != nil {
			return err2
		}

		*s = s2
		return
	}

	return fmt.Errorf("Int64.UnmarshalYAML error: unsupported value type, not int64 or string: %w", err)
}

func (s *Int64) MarshalJSON() ([]byte, error) {
	ss := strconv.FormatInt(int64(*s), 10)
	return json.Marshal(ss)
}

func (s *Int64) UnmarshalJSON(body []byte) error {
	var arg any
	if err := json.Unmarshal(body, &arg); err != nil {
		return err
	}

	val, err := cast(arg)
	if err != nil {
		return err
	}

	*s = Int64(val)
	return nil
}

func (s *Int64) String() string {
	if s == nil {
		return ""
	}
	return strconv.FormatInt(int64(*s), 10)
}

func cast(arg any) (int64, error) {
	switch ta := arg.(type) {
	case string:
		s2, err := strconv.ParseInt(ta, 10, 64)
		if err != nil {
			return 0, err
		}

		return s2, nil
	case int64:
		return ta, nil
	case int32:
		return int64(ta), nil
	case int:
		return int64(ta), nil

	case float32:
		return int64(ta), nil

	case float64:
		return int64(ta), nil

	default:
		return 0, fmt.Errorf("strint error: unsupported value type %T", ta)
	}
}
