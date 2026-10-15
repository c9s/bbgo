# Recording fixture

`binance-BTCUSDT-20260923T03.jsonl` is a real capture, taken with:

    bbgo marketdata record --session binance --symbol BTCUSDT \
        --channels book,trade --depth full --out ./rec --duration 25s --uncompressed

It is here because no public archive can supply L2 — data.binance.vision
publishes no depth diffs — so this is the only real order book data in the
repository, and the format needs to be tested against what a venue actually
sends rather than against something hand-written.

Two deliberate modifications, both noted in the file's own header:

- Only the first snapshot, five diffs and five trades were kept.
- **The snapshot's depth was cut to 20 levels per side.** A full Binance
  snapshot is several thousand levels and 200 KB of JSON; the levels that remain
  are the venue's own values, untouched.

What it demonstrates, and what it does not:

- The `LastUpdateId` values are real and strictly increasing
  (100529710516 onwards). Before the accompanying fix to
  pkg/exchange/binance/stream.go every diff recorded a sequence of zero.
- Consecutive diffs differ by more than one — 100529710516 to 100529710641, for
  instance — with nothing missing. Binance's `u` is the id of the last
  individual update inside a batched event, which is why
  `marketdata.SequenceContiguous` checks `PrevSeq` rather than incrementing.
- The file has no `pu` values, because Binance's previous-update id is not
  exposed through `types.Stream`. Replaying it therefore reports its updates as
  unverified rather than as contiguous, which is the honest outcome.
