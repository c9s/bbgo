package xmaker

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/c9s/bbgo/pkg/bbgo"
	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/types"
)

func newMaintenanceWindow(now time.Time) *bbgo.MaintenanceConfig {
	past := types.LooseFormatTime(now.Add(-time.Hour))
	future := types.LooseFormatTime(now.Add(time.Hour))
	return &bbgo.MaintenanceConfig{StartTime: &past, EndTime: &future}
}

func newConnectedConnectivityGroup() *types.ConnectivityGroup {
	conn := types.NewConnectivity()
	stream := &types.StandardStream{}
	conn.Bind(stream)
	stream.EmitConnect()
	return types.NewConnectivityGroup(conn)
}

func newDisconnectedConnectivityGroup() *types.ConnectivityGroup {
	conn := types.NewConnectivity()
	stream := &types.StandardStream{}
	conn.Bind(stream)
	return types.NewConnectivityGroup(conn)
}

func TestHedgeMarket_isAvailable(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	market := Market("BTCUSDT")
	depth := Number(100.0)

	t.Run("available by default", func(t *testing.T) {
		session, _, _ := newMockSession(mockCtrl, ctx, market.Symbol)
		hm := NewHedgeMarket(&HedgeMarketConfig{
			SymbolSelector: market.Symbol, HedgeInterval: hedgeInterval, QuotingDepth: depth,
		}, session, market)
		assert.NoError(t, hm.stream.Connect(ctx))

		assert.True(t, hm.isAvailable())
	})

	t.Run("unavailable when session is in maintenance", func(t *testing.T) {
		session, _, _ := newMockSession(mockCtrl, ctx, market.Symbol)
		session.Maintenance = newMaintenanceWindow(time.Now())
		hm := NewHedgeMarket(&HedgeMarketConfig{
			SymbolSelector: market.Symbol, HedgeInterval: hedgeInterval, QuotingDepth: depth,
		}, session, market)
		assert.NoError(t, hm.stream.Connect(ctx))

		assert.False(t, hm.isAvailable())
	})

	t.Run("unavailable when session is disconnected", func(t *testing.T) {
		session, _, _ := newMockSession(mockCtrl, ctx, market.Symbol)
		session.Connectivity = newDisconnectedConnectivityGroup()
		hm := NewHedgeMarket(&HedgeMarketConfig{
			SymbolSelector: market.Symbol, HedgeInterval: hedgeInterval, QuotingDepth: depth,
		}, session, market)
		assert.NoError(t, hm.stream.Connect(ctx))

		assert.False(t, hm.isAvailable())
	})

	t.Run("available when session is connected and not in maintenance", func(t *testing.T) {
		session, _, _ := newMockSession(mockCtrl, ctx, market.Symbol)
		session.Connectivity = newConnectedConnectivityGroup()
		hm := NewHedgeMarket(&HedgeMarketConfig{
			SymbolSelector: market.Symbol, HedgeInterval: hedgeInterval, QuotingDepth: depth,
		}, session, market)
		assert.NoError(t, hm.stream.Connect(ctx))

		assert.True(t, hm.isAvailable())
	})
}

func TestHedgeMarket_submitOrder_MaintenanceSkip(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	market := Market("BTCUSDT")
	depth := Number(100.0)

	order := types.SubmitOrder{
		Market:           market,
		Symbol:           market.Symbol,
		Quantity:         Number(1.0),
		Side:             types.SideTypeSell,
		Type:             types.OrderTypeMarket,
		MarginSideEffect: types.SideEffectTypeMarginBuy,
	}

	t.Run("in maintenance", func(t *testing.T) {
		session, _, _ := newMockSession(mockCtrl, ctx, market.Symbol)
		session.Maintenance = newMaintenanceWindow(time.Now())

		// mockExchange.SubmitOrder is intentionally not stubbed: any call to it
		// fails the test via gomock's unexpected-call assertion.

		hm := NewHedgeMarket(&HedgeMarketConfig{
			SymbolSelector: market.Symbol, HedgeInterval: hedgeInterval, QuotingDepth: depth,
		}, session, market)
		assert.NoError(t, hm.stream.Connect(ctx))

		_, err := hm.submitOrder(ctx, order)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "maintenance")
	})

	t.Run("disconnected", func(t *testing.T) {
		session, _, _ := newMockSession(mockCtrl, ctx, market.Symbol)
		session.Connectivity = newDisconnectedConnectivityGroup()

		hm := NewHedgeMarket(&HedgeMarketConfig{
			SymbolSelector: market.Symbol, HedgeInterval: hedgeInterval, QuotingDepth: depth,
		}, session, market)
		assert.NoError(t, hm.stream.Connect(ctx))

		_, err := hm.submitOrder(ctx, order)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "disconnected")
	})
}

func TestSplitHedge_HasAvailableMarket(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	market := Market("BTCUSDT")
	depth := Number(100.0)

	newMarket := func(down bool) *HedgeMarket {
		session, _, _ := newMockSession(mockCtrl, ctx, market.Symbol)
		if down {
			session.Maintenance = newMaintenanceWindow(time.Now())
		}
		hm := NewHedgeMarket(&HedgeMarketConfig{
			SymbolSelector: market.Symbol, HedgeInterval: hedgeInterval, QuotingDepth: depth,
		}, session, market)
		if err := hm.stream.Connect(ctx); err != nil {
			t.Fatalf("failed to connect hedge market stream: %v", err)
		}
		return hm
	}

	t.Run("all markets down", func(t *testing.T) {
		split := &SplitHedge{
			hedgeMarketInstances: map[string]*HedgeMarket{
				"m1": newMarket(true),
				"m2": newMarket(true),
			},
		}
		assert.False(t, split.HasAvailableMarket())
	})

	t.Run("one market up", func(t *testing.T) {
		split := &SplitHedge{
			hedgeMarketInstances: map[string]*HedgeMarket{
				"m1": newMarket(true),
				"m2": newMarket(false),
			},
		}
		assert.True(t, split.HasAvailableMarket())
	})
}
