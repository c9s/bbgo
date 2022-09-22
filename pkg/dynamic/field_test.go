package dynamic

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/stretchr/testify/assert"
)

type Inner struct {
	Field5 float64 `json:"field5,omitempty" modifiable:"true"`
}

type InnerPointer struct {
	Field6 float64 `json:"field6" modifiable:"true"`
}

type Strategy struct {
	Inner
	*InnerPointer
	Field1 fixedpoint.Value  `json:"field1" modifiable:"true"`
	Field2 float64           `json:"field2"`
	field3 float64           `json:"field3" modifiable:"true"`
	Field4 *fixedpoint.Value `json:"field4" modifiable:"true"`
}

func TestGetModifiableFields(t *testing.T) {
	s := Strategy{}
	val := reflect.ValueOf(s)
	GetModifiableFields(val, func(tagName, name string) {
		assert.NotEqual(t, tagName, "field2")
		assert.NotEqual(t, name, "Field2")
		assert.NotEqual(t, tagName, "field3")
		assert.NotEqual(t, name, "Field3")
	})
}

func TestGetModifiableField(t *testing.T) {
	// val must be get from pointer.Elem(), otherwise the fields will be unaddressable
	s := &Strategy{Field1: fixedpoint.NewFromInt(1)}
	val := reflect.ValueOf(s).Elem()
	_, ok := GetModifiableField(val, "Field1")
	assert.True(t, ok)
	_, ok = GetModifiableField(val, "Field5")
	assert.True(t, ok)
	_, ok = GetModifiableField(val, "Field6")
	assert.False(t, ok)
	s.InnerPointer = &InnerPointer{}
	_, ok = GetModifiableField(val, "Field6")
	assert.True(t, ok)
	_, ok = GetModifiableField(val, "Field2")
	assert.False(t, ok)
	_, ok = GetModifiableField(val, "Field3")
	assert.False(t, ok)
	_, ok = GetModifiableField(val, "Random")
	assert.False(t, ok)
	field, ok := GetModifiableField(val, "Field1")
	assert.True(t, ok)
	x := reflect.New(field.Type())
	xi := x.Interface()
	assert.NoError(t, json.Unmarshal([]byte("\"3.1415%\""), &xi))
	assert.True(t, field.CanAddr())
	field.Set(x.Elem())
	assert.Equal(t, s.Field1.String(), "0.031415")
	field, _ = GetModifiableField(val, "Field4")
	x = reflect.New(field.Type())
	xi = x.Interface()
	assert.NoError(t, json.Unmarshal([]byte("311"), &xi))
	field.Set(x.Elem())
	assert.Equal(t, s.Field4.String(), "311")
}
