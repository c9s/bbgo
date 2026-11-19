package xfixedmaker

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStrategy_UnmarshalProfitFixer(t *testing.T) {
	var s Strategy
	err := json.Unmarshal([]byte(`{
		"symbol": "BTCUSDT",
		"profitFixer": {"enabled": true, "apply": true, "tradesSince": "2026-11-01T00:00:00+08:00"}
	}`), &s)
	require.NoError(t, err)
	require.NotNil(t, s.ProfitFixerConfig)
	assert.True(t, s.ProfitFixerConfig.Enabled)
	assert.True(t, s.ProfitFixerConfig.Apply)
	assert.False(t, s.ProfitFixerConfig.TradesSince.Time().IsZero())
	assert.True(t, s.NeedsProfitFixing())

	// the same config was already applied, so it should not run again
	s.LastProfitFixConfig = s.ProfitFixerConfig
	assert.False(t, s.NeedsProfitFixing())
}
