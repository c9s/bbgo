# Fixtures

Every response fixture here is transcribed from the response examples embedded in
AmberData's own OpenAPI specifications, which are published openly at
docs.amberdata.io — each documentation page has a `.md` twin with the spec
inline. They are not invented to match the decoder.

The API has no free tier, no sandbox and no demo key, so these examples are the
only authoritative response bodies available without paying, and they are what
makes this package testable at all.

| file | source | why it is here |
|---|---|---|
| `futures_trades.json` | `http/market/futures-trades.md` | `tradeId` is a quoted string while `price` and `volume` are bare numbers, and `quoteVolume` and `sequence` are both null |
| `futures_order_book_events.json` | `http/market/futures-order-book-events.md` | `sequence` is a quoted string here, unlike in trades; one ask level has `volume: 0`, which is the removal signal |
| `futures_order_book_events_hr.json` | same page | the same payload as the API returns it **without** `timeFormat=milliseconds`: timestamps become `"2024-06-04 16:23:12 414"`, a space-separated millisecond field that is neither RFC3339 nor a number |
| `futures_order_book_snapshots.json` | `http/market/futures-order-book-snapshots.md` | carries `timestamp` and `currentFunding`, which the events payload does not |
| `futures_ohlcv.json` | `http/market/futures-ohlcv.md` | candles, timestamped at the bucket start |
| `error_gateway_403.json` | observed from the live API | what an unauthenticated request actually returns: the AWS API Gateway shape, which does **not** match the documented error envelope |
| `error_documented_404.json` | the specifications' error schema | the envelope the documentation promises |

The inconsistencies above are the reason `Timestamp`, `Number` and `Uint64` in
this package all accept more than one encoding: the same logical field is typed
differently across endpoints, and the timestamp encoding depends on a query
parameter.
