package bbgo

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type myStrategy struct {
	Symbol   string `json:"symbol"`
	Position *types.Position
}

func (m *myStrategy) ID() string {
	return "mystrategy"
}

func (m *myStrategy) InstanceID() string {
	return fmt.Sprintf("%s:%s", m.ID(), m.Symbol)
}

func (m *myStrategy) ClosePosition(ctx context.Context, percentage fixedpoint.Value) error {
	return nil
}

func (m *myStrategy) Run(ctx context.Context, orderExecutor OrderExecutor, session *ExchangeSession) error {
	return nil
}

func Test_getStrategySignature(t *testing.T) {
	signature, err := getStrategySignature(&myStrategy{
		Symbol: "BTCUSDT",
	})
	assert.NoError(t, err)
	assert.Equal(t, "mystrategy:BTCUSDT", signature)
}

func Test_hasTypeField(t *testing.T) {
	s := &myStrategy{
		Symbol: "BTCUSDT",
	}
	ok := hasTypeField(s, &types.Position{})
	assert.True(t, ok)
}

func Test_testInterface(t *testing.T) {
	s := &myStrategy{
		Symbol: "BTCUSDT",
	}

	ok := testInterface(s, (*PositionCloser)(nil))
	assert.True(t, ok)
}
