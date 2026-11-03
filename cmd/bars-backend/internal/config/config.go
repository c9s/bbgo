// Package config parses the bars-backend command line flags into a Config.
package config

import (
	"flag"
	"strings"

	"github.com/c9s/bbgo/pkg/types"
)

// Config holds the runtime configuration for the bars-backend server.
type Config struct {
	ListenAddr   string
	ExchangeName types.ExchangeName
	Symbols      []string
	Intervals    []types.Interval
	CORSOrigin   string
	HistoryLimit int
}

// ParseFlags parses command line flags into a Config.
func ParseFlags() *Config {
	cfg := &Config{}

	var symbols, intervals, exchangeName string

	flag.StringVar(&cfg.ListenAddr, "listen", ":8090", "HTTP listen address")
	flag.StringVar(&symbols, "symbols", "BTCUSDT,ETHUSDT", "comma-separated list of symbols to track")
	flag.StringVar(&intervals, "intervals", "1m,5m,15m,1h,4h,1d", "comma-separated list of kline intervals to track")
	flag.StringVar(&cfg.CORSOrigin, "cors-origin", "http://localhost:3100", "allowed CORS origin for the frontend dev server")
	flag.IntVar(&cfg.HistoryLimit, "history-limit", 500, "number of historical bars/klines to backfill and keep in memory")
	flag.StringVar(&exchangeName, "exchange", "binance", "exchange to pull live market data from")
	flag.Parse()

	cfg.ExchangeName = types.ExchangeName(exchangeName)
	cfg.Symbols = splitAndTrim(symbols)

	for _, s := range splitAndTrim(intervals) {
		cfg.Intervals = append(cfg.Intervals, types.Interval(s))
	}

	return cfg
}

// HasSymbol reports whether the given symbol is one of the tracked symbols.
func (c *Config) HasSymbol(symbol string) bool {
	for _, s := range c.Symbols {
		if s == symbol {
			return true
		}
	}
	return false
}

// HasInterval reports whether the given interval is one of the tracked kline intervals.
func (c *Config) HasInterval(interval types.Interval) bool {
	for _, i := range c.Intervals {
		if i == interval {
			return true
		}
	}
	return false
}

func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
