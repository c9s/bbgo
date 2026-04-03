# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

BBGO is a Go-based cryptocurrency trading framework supporting 8 exchanges (Binance, OKEx, KuCoin, MAX, Bitget, Bybit, Coinbase, Bitfinex) with 40+ built-in strategies, backtesting, and a web dashboard.

## Build Commands

```bash
# Fast local build (no web dashboard — preferred for development)
make bbgo-slim          # → build/bbgo/bbgo-slim

# Full build with embedded web dashboard (requires frontend assets)
make bbgo               # → build/bbgo/bbgo

# Build with high-precision decimal math
make bbgo-slim-dnum

# Rebuild embedded frontend assets (only needed for full build)
make static
```

Build tags: `web` (embed dashboard), `release` (version stamp), `dnum` (decimal math).

## Testing

```bash
# Run all tests
go test ./pkg/...

# Run a specific package/test (preferred during iteration)
go test ./pkg/bbgo -v -run TestLoadConfig

# With race detector and coverage (what CI runs)
go test -count 3 -race -coverprofile coverage.txt -covermode atomic ./pkg/...

# Run with dnum tag (separate test set)
go test -race -tags dnum ./pkg/...
```

Integration tests require exchange API credentials via env vars (e.g., `BINANCE_API_KEY`). Most unit tests run without credentials.

### Test Helpers (`pkg/testing/`)

Two sub-packages provide reusable test utilities:

**`testhelper`** — Domain-specific builders and assertions for trading types:

- `testhelper.Market("BTCUSDT")` / `testhelper.Ticker("ETHUSDT")` — pre-defined market and ticker fixtures (BTCUSDT, ETHUSDT, USDCUSDT, USDTTWD, BTCTWD)
- `testhelper.Number(99.5)` — flexible conversion (string/int/float) to `fixedpoint.Value`
- `testhelper.Balance("BTC", Number(10))` — create a `types.Balance` with zeroed metadata fields
- `testhelper.BalancesFromText("BTC, 10.5\nETH, 100")` — parse text into `types.BalanceMap`
- `testhelper.PriceVolumeSliceFromText("100, 10\n105, 20")` — parse price/volume pairs (supports `//` comments)
- `testhelper.AssertOrdersPriceSideQuantityFromText(t, "BUY, 100, 10\nSELL, 105, 20", orders)` — assert order slice against text spec
- `testhelper.MatchOrder(order)` / `testhelper.Catch(func(x any){...})` — testify matchers for use with mocks

**`httptesting`** — HTTP client mocking and record/replay:

- `httptesting.HttpClientWithJson(data)` — client that returns JSON for any request
- `httptesting.HttpClientSaver(&req, content)` — client that captures the request for inspection
- `MockTransport` — register handlers per method/path: `transport.GET("/api/v1/orders", handlerFunc)`
- `Recorder` — records live HTTP interactions, saves to JSON, replays later via `MockTransport.LoadFromRecorder()`. Automatically strips credential headers. Control with `TEST_HTTP_RECORD=1` env var.
- `BuildResponseJson(code, payload)` / `SetHeader(resp, k, v)` — chainable response builders

## Linting

CI uses **revive** for linting and **golangci-lint** with: staticcheck, bodyclose, contextcheck, dupword, decorder, goconst, govet, gosec, misspell. Format with `gofmt -s -w`.

## Code Generation

Several generators produce committed files — re-run and commit when touching their inputs:

```bash
# SQL migrations (after changing migrations/*.sql)
make migrations         # uses rockhopper → pkg/migrations/{mysql,sqlite3}/

# gRPC protobuf (after changing pkg/pb/*.proto)
make grpc-go            # requires protoc; install deps: make install-grpc-tools

# Go generate (requestgen, callbackgen) — run in the relevant package
go generate ./pkg/exchange/max/...
```

- **requestgen**: generates HTTP API request builders from `//go:generate requestgen` directives
- **callbackgen**: generates `OnXxx()` callback registrations from `//go:generate callbackgen` directives

## Architecture

### Core Flow

```
CLI (cmd/bbgo → pkg/cmd)
  → Environment (pkg/bbgo/environment.go) — manages exchange sessions & services
    → Trader (pkg/bbgo/trader.go) — loads strategies, manages lifecycle
      → Strategies implement SingleExchangeStrategy.Run() or CrossExchangeStrategy.CrossRun()
```

### Key Packages

- **`pkg/bbgo/`** — Core engine: Environment, Trader, Config, strategy registry (`bbgo.RegisterStrategy()`)
- **`pkg/types/`** — Shared types: Exchange interface, Order, Trade, KLine, Balance
- **`pkg/exchange/`** — Exchange adapters (REST + WebSocket); factory in `factory.go`
- **`pkg/strategy/`** — Built-in strategies (grid2, xmaker, bollmaker, supertrend, etc.)
- **`pkg/indicator/`** — Technical indicators (SMA, EMA, MACD, RSI, Bollinger, etc.)
- **`pkg/service/`** — Persistence and business logic (database, backtest, orders, trades)
- **`pkg/core/`** — TradeCollector, order store, KLine driver
- **`pkg/backtest/`** — Backtesting engine
- **`pkg/server/`** — HTTP API and web dashboard server

### Key Interfaces

- `types.Exchange` (`pkg/types/exchange.go`) — unified exchange interface for querying and trading
- `bbgo.SingleExchangeStrategy` / `bbgo.CrossExchangeStrategy` — strategy contracts
- Strategies are registered via `bbgo.RegisterStrategy()` and configured through YAML

### Configuration

YAML config files (e.g., `bbgo.yaml`) define exchange sessions and strategy parameters. API credentials come from `.env.local` environment files. Config parsing is in `pkg/bbgo/config.go`.

### Database

Supports MySQL and SQLite via rockhopper migrations. Migration SQL files are in `migrations/` and compiled Go packages in `pkg/migrations/`.

## Strategy Development

1. Create package under `pkg/strategy/<name>/`
2. Implement `SingleExchangeStrategy` or `CrossExchangeStrategy` interface
3. Register with `bbgo.RegisterStrategy("<name>", &Strategy{})` in an `init()` function
4. Add config tests similar to `pkg/bbgo/config_test.go`
5. Use `testify` for assertions (already in go.mod)

## Build Tag Constraints

Some code and tests use `//go:build dnum` or `//go:build !dnum` to conditionally compile. When adding dnum-specific code paths, mirror the build constraints in tests.
