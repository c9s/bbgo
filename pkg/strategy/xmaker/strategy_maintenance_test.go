package xmaker

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/bbgo"
	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/types"
)

func TestStrategy_updateQuote_MaintenanceSkip(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	symbol := "BTCUSDT"
	market := Market(symbol)

	hedgeSession := &bbgo.ExchangeSession{}
	s := &Strategy{
		Symbol:            symbol,
		hedgeSession:      hedgeSession,
		makerSession:      hedgeSession,
		makerMarket:       market,
		logger:            logrus.New(),
		activeMakerOrders: bbgo.NewActiveOrderBook(symbol),
		cancelOrderDurationMetrics: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name: "test_cancel_order_duration",
		}),
	}

	dummyConn := types.NewConnectivity()
	dummyStream := &types.StandardStream{}
	dummyConn.Bind(dummyStream)
	dummyStream.EmitConnect()

	hedgeSession.Connectivity = types.NewConnectivityGroup(dummyConn)

	now := time.Now()
	past := types.LooseFormatTime(now.Add(-10 * time.Minute))
	future := types.LooseFormatTime(now.Add(10 * time.Minute))
	hedgeSession.Maintenance = &bbgo.MaintenanceConfig{
		StartTime: &past,
		EndTime:   &future,
	}

	err := s.updateQuote(ctx)
	assert.NoError(t, err)
}

func TestStrategy_updateQuote_SplitHedge_NoAvailableMarket(t *testing.T) {
	ctx := context.Background()

	s := &Strategy{
		logger: logrus.New(),
		SplitHedge: &SplitHedge{
			Enabled: true,
			hedgeMarketInstances: map[string]*HedgeMarket{
				"m1": {session: &bbgo.ExchangeSession{Maintenance: newMaintenanceWindow(time.Now())}},
				"m2": {session: &bbgo.ExchangeSession{Maintenance: newMaintenanceWindow(time.Now())}},
			},
		},
	}

	err := s.updateQuote(ctx)
	assert.NoError(t, err)
}

func TestStrategy_updateQuote_SyntheticHedge_NoAvailableMarket(t *testing.T) {
	ctx := context.Background()

	s := &Strategy{
		logger: logrus.New(),
		SyntheticHedge: &SyntheticHedge{
			Enabled:      true,
			sourceMarket: &HedgeMarket{session: &bbgo.ExchangeSession{Maintenance: newMaintenanceWindow(time.Now())}},
			fiatMarket:   &HedgeMarket{session: &bbgo.ExchangeSession{}},
		},
	}

	err := s.updateQuote(ctx)
	assert.NoError(t, err)
}

func TestStrategy_directHedge_MaintenanceSkip(t *testing.T) {
	ctx := context.Background()

	t.Run("in maintenance", func(t *testing.T) {
		hedgeSession := &bbgo.ExchangeSession{}
		hedgeSession.Connectivity = types.NewConnectivityGroup(types.NewConnectivity())

		now := time.Now()
		past := types.LooseFormatTime(now.Add(-10 * time.Minute))
		future := types.LooseFormatTime(now.Add(10 * time.Minute))
		hedgeSession.Maintenance = &bbgo.MaintenanceConfig{
			StartTime: &past,
			EndTime:   &future,
		}

		s := &Strategy{hedgeSession: hedgeSession}

		order, err := s.directHedge(ctx, Number(1.0), Number(-1.0))
		assert.Nil(t, order)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "maintenance")
	})

	t.Run("disconnected", func(t *testing.T) {
		hedgeSession := &bbgo.ExchangeSession{}

		dummyConn := types.NewConnectivity()
		dummyStream := &types.StandardStream{}
		dummyConn.Bind(dummyStream)
		// not connected: no EmitConnect() call

		hedgeSession.Connectivity = types.NewConnectivityGroup(dummyConn)

		s := &Strategy{hedgeSession: hedgeSession}

		order, err := s.directHedge(ctx, Number(1.0), Number(-1.0))
		assert.Nil(t, order)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "disconnected")
	})
}
