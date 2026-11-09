package bbgo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/rpcexport"
)

// mockRPCExportingStrategy is a minimal CrossExchangeStrategy that also
// implements RPCExporter, used to verify Trader auto-detects it.
type mockRPCExportingStrategy struct {
	exported *rpcexport.Registry
}

func (s *mockRPCExportingStrategy) ID() string { return "mock-rpc-strategy" }

func (s *mockRPCExportingStrategy) InstanceID() string { return "mock-rpc-strategy:test" }

func (s *mockRPCExportingStrategy) CrossRun(_ context.Context, _ OrderExecutionRouter, _ map[string]*ExchangeSession) error {
	return nil
}

func (s *mockRPCExportingStrategy) ExportRPC(reg *rpcexport.Registry) {
	reg.Export("ping", "returns pong", false, func() (string, error) {
		return "pong", nil
	})
	s.exported = reg
}

func TestTrader_Initialize_DetectsRPCExporter(t *testing.T) {
	environ := NewEnvironment()
	trader := NewTrader(environ)

	strategy := &mockRPCExportingStrategy{}
	trader.AttachCrossExchangeStrategy(strategy)

	require.NoError(t, trader.Initialize(context.Background()))

	instances := trader.ListRPCInstances()
	require.Len(t, instances, 1)
	assert.Equal(t, "mock-rpc-strategy:test", instances[0].InstanceID)
	assert.Equal(t, "mock-rpc-strategy", instances[0].StrategyID)

	reg, ok := trader.RPCRegistry("mock-rpc-strategy:test")
	require.True(t, ok)
	assert.Same(t, strategy.exported, reg)

	methods := reg.Methods()
	require.Len(t, methods, 1)
	assert.Equal(t, "ping", methods[0].Name)
}

func TestTrader_Initialize_NoRPCExporter(t *testing.T) {
	environ := NewEnvironment()
	trader := NewTrader(environ)

	assert.Empty(t, trader.ListRPCInstances())

	_, ok := trader.RPCRegistry("does-not-exist")
	assert.False(t, ok)
}
