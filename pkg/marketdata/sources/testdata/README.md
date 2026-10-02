# Raw fixtures for providers that do not exist yet

These are unmodified samples from Bybit's and OKX's public trade dumps, kept
from the retired `pkg/datasource/csvsource` so that a future `bybitcsv` or
`okexcsv` source has something real to decode against. Both venues have changed
their CDN layouts before, so re-fetching these is not always possible.

| file | venue | shape |
|---|---|---|
| `bybit/FXSUSDT-ticks-2023-10-10.csv` | Bybit | header row; `timestamp` is fractional seconds; carries `homeNotional` and `foreignNotional` |
| `okex/BTC-USDT-aggtrades-2023-11-18.csv` | OKX | header row; `trade_id, side, size, price, created_time` |

Two gotchas worth knowing before writing those decoders, both learned from the
code that used to read these files:

- **The OKX header is GBK-encoded** and renders as mojibake in a UTF-8 terminal.
  A decoder must skip it positionally or transcode it; matching on header names
  will not work. `archive.Reader` already detects it as a header, because
  mojibake is still not numeric.
- **Bybit's dump has no trade id.** The retired code synthesised one from the
  line number, which is not stable across files and must not be used as an
  ordering sequence. Leave `OrderKey.Seq` at zero instead.
