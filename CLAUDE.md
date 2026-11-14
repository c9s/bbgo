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

## Pull Request Title Convention

PR titles follow `TYPE: [scope] short imperative description`:

- `TYPE` is uppercase. Common values: `FIX`, `FEATURE`, `REFACTOR`, `CHORE`, `DOCS`, `UPGRADE`, `PORT`, `MISC`, `TEST`.
- `[scope]` is the primary package or strategy touched, e.g. `[xmaker]`, `[autoborrow]`, `[indicator/v2]`, `[ci]`.
- Description stays lowercase and concise (no trailing period).

Examples from the repo: `FIX: [autoborrow] enforce Binance minimum borrow quantity`, `FEATURE: [xmaker] add connectionDelay config to delay connectivity establishment`, `REFACTOR: [xfundingv2] refactor round transfer handling and remove retry logic`.

## Agent Tooling

`agent/` holds material for coding agents: `agent/design/` has the design
records (the why behind a workflow, and the dead ends not worth repeating), and
`agent/scripts/` has the helper scripts, each documented in
`agent/scripts/README.md`.

- **Why did CI fail?** Run `./agent/scripts/ci-logs.sh <pr-number>` rather than
  hand-rolling `gh` pipelines. It classifies the failure (code, flaky test,
  dependency, or a GitHub outage) and prints the command to reproduce it. See
  `agent/design/0002-ci-log-inspection.md`; `gh run view --log-failed` is
  unusable on this repository and the record explains why.

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

## Exchange Development

An exchange adapter has two halves. Read the matching guide before touching either:

- **REST** — `doc/development/exchange-api-layer-guide.md`. The `pkg/exchange/<name>/<name>api`
  package layering rule, `requestgen` request builders (`go:generate` flags, `param` tag
  semantics, response wrappers, rate limiters), and the RestClient/auth contract.
- **WebSocket** — `doc/development/exchange-stream-guide.md`. `types.StandardStream` (the
  shared WebSocket client every exchange embeds): lifecycle, parser/dispatcher pipeline,
  subscription mapping, auth patterns, reconnect-safety rules, and exchange registration.

Both use `pkg/exchange/{okex,binance,coinbase}` as reference implementations. See also
`doc/development/adding-new-exchange.md` for the end-to-end checklist.

Exchange-specific notes, to read before extending that adapter:

- **Backpack** — `doc/development/exchange-backpack.md`. ed25519 signing and the per-endpoint
  "instruction" binding, the spot/perp symbol collapse, order-id and timestamp quirks, order
  book side ordering, the WebSocket envelope, and what a futures/perp PR has to change.

Generated files (`*_requestgen.go`, `*_callbacks.go`) are committed — re-run
`go generate ./pkg/exchange/<name>/...` and commit the result when you change their inputs.

## Market Data Layer

`pkg/marketdata` is the multi-source historical market data layer: an event model
with a total ordering, a pull-based `Source`/`Cursor` contract, and a k-way heap
merge that interleaves several origins by event time. Read
`doc/development/marketdata-layer.md` before touching it or adding a provider.

Providers live in `pkg/marketdata/sources/` (binancecsv, replay, amberdata,
grpcsource) and are built from config by `pkg/marketdata/registry`. The core
package must not import them.

Three facts that are easy to get wrong:

- **data.binance.vision has no L2.** No depth diffs, no price-level snapshots;
  `bookTicker` is L1 and stopped in March 2024, `bookDepth` is percentage-band
  notional. L2 comes from AmberData, a recording, or gRPC.
- **Binance spot archives switched from millisecond to microsecond timestamps**
  mid-dataset. `archive.ParseEpochNano` detects the unit per value; do not assume.
- **Venue order book sequences are not counters.** Contiguity is checked against
  `Event.PrevSeq`, the predecessor the venue names, not `Seq + 1`.

Exercise it with `bbgo marketdata download | dump | record` — no database and no
API key needed.

## DeFi / On-Chain Layer

`pkg/chain/evm` is the EVM access layer for Ethereum, BSC and Polygon, and
`pkg/bbgo/chainsession.go` defines `ChainSession` (one chain + one address, the
on-chain counterpart of `ExchangeSession`). Read
`doc/development/defi-chain-layer.md` before touching it or adding a protocol
adapter under `pkg/defi/`.

Two rules that are easy to get wrong:

- **Raw amounts are `*big.Int`, never `fixedpoint.Value`.** The default build of
  `fixedpoint` is an int64 with 8 decimal places, so wei overflows it. Convert
  only at the strategy/display boundary, via the token decimals.
- **Token decimals are read from the chain.** USDT and USDC are 6 decimals on
  Ethereum and 18 on BSC. `pkg/chain/evm/token` resolves and caches metadata;
  batch multi-contract reads through `pkg/chain/evm/multicall` (Multicall3 is at
  the same address on all three chains).

Transaction sending lives in the same package (`evm.Sender`): one transaction in
flight per address, locally counted nonces, mandatory `eth_call` simulation,
`eth_feeHistory` pricing with an absolute ceiling, automatic same-nonce fee
bumping, and confirmation via the `finalized` tag with a confirmation-count
fallback. `TxResult.GasCost` stays a raw wei `*big.Int`.

Exercise it with `bbgo chain info | balance | allowance`, and the write commands
`bbgo chain send | approve`, which simulate unless `--confirm` is passed — no
database and no exchange API key needed.

## DeFi Protocol Adapters

Protocol adapters live in `pkg/defi/<protocol>/` and are shaped like the
contracts they wrap; there is deliberately no protocol-agnostic
`Quoter`/`Swapper` interface yet. It gets extracted once Uniswap, Curve and
Balancer all exist, because those three swap shapes differ too much to abstract
in advance (router+pool vs. int128 coin index vs. vault poolId).

- **Aave v3** — `doc/development/defi-aave-v3.md`. Read it before touching
  `pkg/defi/aave/v3`. Only the `PoolAddressesProvider` is hardcoded, one address
  per chain; everything else is resolved from it. The units are the trap:
  account amounts are USD with 8 decimals (not token units), the health factor
  is WAD and is max uint256 when there is no debt, and the rates are RAY
  per-second values.

- **Uniswap v3** — `doc/development/defi-uniswap-v3.md`. Read it before touching
  `pkg/defi/uniswap/v3`, which also covers the v3 forks (PancakeSwap). Pool
  addresses come from `Factory.getPool`, never from the CREATE2 derivation,
  whose init code hash differs per fork. `QuoterV2` is declared `view` in this
  package's ABI although it is `nonpayable` on chain, so it can be quoted with
  `eth_call`. The fee ladders differ: Uniswap has 0.3%, PancakeSwap has 0.25%.
  `sqrtPriceX96` must be scaled by `10^(decimals0-decimals1)` or a WBTC/USDC
  pool reads 1e-12 off.

Exercise them with `bbgo aave market | account` and `bbgo uniswap pools | quote`,
plus the write commands `bbgo aave supply | withdraw | borrow | repay` and
`bbgo uniswap swap`, which all need `--confirm`.

## Bitcoin Layer

`pkg/chain/btc` is a Bitcoin Core JSON-RPC client whose types mirror the RPC
JSON. Read `doc/development/bitcoin-chain-layer.md` before building on it: the
UTXO model has no sender field and no balances, `Hash` is stored in wire byte
order (the RPC shows it reversed), and `Amount` is int64 satoshis parsed from
the JSON literal, never through float64.

`pkg/chain/btc/stream` follows the chain block by block by polling
`getbestblockhash`, backfilling from its cursor and reporting reorgs, including
a tip replaced at the same height. Read the "Block stream" section of the guide
before building a watcher on it.

Exercise it with `bbgo btc info | block | tx | utxo | scan | stream` and
`--rpc` or `BTC_RPC_ENDPOINT` — no config file needed.

## Strategy Development

Read `doc/development/strategy-layer-design.md` before adding a non-trivial
mechanism to a strategy or changing a core API. In short: new mechanisms start
inside the strategy that needs them, and are promoted into the core API
(`pkg/bbgo`, `pkg/types`, `pkg/core`, `pkg/indicator`) only once their shape has
stopped changing — the way the Slack notification layer, `pkg/indicator` and
`bbgo.GeneralOrderExecutor` arrived. Strategy code itself should stay as small
and plain as possible; the long-term goal is that a strategy is the trading idea
and nothing else.

1. Create package under `pkg/strategy/<name>/`
2. Implement `SingleExchangeStrategy` or `CrossExchangeStrategy` interface
3. Register with `bbgo.RegisterStrategy("<name>", &Strategy{})` in an `init()` function
4. Add config tests similar to `pkg/bbgo/config_test.go`
5. Use `testify` for assertions (already in go.mod)

## Build Tag Constraints

Some code and tests use `//go:build dnum` or `//go:build !dnum` to conditionally compile. When adding dnum-specific code paths, mirror the build constraints in tests.
