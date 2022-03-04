package bbgo

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

func Test_parseStructAndInject(t *testing.T) {
	t.Run("pointer", func(t *testing.T) {
		ss := struct {
			a   int
			Env *Environment
		}{
			a:   1,
			Env: nil,
		}
		err := parseStructAndInject(&ss, &Environment{})
		assert.NoError(t, err)
		assert.NotNil(t, ss.Env)
	})
	t.Run("struct", func(t *testing.T) {
		ss := struct {
			a   int
			Env Environment
		}{
			a: 1,
		}
		err := parseStructAndInject(&ss, Environment{
			startTime: time.Now(),
		})
		assert.NoError(t, err)
		assert.NotEqual(t, time.Time{}, ss.Env.startTime)
	})
	t.Run("interface/any", func(t *testing.T) {
		ss := struct {
			Any interface{} // anything
		}{
			Any: nil,
		}
		err := parseStructAndInject(&ss, &Environment{
			startTime: time.Now(),
		})
		assert.NoError(t, err)
		assert.NotNil(t, ss.Any)
	})
	t.Run("interface/stringer", func(t *testing.T) {
		ss := struct {
			Stringer types.Stringer // stringer interface
		}{
			Stringer: nil,
		}
		err := parseStructAndInject(&ss, &types.Trade{})
		assert.NoError(t, err)
		assert.NotNil(t, ss.Stringer)
	})
}
