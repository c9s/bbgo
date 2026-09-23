// Package registry builds market data sources from configuration.
//
// It is the one place that imports every provider, which is why it is a separate
// package: pkg/marketdata stays free of provider dependencies, so importing the
// event model does not drag in parquet, grpc and an HTTP client.
package registry

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/c9s/bbgo/pkg/marketdata"
	"github.com/c9s/bbgo/pkg/marketdata/sources/amberdata"
	"github.com/c9s/bbgo/pkg/marketdata/sources/binancecsv"
	"github.com/c9s/bbgo/pkg/marketdata/sources/grpcsource"
	"github.com/c9s/bbgo/pkg/marketdata/sources/replay"
)

// Options carries settings a configuration file should not have to repeat for
// every source.
type Options struct {
	// CacheDir is where archive providers cache downloads. A provider's own
	// params override it.
	CacheDir string
}

// builders maps a config type to its constructor. It is a map rather than a
// switch so Types() can list what is available in an error message.
var builders = map[string]func(context.Context, marketdata.SourceConfig, Options) (marketdata.Source, error){
	marketdata.SourceTypeBinanceCSV: buildBinanceCSV,
	marketdata.SourceTypeReplay:     buildReplay,
	marketdata.SourceTypeGRPC:       buildGRPC,
	marketdata.SourceTypeAmberData:  buildAmberData,
}

// Types returns the configurable source types, sorted.
func Types() []string {
	out := make([]string, 0, len(builders))
	for name := range builders {
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}

// New builds one source from its configuration.
func New(ctx context.Context, cfg marketdata.SourceConfig, opts Options) (marketdata.Source, error) {
	build, ok := builders[cfg.Type]
	if !ok {
		return nil, fmt.Errorf("marketdata: unknown source type %q, known types are %s",
			cfg.Type, strings.Join(Types(), ", "))
	}

	src, err := build(ctx, cfg, opts)
	if err != nil {
		return nil, fmt.Errorf("marketdata: building source %s: %w", cfg.Describe(), err)
	}

	return src, nil
}

// NewAll builds every configured source, closing whatever it already built if
// one fails.
func NewAll(
	ctx context.Context, cfgs []marketdata.SourceConfig, opts Options,
) ([]marketdata.Source, error) {
	var sources []marketdata.Source

	for _, cfg := range cfgs {
		src, err := New(ctx, cfg, opts)
		if err != nil {
			CloseAll(sources)
			return nil, err
		}
		sources = append(sources, src)
	}

	return sources, nil
}

// OpenMerged builds every configured source and returns a single time-ordered
// cursor over them. This is the entry point a consumer wants.
func OpenMerged(
	ctx context.Context,
	cfgs []marketdata.SourceConfig,
	req marketdata.Request,
	opts Options,
	mergeOpts ...marketdata.MergeOption,
) (*marketdata.MergeCursor, error) {
	sources, err := NewAll(ctx, cfgs, opts)
	if err != nil {
		return nil, err
	}

	merged, err := marketdata.MergeSources(ctx, sources, req, mergeOpts...)
	if err != nil {
		CloseAll(sources)
		return nil, err
	}

	return merged, nil
}

// CloseAll closes the sources that hold resources. Most do not; the gRPC source
// holds a connection.
func CloseAll(sources []marketdata.Source) {
	for _, src := range sources {
		if closer, ok := src.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
	}
}

func buildBinanceCSV(
	_ context.Context, cfg marketdata.SourceConfig, opts Options,
) (marketdata.Source, error) {
	var params binancecsv.Config
	if err := cfg.DecodeParams(&params); err != nil {
		return nil, err
	}

	params.Name = cfg.Name
	if params.CacheDir == "" {
		params.CacheDir = opts.CacheDir
	}

	return binancecsv.New(params)
}

func buildReplay(
	_ context.Context, cfg marketdata.SourceConfig, opts Options,
) (marketdata.Source, error) {
	var params replay.Config
	if err := cfg.DecodeParams(&params); err != nil {
		return nil, err
	}

	params.Name = cfg.Name
	return replay.New(params)
}

func buildGRPC(
	ctx context.Context, cfg marketdata.SourceConfig, opts Options,
) (marketdata.Source, error) {
	var params grpcsource.Config
	if err := cfg.DecodeParams(&params); err != nil {
		return nil, err
	}

	params.Name = cfg.Name
	return grpcsource.New(ctx, params)
}

func buildAmberData(
	_ context.Context, cfg marketdata.SourceConfig, opts Options,
) (marketdata.Source, error) {
	var params amberdata.Config
	if err := cfg.DecodeParams(&params); err != nil {
		return nil, err
	}

	params.Name = cfg.Name
	return amberdata.New(params)
}
