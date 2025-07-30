package max

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

func TestNewClientOrderID(t *testing.T) {
	t.Run("should return empty string when originalID is NoClientOrderID", func(t *testing.T) {
		result := newClientOrderID(types.NoClientOrderID)
		assert.Equal(t, "", result)
	})

	t.Run("should return originalID when it is not empty", func(t *testing.T) {
		originalID := "test-original-id"
		result := newClientOrderID(originalID)
		assert.Equal(t, originalID, result)
	})

	t.Run("should generate client order ID with prefix and tags", func(t *testing.T) {
		result := newClientOrderID("", "tag1", "tag2")
		assert.Contains(t, result, "x-bbgo-tag1-tag2-")
		assert.LessOrEqual(t, len(result), 36)
	})

	t.Run("should truncate client order ID to 36 characters if it exceeds the limit", func(t *testing.T) {
		result := newClientOrderID("", "verylongtag1", "verylongtag2", "verylongtag3")
		assert.Equal(t, 36, len(result))
	})

	t.Run("should generate client order ID with prefix and tags", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			id := newClientOrderID("", "tag1")
			t.Logf("client order ID: %s", id)
			assert.Contains(t, id, "x-bbgo-tag1")
			assert.LessOrEqual(t, len(id), 36)
		}
	})

	t.Run("should generate client order ID with prefix", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			id := newClientOrderID("")
			t.Logf("client order ID: %s", id)
			assert.Contains(t, id, "x-bbgo")
			assert.LessOrEqual(t, len(id), 36)
		}
	})

	t.Run("uuid", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			id := uuid.New().String()
			t.Logf("uuid: %s", id)
		}
	})
}
