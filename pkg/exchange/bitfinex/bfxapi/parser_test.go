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

// response is a test struct for parseRawArray.
type response struct {
	Field1 int            // basic int conversion
	Field2 float64        // basic float conversion
	Field3 string         // basic string conversion
	Field4 testFixedValue // using UnmarshalJSON method
	Field5 *int           // use pointer int
}

// Test_parseRawArray tests the parseRawArray function using []json.RawMessage.
func Test_parseRawArray(t *testing.T) {
	t.Run("raw message array", func(t *testing.T) {
		arr := []json.RawMessage{
			json.RawMessage("42"),
			json.RawMessage("3.14"),
			json.RawMessage("\"hello\""),
			json.RawMessage("256.0"),
			json.RawMessage("null"),
		}
		var res response
		err := parseRawArray(arr, &res, 0)
		assert.NoError(t, err, "expected no error during parseRawArray")
		assert.Equal(t, 42, res.Field1, "Field1 should be 42")
		assert.Equal(t, 3.14, res.Field2, "Field2 should be 3.14")
		assert.Equal(t, "hello", res.Field3, "Field3 should be 'hello'")
		assert.Equal(t, 256.0, res.Field4.v, "Field4 should be 256.0")
		assert.Nil(t, res.Field5, "Field5 should be nil")
	})

	// using raw json input
	t.Run("raw json input", func(t *testing.T) {
		raw := `[42, 3.14, "hello", 256.0, 123]`
		var rawMsgs []json.RawMessage
		err := json.Unmarshal([]byte(raw), &rawMsgs)
		assert.NoError(t, err, "expected no error unmarshalling raw json")
		var res2 response
		err = parseRawArray(rawMsgs, &res2, 0)
		assert.NoError(t, err, "expected no error during parseRawArray with raw json")
		assert.Equal(t, 42, res2.Field1, "Field1 should be 42")
		assert.Equal(t, 3.14, res2.Field2, "Field2 should be 3.14")
		assert.Equal(t, "hello", res2.Field3, "Field3 should be 'hello'")
		assert.Equal(t, 256.0, res2.Field4.v, "Field4 should be 256.0")
		assert.Equal(t, 123, *res2.Field5, "Field5 should be 123")
	})
}
