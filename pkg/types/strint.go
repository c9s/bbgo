package types

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// StrInt64 is a string type for int64
type StrInt64 int64

func NewStrInt64FromString(ss string) (StrInt64, error) {
	i, err := strconv.ParseInt(ss, 10, 64)
	if err != nil {
		return 0, err
	}

	return StrInt64(i), nil
}

func (s *StrInt64) UnmarshalYAML(unmarshal func(a interface{}) error) (err error) {
	var a int64
	if err = unmarshal(&a); err == nil {
		*s = StrInt64(a)
		return
	}

	var ss string
	if err = unmarshal(&ss); err == nil {
		s2, err2 := NewStrInt64FromString(ss)
		if err2 != nil {
			return err2
		}

		*s = s2
		return
	}

	return fmt.Errorf("StrInt64.UnmarshalYAML error: unsupported value type, not int64 or string: %w", err)
}

func (s *StrInt64) MarshalJSON() ([]byte, error) {
	ss := strconv.FormatInt(int64(*s), 10)
	return json.Marshal(ss)
}

func (s *StrInt64) UnmarshalJSON(body []byte) error {
	var arg interface{}
	if err := json.Unmarshal(body, &arg); err != nil {
		return err
	}

	switch ta := arg.(type) {
	case string:
		s2, err := NewStrInt64FromString(ta)
		if err != nil {
			return err
		}

		*s = StrInt64(s2)

	case int64:
		*s = StrInt64(ta)
	case int32:
		*s = StrInt64(ta)
	case int:
		*s = StrInt64(ta)

	case float32:
		*s = StrInt64(ta)

	case float64:
		*s = StrInt64(ta)

	default:
		return fmt.Errorf("StrInt64 error: unsupported value type %T", ta)
	}

	return nil
}

func (s *StrInt64) String() string {
	if s == nil {
		return ""
	}
	return strconv.FormatInt(int64(*s), 10)
}
