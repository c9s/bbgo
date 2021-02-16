package bbgo

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/service"
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
