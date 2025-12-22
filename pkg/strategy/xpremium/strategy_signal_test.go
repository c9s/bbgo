package xpremium

import (
	"context"
	"testing"

	"github.com/c9s/bbgo/pkg/bbgo"
	makersignal "github.com/c9s/bbgo/pkg/strategy/xmaker/signal"
	"github.com/c9s/bbgo/pkg/types"
)

type staticSignal struct {
	id string
	v  float64
}

func (s *staticSignal) ID() string                                                      { return s.id }
func (s *staticSignal) Subscribe(_ *bbgo.ExchangeSession, _ string)                     {}
func (s *staticSignal) Bind(_ context.Context, _ *bbgo.ExchangeSession, _ string) error { return nil }
func (s *staticSignal) CalculateSignal(_ context.Context) (float64, error)              { return s.v, nil }

func TestAggregateExternalSignal(t *testing.T) {
	st := &Strategy{}
	st.SignalConfigList = &makersignal.DynamicConfig{
		Signals: []makersignal.ProviderWrapper{
			{Weight: 1.0, Signal: &staticSignal{id: "a", v: 1.5}},
			{Weight: 3.0, Signal: &staticSignal{id: "b", v: -0.5}},
		},
	}
	sig, ok := st.aggregateExternalSignal(context.Background())
	if !ok {
		t.Fatalf("expected ok=true")
	}
	// weighted avg = (1.5*1 + -0.5*3) / (|1|+|3|) = (1.5 - 1.5)/4 = 0
	if sig != 0 {
		t.Fatalf("unexpected signal value: %v", sig)
	}
}

func TestSignalToSide(t *testing.T) {
	if s := signalToSide(0.1); s != types.SideTypeBuy {
		t.Fatalf("expected buy side for positive signal, got %v", s)
	}
	if s := signalToSide(-0.1); s != types.SideTypeSell {
		t.Fatalf("expected sell side for negative signal, got %v", s)
	}
	if s := signalToSide(0.0); s != types.SideType("") {
		t.Fatalf("expected empty side for zero signal, got %v", s)
	}
}
