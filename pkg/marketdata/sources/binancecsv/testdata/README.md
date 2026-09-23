# Fixtures

Every file here is the first few lines of a real archive from
data.binance.vision, kept byte-for-byte. They are not hand-written, and they are
not normalized: the point is that the decoders are tested against what Binance
actually publishes, including the inconsistencies between trees.

| file | why it is here |
|---|---|
| `um/BTCUSDT-aggTrades-2026-09-15.csv` | futures layout: header row, millisecond timestamps, lowercase `true`/`false` |
| `spot/BTCUSDT-aggTrades-2026-09-15.csv` | spot layout: no header, **microsecond** timestamps, Python-style `True`/`False`, extra `is_best_match` column |
| `spot/BTCUSDT-aggTrades-2023-11-17.csv` | the same spot dataset before the precision change, with **millisecond** timestamps |
| `um/BTCUSDT-1h-2026-09-15.csv` | futures klines: header row, milliseconds |
| `spot/BTCUSDT-1m-2026-09-15.csv` | spot klines: no header, microseconds |
| `um/BTCUSDT-trades-2026-09-15.csv` | raw trades, which publish `quote_qty` directly |
| `um/BTCUSDT-bookDepth-2026-09-15.csv` | percentage-band notional, formatted timestamp, no price levels |
| `um/BTCUSDT-metrics-2026-09-15.csv` | open interest and long/short ratios, formatted timestamp |
| `um/BTCUSDT-bookTicker-2024-01-15.csv` | L1 top of book. Dated 2024-01 because Binance stopped publishing this dataset after 2024-03-30 |

The two spot aggTrades files bracket the undocumented switch from milliseconds
to microseconds, which is the reason `archive.ParseEpochNano` classifies by
magnitude instead of trusting the dataset.

To refresh one:

    curl -sO https://data.binance.vision/data/futures/um/daily/aggTrades/BTCUSDT/BTCUSDT-aggTrades-2026-09-15.zip
    unzip -p BTCUSDT-aggTrades-2026-09-15.zip | head -6 > um/BTCUSDT-aggTrades-2026-09-15.csv
