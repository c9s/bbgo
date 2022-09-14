package dynamic

import (
	"reflect"
	"testing"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/stretchr/testify/assert"
)

type Strategy struct {
	Field1 fixedpoint.Value `json:"field1" modifiable:"true"`
	Field2 float64          `json:"field2"`
	field3 float64          `json:"field3" modifiable:"true"`
}

func TestGetModifiableFields(t *testing.T) {
	s := Strategy{}
	val := reflect.ValueOf(s)
	GetModifiableFields(val, func(tagName, name string) {
		assert.Equal(t, tagName, "field1")
		assert.Equal(t, name, "Field1")
	})
}

func TestGetModifiableField(t *testing.T) {
	s := Strategy{}
	val := reflect.ValueOf(s)
	_, ok := GetModifiableField(val, "Field1")
	assert.True(t, ok)
	_, ok = GetModifiableField(val, "Field2")
	assert.False(t, ok)
	_, ok = GetModifiableField(val, "Field3")
	assert.False(t, ok)
	_, ok = GetModifiableField(val, "Random")
	assert.False(t, ok)
}
