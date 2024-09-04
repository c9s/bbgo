package xmaker

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"

	. "github.com/c9s/bbgo/pkg/testing/testhelper"
)

var tradeId = 0

func Trade(symbol string, side types.SideType, price, quantity fixedpoint.Value, t time.Time) types.Trade {
	tradeId++
	return types.Trade{
		ID:       uint64(tradeId),
		Symbol:   symbol,
		Side:     side,
		Price:    price,
		IsBuyer:  side == types.SideTypeBuy,
		Quantity: quantity,
		Time:     types.Time(t),
	}
}

func TestMarketTradeWindowSignal(t *testing.T) {
	now := time.Now()
	symbol := "BTCUSDT"
	sig := &TradeVolumeWindowSignal{
		symbol:    symbol,
		Threshold: fixedpoint.NewFromFloat(0.65),
		Window:    types.Duration(time.Minute),
	}

	sig.trades = []types.Trade{
		Trade(symbol, types.SideTypeBuy, Number(18000.0), Number(1.0), now.Add(-2*time.Minute)),
		Trade(symbol, types.SideTypeSell, Number(18000.0), Number(0.5), now.Add(-2*time.Second)),
		Trade(symbol, types.SideTypeBuy, Number(18000.0), Number(1.0), now.Add(-1*time.Second)),
	}

	ctx := context.Background()
	sigNum, err := sig.CalculateSignal(ctx)
	if assert.NoError(t, err) {
		// buy ratio: 1/1.5 = 0.6666666666666666
		// sell ratio: 0.5/1.5 = 0.3333333333333333
		assert.InDelta(t, 0.0083333, sigNum, 0.0001)
	}

	assert.Len(t, sig.trades, 2)
}
