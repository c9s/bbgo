package xmaker

import (
	"context"
	"errors"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/mocks"
)

// guardExchange wraps a mock exchange and optionally implements the optional
// CheckMarginAccount guard that the okex exchange provides. When checkErr is
// nil the guard succeeds; the call count proves whether it was ever invoked.
type guardExchange struct {
	*mocks.MockExchange
	checkErr  error
	callCount int
}

func (g *guardExchange) CheckMarginAccount(_ context.Context) error {
	g.callCount++
	return g.checkErr
}

func newMockExchangeWithStream(t *testing.T) *mocks.MockExchange {
	t.Helper()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)
	ex := mocks.NewMockExchange(ctrl)
	ex.EXPECT().NewStream().Return(&types.StandardStream{}).AnyTimes()
	ex.EXPECT().Name().Return(types.ExchangeName("okex")).AnyTimes()
	return ex
}

// newHedgeSessionWithGuard builds a hedge session whose exchange implements
// CheckMarginAccount and returns (session, guard) so the test can inspect the
// guard's call count.
func newHedgeSessionWithGuard(t *testing.T, checkErr error) (*bbgo.ExchangeSession, *guardExchange) {
	t.Helper()
	inner := newMockExchangeWithStream(t)
	guard := &guardExchange{MockExchange: inner, checkErr: checkErr}
	return bbgo.NewExchangeSession("okex", guard), guard
}

// newHedgeSessionNoGuard builds a hedge session whose exchange does NOT
// implement CheckMarginAccount (e.g. binance / max), so the probe is a no-op.
func newHedgeSessionNoGuard(t *testing.T) *bbgo.ExchangeSession {
	t.Helper()
	return bbgo.NewExchangeSession("okex", newMockExchangeWithStream(t))
}

// crossRunSessions returns a sessions map wired for the cross-run setup: an
// okex hedge session and a (bare) maker session.
func crossRunSessions(hedge *bbgo.ExchangeSession) map[string]*bbgo.ExchangeSession {
	return map[string]*bbgo.ExchangeSession{
		"okex": hedge,
		"max":  &bbgo.ExchangeSession{},
	}
}

func crossRunStrategy(hedge *bbgo.ExchangeSession) *Strategy {
	return &Strategy{
		StrategyConfig: StrategyConfig{
			SourceSymbol:   "BTCUSDT",
			SourceExchange: "okex",
			MakerExchange:  "max",
		},
		Symbol: "BTCUSDT",
		logger: logrus.New(),
	}
}

// marker that CrossRun passed the account guard and continued to the (missing)
// hedge market lookup.
const marketNotDefined = "source session market BTCUSDT is not defined"

// Ticket 06: a misconfigured OKX hedge account blocks only this strategy
// instance (with a descriptive error), while a good account, an exchange
// without the guard, and a margin-disabled session all proceed past the guard.
func TestStrategy_CrossRun_MarginAccountGuard(t *testing.T) {
	t.Run("guard fails: blocks only this instance with the guard's error", func(t *testing.T) {
		guardErr := errors.New("okex margin session requires a multi-currency margin account (acctLv >= 3), got acctLv=1")
		hedge, guard := newHedgeSessionWithGuard(t, guardErr)
		hedge.Margin = true

		s := crossRunStrategy(hedge)
		err := s.CrossRun(context.Background(), nil, crossRunSessions(hedge))

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "acctLv", "the guard's descriptive error must surface")
		assert.NotContains(t, err.Error(), "not defined", "must stop at the guard, not the market lookup")
		assert.Equal(t, 1, guard.callCount, "the guard must be invoked exactly once")
	})

	t.Run("guard succeeds: proceeds past the guard to the market lookup", func(t *testing.T) {
		hedge, guard := newHedgeSessionWithGuard(t, nil)
		hedge.Margin = true

		s := crossRunStrategy(hedge)
		err := s.CrossRun(context.Background(), nil, crossRunSessions(hedge))

		// no market configured on the hedge session, so a successful guard lets
		// CrossRun advance to the first market lookup and fail there
		assert.Error(t, err)
		assert.Contains(t, err.Error(), marketNotDefined, "a good account must proceed past the guard")
		assert.NotContains(t, err.Error(), "acctLv")
		assert.Equal(t, 1, guard.callCount)
	})

	t.Run("exchange without the guard: probe is a no-op and proceeds", func(t *testing.T) {
		hedge := newHedgeSessionNoGuard(t)
		hedge.Margin = true

		s := crossRunStrategy(hedge)
		err := s.CrossRun(context.Background(), nil, crossRunSessions(hedge))

		assert.Error(t, err)
		assert.Contains(t, err.Error(), marketNotDefined, "a non-guarding exchange must proceed unchanged")
	})

	t.Run("margin disabled: guard is never invoked", func(t *testing.T) {
		hedge, guard := newHedgeSessionWithGuard(t, errors.New("should not be reached"))
		hedge.Margin = false

		s := crossRunStrategy(hedge)
		err := s.CrossRun(context.Background(), nil, crossRunSessions(hedge))

		// even a failing guard must not fire when margin is off
		assert.Error(t, err)
		assert.Contains(t, err.Error(), marketNotDefined)
		assert.Equal(t, 0, guard.callCount, "the guard must not run when margin is disabled")
	})
}
