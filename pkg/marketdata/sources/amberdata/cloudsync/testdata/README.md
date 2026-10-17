# Parquet fixture

`order-book-updates-binance-BTCUSDT-trimmed.parquet` is the first 400 rows of
AmberData's public CloudSync sample for Binance USDⓈ-M order book updates,
with each row's depth truncated to 12 levels. It is 97 KB; the original is 40 MB.

Regenerate it with:

    go run ./testdata/gen -rows 400 -levels 12

The generator is `//go:build ignore` and is never run by CI. It downloads from
`https://amberdata-samples.s3.amazonaws.com/market/futures/order-book-updates/2026-02-15.binance.BTCUSDT.00.parquet`,
which is publicly readable with **no credentials** — unlike the CloudSync bucket
itself, which is Requester Pays. That is what makes this package verifiable
against real vendor data without paying for access.

## What this fixture established

The specifications do not say what the numeric fields mean. Reading the real
file did:

- `sequence` is Binance's `u`, the last update id the event covers.
- `metadata.firstUpdateId` is Binance's `U`, the first update id it covers.
  Across the full sample the relation `U == previous u + 1` holds exactly, on
  13-digit numbers, in 190 places — which is not a coincidence. This is why the
  reader can populate `Event.PrevSeq` and make contiguity provable, something
  the REST feed cannot do.
- `metadata.lastId` and `metadata.version` are present in the schema but zero
  throughout the Binance samples, so nothing is derived from them.
- Rows are **per side**, tagged `isBid`. One book change is stored as up to two
  rows sharing a timestamp and sequence; 6 of this fixture's 203 changes touch
  only one side, which is normal.
- `data` is a list of `[price, volume]` pairs as **float64**, so a decoded price
  is only as exact as a float64 — unlike the REST feed's decimal strings.
- A `volume` of zero is the removal signal, the same convention as the REST
  events and as Binance itself.

## What it does not establish

**The public sample is decimated.** Only about 0.1% of its 136,669 events
satisfy the contiguity relation, so it is a taste of the product rather than a
usable dataset. Whether the paid files are complete cannot be checked without a
subscription, and is recorded as unverified in the parent package's
documentation.
