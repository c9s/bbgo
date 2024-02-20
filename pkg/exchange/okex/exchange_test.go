package okex

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func Test_clientOrderIdRegex(t *testing.T) {
	t.Run("empty client order id", func(t *testing.T) {
		assert.True(t, clientOrderIdRegex.MatchString(""))
	})

	t.Run("mixed of digit and char", func(t *testing.T) {
		assert.True(t, clientOrderIdRegex.MatchString("1s2f3g4h5j"))
	})

	t.Run("mixed of 16 chars and 16 digit", func(t *testing.T) {
		assert.True(t, clientOrderIdRegex.MatchString(strings.Repeat("s", 16)+strings.Repeat("1", 16)))
	})

	t.Run("out of maximum length", func(t *testing.T) {
		assert.False(t, clientOrderIdRegex.MatchString(strings.Repeat("s", 33)))
	})

	t.Run("invalid char: `-`", func(t *testing.T) {
		assert.False(t, clientOrderIdRegex.MatchString(uuid.NewString()))
	})
}
