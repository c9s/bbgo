package dynamic

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

func Test_convertToStr(t *testing.T) {
	t.Run("str-str", func(t *testing.T) {
		out := convertToStr(reflect.ValueOf("a"))
		assert.Equal(t, "a", out)
	})

	t.Run("bool-str", func(t *testing.T) {
		out := convertToStr(reflect.ValueOf(false))
		assert.Equal(t, "false", out)

		out = convertToStr(reflect.ValueOf(true))
		assert.Equal(t, "true", out)
	})

	t.Run("float-str", func(t *testing.T) {
		out := convertToStr(reflect.ValueOf(0.444))
		assert.Equal(t, "0.444", out)
	})

	t.Run("int-str", func(t *testing.T) {
		a := int(123)
		out := convertToStr(reflect.ValueOf(a))
		assert.Equal(t, "123", out)
	})

	t.Run("uint-str", func(t *testing.T) {
		a := uint(123)
		out := convertToStr(reflect.ValueOf(a))
		assert.Equal(t, "123", out)
	})

	t.Run("fixedpoint-str", func(t *testing.T) {
		a := fixedpoint.NewFromInt(100)
		out := convertToStr(reflect.ValueOf(a))
		assert.Equal(t, "100", out)
	})
}
