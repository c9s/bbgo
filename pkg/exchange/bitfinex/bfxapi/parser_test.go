package bfxapi

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

// testFixedValue is a mock type implementing json.Unmarshaler.
type testFixedValue struct {
	v float64
}

// UnmarshalJSON implements json.Unmarshaler for testFixedValue.
func (f *testFixedValue) UnmarshalJSON(data []byte) error {
	var temp float64
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}
	f.v = temp
	return nil
}

// response is a test struct for parseArray.
type response struct {
	Field1 int            // basic int conversion
	Field2 float64        // basic float conversion
	Field3 string         // basic string conversion
	Field4 testFixedValue // using UnmarshalJSON method
}

// TestParseArray tests the parseArray function using []json.RawMessage.
func TestParseArray(t *testing.T) {
	t.Run("raw message array", func(t *testing.T) {
		arr := []json.RawMessage{
			json.RawMessage("42"),
			json.RawMessage("3.14"),
			json.RawMessage("\"hello\""),
			json.RawMessage("256.0"),
		}
		var res response
		err := parseArray(arr, &res)
		assert.NoError(t, err, "expected no error during parseArray")
		assert.Equal(t, 42, res.Field1, "Field1 should be 42")
		assert.Equal(t, 3.14, res.Field2, "Field2 should be 3.14")
		assert.Equal(t, "hello", res.Field3, "Field3 should be 'hello'")
		assert.Equal(t, 256.0, res.Field4.v, "Field4 should be 256.0")
	})

	// new sub-test using raw json input
	t.Run("raw json input", func(t *testing.T) {
		raw := `[42, 3.14, "hello", 256.0]`
		var rawMsgs []json.RawMessage
		err := json.Unmarshal([]byte(raw), &rawMsgs)
		assert.NoError(t, err, "expected no error unmarshalling raw json")
		var res2 response
		err = parseArray(rawMsgs, &res2)
		assert.NoError(t, err, "expected no error during parseArray with raw json")
		assert.Equal(t, 42, res2.Field1, "Field1 should be 42")
		assert.Equal(t, 3.14, res2.Field2, "Field2 should be 3.14")
		assert.Equal(t, "hello", res2.Field3, "Field3 should be 'hello'")
		assert.Equal(t, 256.0, res2.Field4.v, "Field4 should be 256.0")
	})
}
