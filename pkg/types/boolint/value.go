package boolint

import (
	"encoding/json"
	"fmt"
)

type Value bool

func (v Value) String() string {
	if v {
		return "true"
	}
	return "false"
}

// UnmarshalJSON decodes JSON data for a Value.
// It supports both boolean values (true/false) and integer values (0/1).
func (v *Value) UnmarshalJSON(data []byte) error {
	// Try unmarshalling data into a bool.
	var b bool
	if err := json.Unmarshal(data, &b); err == nil {
		*v = Value(b)
		return nil
	}

	// Try unmarshalling data into an int.
	var n int
	if err := json.Unmarshal(data, &n); err == nil {
		if n == 1 {
			*v = Value(true)
			return nil
		} else if n == 0 {
			*v = Value(false)
			return nil
		} else {
			return fmt.Errorf("invalid integer value for bool: %d", n)
		}
	}

	return fmt.Errorf("failed to unmarshal JSON data into boolint.Value: %s", data)
}
