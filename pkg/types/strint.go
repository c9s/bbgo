package types

import (
	"encoding/json"
	"fmt"
	"strconv"
)

type StrInt64 int64

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
		// parse string
		i, err := strconv.ParseInt(ta, 10, 64)
		if err != nil {
			return err
		}
		*s = StrInt64(i)

	case int64:
		*s = StrInt64(ta)
	case int32:
		*s = StrInt64(ta)
	case int:
		*s = StrInt64(ta)

	default:
		return fmt.Errorf("StrInt64 error: unsupported value type %T", ta)
	}

	return nil
}
