package bbgo

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

func TestRedisPersistentService(t *testing.T) {
	redisService := NewRedisPersistenceService(&RedisPersistenceConfig{
		Host:     "127.0.0.1",
		Port:     "6379",
		DB:       0,
	})
	assert.NotNil(t, redisService)

	store := redisService.NewStore("bbgo", "test")
	assert.NotNil(t, store)

	err := store.Reset()
	assert.NoError(t, err)

	var fp fixedpoint.Value
	err = store.Load(fp)
	assert.Error(t, err)
	assert.EqualError(t, ErrPersistenceNotExists, err.Error())

	fp = fixedpoint.NewFromFloat(3.1415)
	err = store.Save(&fp)
	assert.NoError(t, err, "should store value without error")

	var fp2 fixedpoint.Value
	err = store.Load(&fp2)
	assert.NoError(t, err, "should load value without error")
	assert.Equal(t, fp, fp2)

	err = store.Reset()
	assert.NoError(t, err)
}

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
		err := store.Save(&i)

		assert.NoError(t, err)

		var j = 0
		err = store.Load(&j)
		assert.NoError(t, err)
		assert.Equal(t, i, j)
	})
}

