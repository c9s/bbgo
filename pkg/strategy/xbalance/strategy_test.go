package xbalance

import (
	"testing"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/stretchr/testify/assert"
)

func TestState_PlainText(t *testing.T) {
	var state = State{
		Asset:                  "USDT",
		DailyNumberOfTransfers: 1,
		DailyAmountOfTransfers: fixedpoint.NewFromFloat(1000.0),
		Since:                  0,
	}

	assert.Equal(t, "USDT transfer stats:\ndaily number of transfers: 1\ndaily amount of transfers 1000", state.PlainText())
}
