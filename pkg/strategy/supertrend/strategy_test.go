package supertrend

import (
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/mocks"
)

func newTestSession(t *testing.T) *bbgo.ExchangeSession {
	t.Helper()

	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockEx := mocks.NewMockExchange(mockCtrl)
	mockEx.EXPECT().NewStream().Return(&types.StandardStream{}).AnyTimes()
	mockEx.EXPECT().Name().Return(types.ExchangeName("backtest")).AnyTimes()

	return bbgo.NewExchangeSession("test", mockEx)
}

// A linearRegression block with window: 0 (or no interval) is nil-ed by
// setupIndicators, but Subscribe used to dereference it unconditionally and
// session.Subscribe panics on an empty kline interval.
func TestStrategySubscribe_LinearRegressionNilSafe(t *testing.T) {
	cases := []struct {
		name   string
		linReg *LinReg
		subs   int
	}{
		{name: "nil block", linReg: nil, subs: 1},
		{
			name:   "window zero without interval",
			linReg: &LinReg{IntervalWindow: types.IntervalWindow{Window: 0}},
			subs:   1,
		},
		{
			name: "valid block",
			linReg: &LinReg{
				IntervalWindow: types.IntervalWindow{Window: 20, Interval: types.Interval4h},
			},
			subs: 2,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			session := newTestSession(t)
			s := &Strategy{
				Symbol:           "BTCUSDT",
				IntervalWindow:   types.IntervalWindow{Interval: types.Interval1h, Window: 5},
				LinearRegression: tc.linReg,
			}

			s.Subscribe(session)

			if got := len(session.Subscriptions); got != tc.subs {
				t.Fatalf("subscriptions = %d, want %d", got, tc.subs)
			}
		})
	}
}
