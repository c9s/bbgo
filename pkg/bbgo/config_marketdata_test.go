package bbgo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/marketdata"
)

// TestLoadConfig_DataSources checks that the market data sources round-trip
// through the same yaml loader everything else in bbgo uses.
func TestLoadConfig_DataSources(t *testing.T) {
	config, err := Load("../../config/marketdata.yaml", false)
	require.NoError(t, err)
	require.NotNil(t, config.Backtest)

	assert.Equal(t, "~/.bbgo/marketdata", config.Backtest.CacheDir)
	require.Len(t, config.Backtest.DataSources, 2,
		"the commented-out sources must not be parsed")

	archives := config.Backtest.DataSources[0]
	assert.Equal(t, marketdata.SourceTypeBinanceCSV, archives.Type)
	assert.Equal(t, "binance-archives", archives.Name)

	// The params are provider-specific, so they arrive as a map and are decoded
	// by the provider rather than by this struct.
	var params struct {
		Market           string   `json:"market"`
		Period           string   `json:"period"`
		Datasets         []string `json:"datasets"`
		Symbols          []string `json:"symbols"`
		Intervals        []string `json:"intervals"`
		AllowMissingDays bool     `json:"allowMissingDays"`
	}
	require.NoError(t, archives.DecodeParams(&params))

	assert.Equal(t, "um", params.Market)
	assert.Equal(t, "daily", params.Period)
	assert.Equal(t, []string{"aggTrades", "klines"}, params.Datasets)
	assert.Equal(t, []string{"BTCUSDT"}, params.Symbols)
	assert.Equal(t, []string{"1h"}, params.Intervals)
	assert.False(t, params.AllowMissingDays)

	recording := config.Backtest.DataSources[1]
	assert.Equal(t, marketdata.SourceTypeReplay, recording.Type)
	assert.Equal(t, "./recordings", recording.Params["path"])
}

// TestLoadConfig_DataSourcesOptional guards the existing configs: a backtest
// section without dataSources must keep loading.
func TestLoadConfig_DataSourcesOptional(t *testing.T) {
	config, err := Load("testdata/backtest.yaml", false)
	require.NoError(t, err)

	if config.Backtest != nil {
		assert.Empty(t, config.Backtest.DataSources)
	}
}
