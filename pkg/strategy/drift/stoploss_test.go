package drift

import (
	"testing"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/stretchr/testify/assert"
)

func Test_StopLossLong(t *testing.T) {
	s := &Strategy{}
	s.highestPrice = 30.
	s.buyPrice = 30.
	s.lowestPrice = 29.7
	s.StopLoss = fixedpoint.NewFromFloat(0.01)
	s.UseAtr = false
	s.UseStopLoss = true
	assert.True(t, s.CheckStopLoss())
}

func Test_StopLossShort(t *testing.T) {
	s := &Strategy{}
	s.lowestPrice = 30.
	s.sellPrice = 30.
	s.highestPrice = 30.3
	s.StopLoss = fixedpoint.NewFromFloat(0.01)
	s.UseAtr = false
	s.UseStopLoss = true
	assert.True(t, s.CheckStopLoss())
}

func Test_ATRLong(t *testing.T) {
	s := &Strategy{}
	s.highestPrice = 30.
	s.buyPrice = 30.
	s.lowestPrice = 28.7
	s.UseAtr = true
	s.UseStopLoss = false
	s.atr = &indicator.ATR{RMA: &indicator.RMA{
		Values: floats.Slice{1., 1.2, 1.3},
	}}
	assert.True(t, s.CheckStopLoss())
}

func Test_ATRShort(t *testing.T) {
	s := &Strategy{}
	s.highestPrice = 31.3
	s.sellPrice = 30.
	s.lowestPrice = 30.
	s.UseAtr = true
	s.UseStopLoss = false
	s.atr = &indicator.ATR{RMA: &indicator.RMA{
		Values: floats.Slice{1., 1.2, 1.3},
	}}
	assert.True(t, s.CheckStopLoss())
}
