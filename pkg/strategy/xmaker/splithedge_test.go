//go:build !dnum

package xmaker

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/tradeid"
	"github.com/c9s/bbgo/pkg/types"
)

func init() {
	tradeid.GlobalGenerator = tradeid.NewDeterministicGenerator()
}

// TestSplitHedge_HedgeWithProportionAlgo verifies that hedgeWithProportionAlgo:
// - splits the hedge quantity according to ratios
// - covers the strategy position exposure with the dispatched deltas
// - forwards the cover deltas to each hedgeMarket's positionDeltaC channel
func TestSplitHedge_HedgeWithProportionAlgo(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	market := Market("BTCUSDT")

	// create two mocked hedge market sessions
	session1, md1, _ := newMockSession(mockCtrl, ctx, market.Symbol)
	session2, md2, _ := newMockSession(mockCtrl, ctx, market.Symbol)

	depth := Number(100.0)
	hm1 := NewHedgeMarket(&HedgeMarketConfig{
		SymbolSelector: market.Symbol,
		HedgeInterval:  hedgeInterval,
		QuotingDepth:   depth,
	}, session1, market)
	assert.NoError(t, hm1.stream.Connect(ctx))

	hm2 := NewHedgeMarket(&HedgeMarketConfig{
		SymbolSelector: market.Symbol,
		HedgeInterval:  hedgeInterval,
		QuotingDepth:   depth,
	}, session2, market)
	assert.NoError(t, hm2.stream.Connect(ctx))

	// provide order book snapshots so getQuotePrice() can work
	orderBook := types.SliceOrderBook{
		Symbol: market.Symbol,
		Bids:   types.PriceVolumeSlice{{Price: Number(10000), Volume: Number(100)}},
		Asks:   types.PriceVolumeSlice{{Price: Number(10010), Volume: Number(100)}},
	}
	md1.EmitBookSnapshot(orderBook)
	md2.EmitBookSnapshot(orderBook)

	// create a minimal strategy with position exposure and maker market
	strategy := &Strategy{}
	strategy.positionExposure = NewPositionExposure(market.Symbol)
	strategy.makerMarket = market
	strategy.logger = logrus.New()

	// build split hedge with proportion algo
	split := &SplitHedge{
		Enabled: true,
		Algo:    SplitHedgeAlgoProportion,
		ProportionAlgo: &SplitHedgeProportionAlgo{
			ProportionMarkets: []*SplitHedgeProportionMarket{
				{Name: "m1", Ratio: Number(0.5)}, // 50%
				{Name: "m2"},                     // remaining
			},
		},
		hedgeMarketInstances: map[string]*HedgeMarket{
			"m1": hm1,
			"m2": hm2,
		},
		strategy: strategy,
		logger:   logrus.New(),
	}

	// uncovered +2 means we need hedgeDelta -2 (sell 2)
	uncovered := Number(2.0)
	hedgeDelta := uncovered.Neg()

	err := split.hedgeWithProportionAlgo(ctx, uncovered, hedgeDelta)
	assert.NoError(t, err)

	// The strategy pending exposure should be covered by +2 (sum of dispatched deltas)
	assert.Equal(t, Number(2.0), strategy.positionExposure.pending.Get())

	// Expect two dispatched cover deltas: +1 to each hedge market (sell side -> cover +quantity)
	expectCover := Number(1.0)

	// give a tiny time for channel writes to complete (channels are buffered, but be safe)
	time.Sleep(stepTime)

	// ensure both hedge markets received the correct cover delta
	select {
	case d := <-hm1.positionDeltaC:
		assert.Equal(t, expectCover, d, "hm1 cover delta")
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("timeout waiting for hm1 position delta")
	}

	select {
	case d := <-hm2.positionDeltaC:
		assert.Equal(t, expectCover, d, "hm2 cover delta")
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("timeout waiting for hm2 position delta")
	}
}
