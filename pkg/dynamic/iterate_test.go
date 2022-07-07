package dynamic

import (
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIterateFields(t *testing.T) {

	t.Run("basic", func(t *testing.T) {
		var a = struct {
			A int
			B float64
			C *os.File
		}{}

		cnt := 0
		err := IterateFields(&a, func(ft reflect.StructField, fv reflect.Value) error {
			cnt++
			return nil
		})
		assert.NoError(t, err)
		assert.Equal(t, 3, cnt)
	})

	t.Run("non-ptr", func(t *testing.T) {
		err := IterateFields(struct{}{}, func(ft reflect.StructField, fv reflect.Value) error {
			return nil
		})
		assert.Error(t, err)
	})

	t.Run("nil", func(t *testing.T) {
		err := IterateFields(nil, func(ft reflect.StructField, fv reflect.Value) error {
			return nil
		})
		assert.Error(t, err)
	})


}
