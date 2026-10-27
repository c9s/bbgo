package xmaker

import (
	"context"
	"regexp"
	"strconv"
	"testing"

	"github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/pricesolver"
	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/types"
)

// newConnectedSession builds a minimal, already-connected ExchangeSession
// seeded with the given balances, for driving updateQuote in tests without
// hitting any real exchange.
func newConnectedSession(balances types.BalanceMap) *bbgo.ExchangeSession {
	account := types.NewAccount()
	account.UpdateBalances(balances)

	session := &bbgo.ExchangeSession{Account: account}

	conn := types.NewConnectivity()
	stream := &types.StandardStream{}
	conn.Bind(stream)
	stream.EmitConnect()
	session.Connectivity = types.NewConnectivityGroup(conn)

	return session
}

var dryRunOrderLineRegexp = regexp.MustCompile(`^\[dryRun\] (\d+): price=[0-9.]+, quantity=([0-9.]+),`)

// countDryRunOrderLines parses the "[dryRun] <layer index>: price=..., quantity=..." log lines
// emitted by updateQuote under DryRun, grouped by side, and returns the quantities in layer order.
func dryRunOrderQuantities(entries []*logrus.Entry, sideHeader string) []float64 {
	var quantities []float64
	inSide := false
	for _, entry := range entries {
		switch entry.Message {
		case "[dryRun] maker buy orders:", "[dryRun] maker sell orders:":
			inSide = entry.Message == sideHeader
			continue
		}

		if !inSide {
			continue
		}

		m := dryRunOrderLineRegexp.FindStringSubmatch(entry.Message)
		if m == nil {
			continue
		}

		var q float64
		q, err := strconv.ParseFloat(m[2], 64)
		if err != nil {
			continue
		}
		quantities = append(quantities, q)
	}
	return quantities
}

func TestStrategy_updateQuote_SkipRemainingLayersWhenMakerBalanceInsufficient(t *testing.T) {
	ctx := context.Background()
	symbol := "BTCUSDT"
	market := Market(symbol)

	// layer 0 needs 0.01 BTC / 200 USDT, layer 1 needs 0.02 BTC / 400 USDT (multiplier=2)
	makerSession := newConnectedSession(types.BalanceMap{
		"USDT": types.NewBalance("USDT", Number(250)),  // enough for layer 0 (200) but not layer 0+1 (200+400)
		"BTC":  types.NewBalance("BTC", Number(0.015)), // enough for layer 0 (0.01) but not layer 0+1 (0.01+0.02)
	})
	hedgeSession := newConnectedSession(types.BalanceMap{
		"USDT": types.NewBalance("USDT", Number(200000)),
		"BTC":  types.NewBalance("BTC", Number(100)),
	})

	sourceBook := types.NewStreamBook(symbol, "")
	sourceBook.Load(types.SliceOrderBook{
		Symbol: symbol,
		Bids:   types.PriceVolumeSlice{{Price: Number(20000), Volume: Number(100)}},
		Asks:   types.PriceVolumeSlice{{Price: Number(20010), Volume: Number(100)}},
	})

	logger, hook := logrustest.NewNullLogger()
	logger.SetLevel(logrus.InfoLevel)

	s := &Strategy{
		Symbol: symbol,
		StrategyConfig: StrategyConfig{
			DryRun:             true,
			NumLayers:          3,
			Quantity:           Number(0.01),
			QuantityMultiplier: Number(2),
		},
		makerSession:      makerSession,
		hedgeSession:      hedgeSession,
		makerMarket:       market,
		hedgeMarket:       market,
		activeMakerOrders: bbgo.NewActiveOrderBook(symbol),
		priceSolver:       pricesolver.NewSimplePriceResolver(types.MarketMap{symbol: market}),
		sourceBook:        sourceBook,
	}

	require.NoError(t, s.Initialize())
	s.logger = logger

	require.NoError(t, s.updateQuote(ctx))

	entries := hook.AllEntries()

	bidQuantities := dryRunOrderQuantities(entries, "[dryRun] maker buy orders:")
	askQuantities := dryRunOrderQuantities(entries, "[dryRun] maker sell orders:")

	// only layer 0 should be placed on each side, at the full (unshrunk) desired quantity
	assert.Equal(t, []float64{0.01}, bidQuantities, "bid: only layer 0 should be quoted, at full quantity")
	assert.Equal(t, []float64{0.01}, askQuantities, "ask: only layer 0 should be quoted, at full quantity")
}
