package xmaker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/bbgo"
)

func TestSyntheticHedge_IsAvailable(t *testing.T) {
	newSession := func(down bool) *bbgo.ExchangeSession {
		session := &bbgo.ExchangeSession{}
		if down {
			session.Maintenance = newMaintenanceWindow(time.Now())
		}
		return session
	}

	newHedgeMarket := func(down bool) *HedgeMarket {
		return &HedgeMarket{session: newSession(down)}
	}

	t.Run("source down, fiat up", func(t *testing.T) {
		s := &SyntheticHedge{
			sourceMarket: newHedgeMarket(true),
			fiatMarket:   newHedgeMarket(false),
		}
		assert.False(t, s.IsAvailable())
	})

	t.Run("source up, fiat down", func(t *testing.T) {
		s := &SyntheticHedge{
			sourceMarket: newHedgeMarket(false),
			fiatMarket:   newHedgeMarket(true),
		}
		assert.False(t, s.IsAvailable())
	})

	t.Run("both up", func(t *testing.T) {
		s := &SyntheticHedge{
			sourceMarket: newHedgeMarket(false),
			fiatMarket:   newHedgeMarket(false),
		}
		assert.True(t, s.IsAvailable())
	})

	t.Run("split source: one source market up, fiat up", func(t *testing.T) {
		split := &SplitHedge{
			Enabled: true,
			hedgeMarketInstances: map[string]*HedgeMarket{
				"m1": newHedgeMarket(true),
				"m2": newHedgeMarket(false),
			},
		}
		s := &SyntheticHedge{
			SplitSource: split,
			fiatMarket:  newHedgeMarket(false),
		}
		assert.True(t, s.IsAvailable())
	})

	t.Run("split source: all source markets down", func(t *testing.T) {
		split := &SplitHedge{
			Enabled: true,
			hedgeMarketInstances: map[string]*HedgeMarket{
				"m1": newHedgeMarket(true),
				"m2": newHedgeMarket(true),
			},
		}
		s := &SyntheticHedge{
			SplitSource: split,
			fiatMarket:  newHedgeMarket(false),
		}
		assert.False(t, s.IsAvailable())
	})
}
