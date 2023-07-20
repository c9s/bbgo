package dynamic

import (
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

type TestEmbedded struct {
	Foo int `persistence:"foo"`
	Bar int `persistence:"bar"`
}

type TestA struct {
	*TestEmbedded
	Outer int `persistence:"outer"`
}

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

func TestIterateFieldsByTag(t *testing.T) {
	t.Run("nested", func(t *testing.T) {
		var a = struct {
			A int `persistence:"a"`
			B int `persistence:"b"`
			C *struct {
				D int `persistence:"d"`
				E int `persistence:"e"`
			}
		}{
			A: 1,
			B: 2,
			C: &struct {
				D int `persistence:"d"`
				E int `persistence:"e"`
			}{
				D: 3,
				E: 4,
			},
		}

		collectedTags := []string{}
		cnt := 0
		err := IterateFieldsByTag(&a, "persistence", true, func(tag string, ft reflect.StructField, fv reflect.Value) error {
			cnt++
			collectedTags = append(collectedTags, tag)
			return nil
		})
		assert.NoError(t, err)
		assert.Equal(t, 4, cnt)
		assert.Equal(t, []string{"a", "b", "d", "e"}, collectedTags)
	})

	t.Run("nested nil", func(t *testing.T) {
		var a = struct {
			A int `persistence:"a"`
			B int `persistence:"b"`
			C *struct {
				D int `persistence:"d"`
				E int `persistence:"e"`
			}
		}{
			A: 1,
			B: 2,
			C: nil,
		}

		collectedTags := []string{}
		cnt := 0
		err := IterateFieldsByTag(&a, "persistence", true, func(tag string, ft reflect.StructField, fv reflect.Value) error {
			cnt++
			collectedTags = append(collectedTags, tag)
			return nil
		})
		assert.NoError(t, err)
		assert.Equal(t, 2, cnt)
		assert.Equal(t, []string{"a", "b"}, collectedTags)
	})

	t.Run("embedded", func(t *testing.T) {
		a := &TestA{
			TestEmbedded: &TestEmbedded{Foo: 1, Bar: 2},
			Outer:        3,
		}

		collectedTags := []string{}
		cnt := 0
		err := IterateFieldsByTag(a, "persistence", true, func(tag string, ft reflect.StructField, fv reflect.Value) error {
			cnt++
			collectedTags = append(collectedTags, tag)
			return nil
		})
		assert.NoError(t, err)
		assert.Equal(t, 3, cnt)
		assert.Equal(t, []string{"foo", "bar", "outer"}, collectedTags)
	})
}
