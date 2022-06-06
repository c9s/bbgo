package bbgo

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

type myStrategy struct {
	Symbol string `json:"symbol"`
}

func (m myStrategy) ID() string {
	return "mystrategy"
}

func (m myStrategy) InstanceID() string {
	return fmt.Sprintf("%s:%s", m.ID(), m.Symbol)
}

func (m *myStrategy) Run(ctx context.Context, orderExecutor OrderExecutor, session *ExchangeSession) error {
	return nil
}

func Test_getStrategySignature(t *testing.T) {
	signature, err := getStrategySignature(&myStrategy{
		Symbol: "BTCUSDT",
	})
	assert.NoError(t, err)
	assert.Equal(t, "mystrategy:BTCUSDT.BTCUSDT", signature)
}
