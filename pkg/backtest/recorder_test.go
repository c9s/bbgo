package backtest

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type testStrategy struct {
	Symbol string

	Position *types.Position
}

func (s *testStrategy) ID() string         { return "my-test" }
func (s *testStrategy) InstanceID() string { return "my-test:" + s.Symbol }

func TestStateRecorder(t *testing.T) {
	tmpDir, _ := os.MkdirTemp(os.TempDir(), "bbgo")
	t.Logf("tmpDir: %s", tmpDir)

	st := &testStrategy{
		Symbol:   "BTCUSDT",
		Position: types.NewPosition("BTCUSDT", "BTC", "USDT"),
	}

	recorder := NewStateRecorder(tmpDir)
	err := recorder.Scan(st)
	assert.NoError(t, err)
	assert.Len(t, recorder.writers, 1)

	st.Position.AddTrade(types.Trade{
		OrderID:       1,
		Exchange:      types.ExchangeBinance,
		Price:         fixedpoint.NewFromFloat(18000.0),
		Quantity:      fixedpoint.NewFromFloat(1.0),
		QuoteQuantity: fixedpoint.NewFromFloat(18000.0),
		Symbol:        "BTCUSDT",
		Side:          types.SideTypeBuy,
		IsBuyer:       true,
		IsMaker:       false,
		Time:          types.Time(time.Now()),
		Fee:           fixedpoint.NewFromFloat(0.00001),
		FeeCurrency:   "BNB",
		IsMargin:      false,
		IsFutures:     false,
		IsIsolated:    false,
	})

	n, err := recorder.Snapshot()
	assert.NoError(t, err)
	assert.Equal(t, 1, n)

	err = recorder.Close()
	assert.NoError(t, err)
}
