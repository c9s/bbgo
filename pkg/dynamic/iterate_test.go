package dynamic

import (
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIterateFields(t *testing.T) {
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
}
