package registry_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/c9s/bbgo/pkg/marketdata"
	"github.com/c9s/bbgo/pkg/marketdata/registry"
	"github.com/c9s/bbgo/pkg/types"
)

// TestYAMLRoundTrip pins the configuration shape, which is the part users see.
func TestYAMLRoundTrip(t *testing.T) {
	const doc = `
- type: binanceCsv
  name: binance-trades
  params:
    market: um
    datasets: [aggTrades, klines]
    symbols: [BTCUSDT]
    intervals: [1h]
    cacheDir: /tmp/md
    allowMissingDays: true
- type: replay
  name: recorded-book
  params:
    path: ./recordings
- type: amberdata
  params:
    exchange: binance
    assetClass: futures
    maxLevel: 20
`

	var cfgs []marketdata.SourceConfig
	require.NoError(t, yaml.Unmarshal([]byte(doc), &cfgs))
	require.Len(t, cfgs, 3)

	assert.Equal(t, marketdata.SourceTypeBinanceCSV, cfgs[0].Type)
	assert.Equal(t, "binance-trades", cfgs[0].Name)
	assert.Equal(t, "um", cfgs[0].Params["market"])
	assert.Equal(t, true, cfgs[0].Params["allowMissingDays"])

	assert.Equal(t, "recorded-book", cfgs[1].Name)
	assert.Equal(t, "./recordings", cfgs[1].Params["path"])

	// An unnamed source still describes itself by type.
	assert.Equal(t, marketdata.SourceTypeAmberData, cfgs[2].Describe())
}

// TestDecodeParams checks the json round-trip that turns a parsed yaml map into
// a provider's own config struct, which is how pkg/bbgo already loads strategies.
func TestDecodeParams(t *testing.T) {
	cfg := marketdata.SourceConfig{
		Type: marketdata.SourceTypeBinanceCSV,
		Params: map[string]any{
			"market":   "um",
			"datasets": []any{"aggTrades"},
			"maxLevel": 20,
		},
	}

	var out struct {
		Market   string   `json:"market"`
		Datasets []string `json:"datasets"`
		MaxLevel int      `json:"maxLevel"`
	}
	require.NoError(t, cfg.DecodeParams(&out))

	assert.Equal(t, "um", out.Market)
	assert.Equal(t, []string{"aggTrades"}, out.Datasets)
	assert.Equal(t, 20, out.MaxLevel)
}

func TestDecodeParams_Empty(t *testing.T) {
	var out struct{ Market string }
	assert.NoError(t, marketdata.SourceConfig{}.DecodeParams(&out))
}

func TestDecodeParams_TypeMismatch(t *testing.T) {
	cfg := marketdata.SourceConfig{
		Type:   marketdata.SourceTypeBinanceCSV,
		Name:   "bad",
		Params: map[string]any{"market": 42},
	}

	var out struct {
		Market string `json:"market"`
	}
	err := cfg.DecodeParams(&out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad (binanceCsv)",
		"the error must name which source is misconfigured")
}

// TestNew_UnknownTypeListsKnownTypes makes a typo self-correcting rather than
// leaving the user to grep the source for valid values.
func TestNew_UnknownTypeListsKnownTypes(t *testing.T) {
	_, err := registry.New(context.Background(),
		marketdata.SourceConfig{Type: "binance-csv"}, registry.Options{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown source type "binance-csv"`)
	for _, known := range registry.Types() {
		assert.Contains(t, err.Error(), known)
	}
}

func TestTypes(t *testing.T) {
	assert.Equal(t, []string{"amberdata", "binanceCsv", "grpc", "replay"}, registry.Types())
}

func TestNew_BinanceCSV(t *testing.T) {
	src, err := registry.New(context.Background(), marketdata.SourceConfig{
		Type: marketdata.SourceTypeBinanceCSV,
		Name: "archives",
		Params: map[string]any{
			"market":   "um",
			"datasets": []any{"aggTrades"},
		},
	}, registry.Options{CacheDir: t.TempDir()})

	require.NoError(t, err)
	assert.Equal(t, "archives", src.Name(), "the config name must override the default")
	assert.Contains(t, src.Capabilities().Channels, types.AggTradeChannel)
	assert.NotContains(t, src.Capabilities().Channels, types.BookChannel,
		"the archives cannot serve L2, whatever the configuration says")
}

// TestNew_CacheDirFallback checks the shared option: a config should not have to
// repeat the cache directory for every archive source.
func TestNew_CacheDirFallback(t *testing.T) {
	shared := t.TempDir()

	_, err := registry.New(context.Background(), marketdata.SourceConfig{
		Type:   marketdata.SourceTypeBinanceCSV,
		Params: map[string]any{"market": "um", "datasets": []any{"aggTrades"}},
	}, registry.Options{CacheDir: shared})
	require.NoError(t, err, "the shared cacheDir must satisfy the provider's requirement")

	_, err = registry.New(context.Background(), marketdata.SourceConfig{
		Type:   marketdata.SourceTypeBinanceCSV,
		Params: map[string]any{"market": "um", "datasets": []any{"aggTrades"}},
	}, registry.Options{})
	require.Error(t, err, "without one, the provider must say so")
	assert.Contains(t, err.Error(), "cacheDir")
}

func TestNew_Replay(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.jsonl"),
		[]byte(`{"v":1,"exchange":"binance","symbols":["BTCUSDT"],"startedAt":0}`+"\n"), 0o644))

	src, err := registry.New(context.Background(), marketdata.SourceConfig{
		Type:   marketdata.SourceTypeReplay,
		Name:   "rec",
		Params: map[string]any{"path": dir},
	}, registry.Options{})

	require.NoError(t, err)
	assert.Equal(t, "rec", src.Name())
	assert.Contains(t, src.Capabilities().Channels, types.BookChannel,
		"a recording is what can serve L2")
}

func TestNew_AmberDataRequiresExchange(t *testing.T) {
	_, err := registry.New(context.Background(), marketdata.SourceConfig{
		Type:   marketdata.SourceTypeAmberData,
		Params: map[string]any{"apiKey": "UATx"},
	}, registry.Options{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "exchange is required")
}

// TestNewAll_ClosesOnFailure guards against leaking a connection when a later
// source in the list fails to build.
func TestNewAll_ClosesOnFailure(t *testing.T) {
	_, err := registry.NewAll(context.Background(), []marketdata.SourceConfig{
		{
			Type:   marketdata.SourceTypeBinanceCSV,
			Params: map[string]any{"market": "um", "datasets": []any{"aggTrades"}},
		},
		{Type: "nonsense"},
	}, registry.Options{CacheDir: t.TempDir()})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown source type")
}
