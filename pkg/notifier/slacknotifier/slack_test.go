package slacknotifier

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

func Test_objectName(t *testing.T) {
	t.Run("deposit", func(t *testing.T) {
		var deposit = &types.Deposit{}
		assert.Equal(t, "Deposit", objectName(deposit))
	})

	t.Run("local type", func(t *testing.T) {
		type A struct{}
		var obj = &A{}
		assert.Equal(t, "A", objectName(obj))
	})
}
