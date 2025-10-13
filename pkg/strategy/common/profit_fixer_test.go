package common

import (
	"testing"
	"time"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestProfitFixerConfigEqual(t *testing.T) {
	t.Run(
		"BothNil", func(t *testing.T) {
			var c1, c2 *ProfitFixerConfig
			assert.True(t, c1.Equal(c2))
		},
	)
	t.Run(
		"OneNil", func(t *testing.T) {
			var c1 *ProfitFixerConfig
			c2 := &ProfitFixerConfig{}
			assert.False(t, c1.Equal(c2))
		},
	)
	t.Run(
		"BothEmpty", func(t *testing.T) {
			c1 := &ProfitFixerConfig{}
			c2 := &ProfitFixerConfig{}
			assert.True(t, c1.Equal(c2))
		},
	)
	t.Run(
		"DifferentTradeSince", func(t *testing.T) {
			c1 := &ProfitFixerConfig{TradesSince: types.Time(time.Now())}
			c2 := &ProfitFixerConfig{TradesSince: types.Time(time.Now().Add(time.Hour))}
			assert.False(t, c1.Equal(c2))
		},
	)
	t.Run(
		"SameTradeSince", func(t *testing.T) {
			now := time.Now()
			c1 := &ProfitFixerConfig{TradesSince: types.Time(now)}
			c2 := &ProfitFixerConfig{TradesSince: types.Time(now)}
			assert.True(t, c1.Equal(c2))
		},
	)
	t.Run(
		"DifferentPatch", func(t *testing.T) {
			c1 := &ProfitFixerConfig{Patch: "a"}
			c2 := &ProfitFixerConfig{Patch: "b"}
			assert.False(t, c1.Equal(c2))
		},
	)
	t.Run(
		"SamePatch", func(t *testing.T) {
			c1 := &ProfitFixerConfig{Patch: "a"}
			c2 := &ProfitFixerConfig{Patch: "a"}
			assert.True(t, c1.Equal(c2))
		},
	)
	t.Run(
		"AllSame", func(t *testing.T) {
			now := time.Now()
			c1 := &ProfitFixerConfig{TradesSince: types.Time(now), Patch: "a"}
			c2 := &ProfitFixerConfig{TradesSince: types.Time(now), Patch: "a"}
			assert.True(t, c1.Equal(c2))
		},
	)
}
