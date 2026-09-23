# Market data layer

`pkg/marketdata` reads historical market data from several origins and merges it
into a single stream ordered by event time. It is independent of the backtest
engine: nothing in `pkg/backtest` reads it yet.

## Why it exists

The engine could previously consume one thing from one place. `service.BackTestable`
exposes a single streaming method returning `chan types.KLine`,
`backtest.Exchange.SubscribeMarketData` drops every channel that is not
`KLineChannel`, and the multi-source path in `pkg/cmd/backtest.go` pulls one
kline from each session in turn **without comparing timestamps at all**. So two
exchanges with different kline density silently desynchronise, and L2 has no way
in.

## The three pieces

**`marketdata.Event`** is a tagged union over klines, trades, book snapshots and
updates, book tickers, mark prices, metrics and Binance's bookDepth bands. Each
carries an `OrderKey` that totally orders it against every other event:
nanosecond event time, then a class rank, then the venue sequence, then a source
index.

The rank encodes causality. At one timestamp a trade precedes the book update it
caused; a snapshot precedes the updates applied on top of it; klines come last,
because a kline closing at T summarizes everything up to T. Within klines a
shorter interval precedes a longer one, which reproduces the deliberate
`ORDER BY end_time ASC, start_time DESC` in `BacktestService.QueryKLinesCh` — the
matching engine has to see the 1m bar before the 1h bar that closed at the same
instant.

**`marketdata.Source` and `Cursor`** are a pull-based iterator. A source declares
a `Capability`, and `Open` rejects a request it can serve nothing of rather than
returning an empty cursor. Errors live in exactly one place, `Err()`, which is
what the old `chan types.KLine` plus `chan error` pair kept getting wrong.

**`marketdata.Merge`** is a `container/heap` k-way merge: `O(n log k)` comparisons
and `O(k)` resident memory, with no global buffer and no global sort. That is what
makes a multi-year, multi-source tick replay possible; the previous CSV path read
every tick into memory and sorted it.

## Providers

| type | package | serves | notes |
|---|---|---|---|
| `binanceCsv` | `sources/binancecsv` | trades, aggTrades, klines, bookTicker, bookDepth, metrics | data.binance.vision archives. **No L2.** |
| `replay` | `sources/replay` | whatever was recorded, including L2 | records a live stream and plays it back |
| `amberdata` | `sources/amberdata` | trades, L2 snapshots and events, OHLCV | needs a paid key; `cloudsync` reads the Parquet bulk files |
| `grpc` | `sources/grpcsource` | whatever the server serves | client only; no server implementation yet |

### There is no L2 in the public Binance archives

This trips people up, so it is worth stating plainly. data.binance.vision
publishes no depth-diff dataset and no price-level snapshot dataset. The two
datasets whose names suggest otherwise do not help:

- `bookTicker` is L1 only — one best bid and one best ask — and it was
  discontinued: the last file is 2024-03-30, coverage starting 2023-05-16.
- `bookDepth` is aggregate notional within ±1–5% bands of mid, sampled about
  once a minute. It has no prices.

So `binancecsv` does not declare `BookChannel` and says why when asked for it.
Real L2 comes from AmberData, from a recording, or from a gRPC server.

### Timestamp precision is not fixed

Binance moved its **spot** archives from milliseconds to **microseconds**
somewhere between 2023-11 and 2025-01, without renaming the column and without
announcing a date. Futures stayed on milliseconds. `archive.ParseEpochNano`
therefore classifies each value by magnitude, which is exact because the units are
three orders of magnitude apart. A date-keyed table was rejected: the cutover is
undocumented, differs per dataset, and Binance regenerates historical files.

Futures archives also carry a header row while spot archives do not, so
`archive.Reader` detects one by testing whether the first field is numeric.

### Order book sequences are not counters

`marketdata.SequenceContiguous` compares `Event.PrevSeq` — the predecessor the
venue names — and not `Seq + 1`. Binance's `u` is the id of the *last individual
update* inside a batched event, so consecutive events jump by however many updates
they carried: a real capture goes from 100529710516 to 100529710641 with nothing
missing. The rule that actually holds is `U == previous u + 1`.

Where a source cannot supply `PrevSeq`, `BookState` counts the updates it could
not verify rather than accepting them silently or rejecting them falsely. Today
that is the case for:

- **recordings**, because Binance's `pu` lives on `depth.Update` and
  `types.Stream` emits only the inner `SliceOrderBook`;
- **the AmberData REST feed**, which has no equivalent field.

The AmberData **Parquet** files do carry it as `metadata.firstUpdateId`, so that
path proves contiguity.

## Trying it out

```bash
# warm the archive cache
bbgo marketdata download --market um --dataset aggTrades,klines \
    --interval 1h --symbol BTCUSDT --since 2026-09-15 --until 2026-09-17

# print the merged stream; it asserts ordering as it goes
bbgo marketdata dump --market um --dataset aggTrades,klines --interval 1h \
    --symbol BTCUSDT --since 2026-09-15T23:59:00Z --until 2026-09-16T00:01:00Z

# capture real L2, then replay it and verify the book
bbgo marketdata record --session binance --symbol BTCUSDT \
    --channels book,trade --depth full --out ./rec --duration 5m
bbgo marketdata dump --replay ./rec --symbol BTCUSDT --check-book

# or drive everything from a config file
bbgo marketdata dump --config config/marketdata.yaml --from-config \
    --since 2026-09-15 --until 2026-09-16 --check-book
```

## Adding a provider

1. Create `pkg/marketdata/sources/<name>/`, implement `marketdata.Source`, and
   honour the `Cursor` contract in its doc comment — particularly that events are
   non-decreasing and that `Close` releases everything.
2. Declare a `Capability` that is honest about what the origin has. Rejecting a
   request beats returning nothing.
3. Adapt a push-style feed with `marketdata.NewChanCursor`, which implements
   backpressure once for everyone.
4. Register it in `pkg/marketdata/registry` and add its type constant to
   `pkg/marketdata/config.go`.
5. Test against real data from the venue, not fixtures written to match the
   decoder. Every provider here does: `binancecsv` uses trimmed real archives,
   `replay` a real capture, `amberdata` the vendor's own published response
   examples plus a real Parquet sample.

## Wiring it into the engine

Not done yet. The shape it will take: `SubscribeMarketData` builds a
`marketdata.Request` from `MarketDataStream.GetSubscriptions()` — honouring
`BookChannel` and `MarketTradeChannel` instead of logging that they are
unsupported — and the round-robin loop in `pkg/cmd/backtest.go` becomes a single
`MergeCursor` across every session, which is what actually fixes cross-exchange
time skew. `ConsumeKLine` becomes `ConsumeEvent`, and `e.currentTime` advances
from `ev.Key.TimeNs` for every event type rather than only for klines, which is
what makes intra-bar order timing meaningful. A strategy wanting klines from a
trades-only source gets them from `pkg/core/klinedriver.TickKLineDriver`, whose
`ProcessTick` takes an explicit time for exactly this reason.
