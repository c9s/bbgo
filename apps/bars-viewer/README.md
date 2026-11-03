# bars-viewer

A standalone web UI for watching K-lines and [`indicator/v2`](../../pkg/indicator/v2)
tick/volume bars live. Two panels:

- **top** — tick bars or volume bars (mode + parameter selectable)
- **bottom** — regular K-lines (interval selectable)

Both panels share one symbol selector and reconnect automatically when you
change any dropdown/param.

This frontend talks to [`cmd/bars-backend`](../../cmd/bars-backend), a small
Go server written specifically for this UI — it does **not** reuse bbgo's own
`pkg/server` dashboard.

## 1. Run the backend

From the repo root (`bbgo/`):

```bash
go run ./cmd/bars-backend
```

By default it listens on `:8090`, tracks `BTCUSDT,ETHUSDT` on Binance, and
tracks the `1m,5m,15m,1h,4h,1d` kline intervals. Override with flags:

```bash
go run ./cmd/bars-backend \
  -listen :8090 \
  -exchange binance \
  -symbols BTCUSDT,ETHUSDT \
  -intervals 1m,5m,15m,1h,4h,1d \
  -cors-origin http://localhost:3100 \
  -history-limit 500
```

### API key for the bars panel (tick/volume bars)

The K-line panel works with no credentials at all (public kline/trade
websocket channels + public REST klines). The **bars panel needs more**: it
backfills history via Binance's `GET /api/v3/historicalTrades`, which
requires an API key (no trading permission needed — a read-only key is
enough). Without one, `/ws/bars` will report `disconnected` in the UI and the
backend log will show an error like:

```
backfill trades for BTCUSDT failed: empty api key
```

Set the standard bbgo exchange env vars before starting the backend:

```bash
export BINANCE_API_KEY=xxxx
export BINANCE_API_SECRET=xxxx
go run ./cmd/bars-backend
```

### Endpoints

| Method | Path | Purpose |
|---|---|---|
| GET | `/api/symbols` | configured symbol list |
| GET | `/api/klines?symbol=&interval=&limit=` | historical klines snapshot |
| GET | `/api/bars?symbol=&type=tick\|volume&param=&limit=` | historical bars snapshot |
| WS | `/ws/klines?symbol=&interval=` | snapshot + live kline updates |
| WS | `/ws/bars?symbol=&type=tick\|volume&param=` | snapshot + live bar updates |

Quick checks once it's running:

```bash
curl http://localhost:8090/api/symbols
curl 'http://localhost:8090/api/klines?symbol=BTCUSDT&interval=1m&limit=5'
curl 'http://localhost:8090/api/bars?symbol=BTCUSDT&type=tick&param=500&limit=5'
```

## 2. Run the frontend

From `apps/bars-viewer/` (this directory):

```bash
yarn install
yarn dev
```

Opens at `http://localhost:3100`. It talks to the backend at
`http://localhost:8090` by default; override with an `.env.local`:

```bash
cp .env.local.example .env.local
# edit NEXT_PUBLIC_BARS_API_URL if the backend runs elsewhere
```

## Notes

- Frontend stack mirrors [`apps/backtest-report`](../backtest-report): Next.js
  + Mantine + `lightweight-charts`, so dependencies/versions are reused
  rather than reinvented.
- If the backend's `-symbols`/`-intervals`/`-cors-origin` differ from the
  defaults above, update them to match whatever you pass on the frontend's
  `NEXT_PUBLIC_BARS_API_URL` host — the backend only accepts the exact
  symbols/intervals it was started with, and only CORS/WS-origin requests
  from `-cors-origin`.
