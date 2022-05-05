package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMemoryService(t *testing.T) {
	t.Run("load_empty", func(t *testing.T) {
		service := NewMemoryService()
		store := service.NewStore("test")

		j := 0
		err := store.Load(&j)
		assert.Error(t, err)
	})

	t.Run("save_and_load", func(t *testing.T) {
		service := NewMemoryService()
		store := service.NewStore("test")

		i := 3
		err := store.Save(i)

		assert.NoError(t, err)

		var j = 0
		err = store.Load(&j)
		assert.NoError(t, err)
		assert.Equal(t, i, j)
	})
}
