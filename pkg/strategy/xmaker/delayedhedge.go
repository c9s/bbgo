package xmaker

import (
	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
)

type DelayedHedge struct {
	// EnableDelayHedge enables the delay hedge feature
	Enabled bool `json:"enabled"`

	// MaxDelayDuration is the maximum delay duration to hedge the position
	MaxDelayDuration types.Duration `json:"maxDelay"`

	// FixedDelayDuration is the fixed delay duration
	FixedDelayDuration types.Duration `json:"fixedDelay"`

	// SignalThreshold is the signal threshold to trigger the delay hedge
	SignalThreshold float64 `json:"signalThreshold"`

	// DynamicDelayScale is the dynamic delay scale
	DynamicDelayScale *bbgo.SlideRule `json:"dynamicDelayScale,omitempty"`
}
