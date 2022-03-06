package bbgo

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
)

func Test_injectField(t *testing.T) {
	type TT struct {
		TradeService *service.TradeService
	}

	// only pointer object can be set.
	var tt = &TT{}

	// get the value of the pointer, or it can not be set.
	var rv = reflect.ValueOf(tt).Elem()

	_, ret := hasField(rv, "TradeService")
	assert.True(t, ret)

	ts := &service.TradeService{}

	err := injectField(rv, "TradeService", ts, true)
	assert.NoError(t, err)
}

func Test_parseStructAndInject(t *testing.T) {
	t.Run("skip nil", func(t *testing.T) {
		ss := struct {
			a   int
			Env *Environment
		}{
			a:   1,
			Env: nil,
		}
		err := parseStructAndInject(&ss, nil)
		assert.NoError(t, err)
		assert.Nil(t, ss.Env)
	})
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

	t.Run("composition", func(t *testing.T) {
		type TT struct {
			*service.TradeService
		}
		ss := &TT{}
		err := parseStructAndInject(&ss, &service.TradeService{})
		assert.NoError(t, err)
		assert.NotNil(t, ss.TradeService)
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
