# Backpack Exchange — implementation notes

Working notes from building the Backpack adapter (`pkg/exchange/backpack` +
`pkg/exchange/backpack/backpackapi`). This is the exchange-specific companion to
`exchange-api-layer-guide.md` (REST) and `exchange-stream-guide.md` (WebSocket): those two
describe the bbgo-wide contract, this one records what is peculiar to Backpack and what a
future change needs to know before touching the adapter.

Official API docs: <https://docs.backpack.exchange/>

---

## 1. What is implemented

| Layer | Status |
|---|---|
| `backpackapi` REST client | public market data, account, order, history endpoints |
| `types.Exchange` | 12 required methods |
| `types.ExchangeOrderQueryService` | `QueryOrder`, `QueryOrderTrades` |
| `types.ExchangeTradeHistoryService` | `QueryTrades`, `QueryClosedOrders` |
| `types.Stream` / `types.Unsubscriber` | public + private WebSocket |
| Futures / perp | **not** implemented — the seams are in place, see §9 |

Not implemented, deliberately: `CustomIntervalProvider`, `ExchangeDefaultFeeRates`,
deposit/withdraw, position/leverage/funding services, `account.positionUpdate`.

---

## 2. Authentication: ed25519, not HMAC

Every other exchange in bbgo signs with HMAC-SHA256 over a string. Backpack signs with
**ed25519**, and that changes several things:

- The **API secret is a base64-encoded 32-byte ed25519 seed**, and the **API key is the
  base64-encoded verifying (public) key of that same keypair**. They are not independent
  values: `Auth()` derives the public key from the seed and rejects the pair if it does not
  match the given key. That check has already caught a mis-paste during development — keep it.
- `RestClient.Auth(key, secret) error` **returns an error**, unlike the `Auth()` of the other
  adapters. A bad seed cannot be detected any other way than at parse time, and a silent
  failure surfaces later as an unexplained cascade of signature rejections.
- The signature is `base64(ed25519.Sign(privateKey, signingString))`, sent in `X-Signature`.

### The signing string

```
instruction=<name>&<params sorted alphabetically by "k=v">&timestamp=<ms>&window=<ms>
```

Headers: `X-API-Key`, `X-Signature`, `X-Timestamp`, `X-Window`.

Three traps, all of which cost time during development:

1. **`window` is always in the signing string**, even though the `X-Window` header is
   optional. Default 5000 ms, max 60000 ms.
2. **Parameters come from two places.** For GET requests requestgen puts them in the query
   `url.Values`; for everything else they are in the JSON body map. Exactly one is ever
   populated — `signingParams()` handles both and sorts the resulting `k=v` pairs
   alphabetically.
3. **Numbers must not be formatted in exponent notation.** A JSON round trip turns every
   number into `float64`, and `fmt.Sprintf("%v", 1e9)` produces `1e+09`, which does not match
   what the server signs. `formatSigningValue()` uses `strconv.FormatFloat(v, 'f', -1, 64)`.
   Booleans must be `true`/`false`, not `1`/`0`.

### The instruction problem, and how it is solved

Backpack binds each authenticated endpoint to an **instruction** name (`balanceQuery`,
`orderExecute`, `orderCancel`, …; the full list is `backpackapi/constants.go`). requestgen's
`AuthenticatedAPIClient` interface has **nowhere to carry a per-endpoint value** — it only
gives you `NewAuthenticatedRequest(ctx, method, refURL, params, payload)`.

The chosen solution is a tiny per-request wrapper that binds the instruction to the client:

```go
type instructionClient struct {
    *RestClient
    instruction Instruction
}

func (c *RestClient) withInstruction(instruction Instruction) requestgen.AuthenticatedAPIClient {
    return &instructionClient{RestClient: c, instruction: instruction}
}
```

Each `New<...>Request` constructor passes its own instruction, so **the instruction lives next
to the endpoint it belongs to** rather than in a lookup table that can drift out of sync.

`(*RestClient).NewAuthenticatedRequest` is still defined — it has to be, for `*RestClient` to
satisfy the interface — but it **always returns an error**. That is intentional: an
authenticated request built without an instruction is a bug, and it should fail loudly at the
call site instead of producing a request the server rejects for an unrelated-looking reason.

**When adding an authenticated endpoint:** add the instruction constant, then use
`c.withInstruction(InstructionXxx)` in the constructor. Forgetting it produces the error above,
which is the intended failure mode.

### No response envelope

Backpack returns the payload directly on success — there is no `{code, msg, data}` wrapper, so
there is no `Validate()` hook to detect errors in. Errors have to be recognized at the HTTP
layer instead: `RestClient.SendRequest` overrides the base implementation, and on a non-2xx
decodes the body into a typed `*APIError`. Callers can therefore use `errors.As` to check for
e.g. `INSUFFICIENT_FUNDS`, which the order tests rely on to skip rather than fail.

---

## 3. Symbols

Backpack uses `BASE_QUOTE` for spot and `BASE_QUOTE_PERP` for perpetuals. bbgo's global symbol
is `BASEQUOTE`.

**local → global** is a pure function: drop `_PERP`, remove `_`. Spot and perp therefore
**collapse onto the same global symbol** (`SOL_USDC` and `SOL_USDC_PERP` both → `SOLUSDC`),
exactly like okex's `BTC-USDT` / `BTC-USDT-SWAP`. They are told apart on the way back by market
type.

**global → local** cannot be a pure function, because `SOLUSDC` does not say where the split
is. Two mechanisms, in order:

1. **Runtime map** — `spotSymbolSyncMap` / `perpSymbolSyncMap`, populated by
   `syncLocalSymbolMap()` from the live market list in `QueryMarkets`. Live data is
   authoritative. The sync also **deletes** symbols that no longer appear, so delisted markets
   do not linger.
2. **Fallback** — split on the longest matching known quote suffix from
   `defaultQuoteCurrencies`, used before `QueryMarkets` has run.

The subtle part of the fallback: **only the length ordering matters.** Same-length candidates
can never shadow each other (`BTCUSDT` can only match `USDT`, `BTCUSDC` only `USDC`), but a
shorter candidate must be tried last, or `BTCUSDT` splits into `BTC` + `USD` with a stray `T`.
`sortedQuoteCurrencies` enforces this once at init. Note this is *why* the list is not simply
`currency.FiatCurrencies` — that list has `BUSD` after `USD`, which would mis-split `xxxBUSD`.

The quote list is deliberately **not hardcoded to USDT**: exchanges also quote in USDC and USD
(coinbase lists both `BTC/USD` and `BTC/USDC`).

### `isLocalSymbolOfMarketType` — the collision bug

Several endpoints, `/api/v1/tickers` above all, have **no market type filter** and return spot
and perp together. Since `toGlobalSymbol` drops the `_PERP` suffix, the two collapse onto one
global symbol and **silently overwrite each other**, with a winner decided by Go's random map
iteration order — this produced 41 wrong tickers before it was caught.

**Any endpoint that returns a mixed market list must be filtered with
`isLocalSymbolOfMarketType`.** Check this whenever adding an endpoint. Where a market type
parameter exists (open orders, order history), pass it as well.

---

## 4. Order IDs

`types.Order.OrderID` is a `uint64`; Backpack types its order id as a **string**. In practice
the recorded values are decimal numbers (`59787279606`), so `toGlobalOrderID`:

1. tries `strconv.ParseUint` — this keeps the real exchange id in the field, which is what
   appears in logs and in the database, and preserves ordering;
2. falls back to `util.FNV64` for anything non-numeric, so the conversion can never fail.

**The original string is always preserved in `types.Order.UUID`.** Because the hash is not
reversible, `QueryOrder` must prefer `types.OrderQuery.OrderUUID` — and `CancelOrders`
likewise cannot work from the numeric id alone.

A related API constraint: **`orderId` and `clientId` are mutually exclusive.** Sending both
changes the signing parameters and breaks the signature.

Backpack's **client order id is a `uint32`**, not a free-form string. `parseClientOrderID`
converts, and `clientOrderIDString` maps the unset value `0` back to `""` so it round-trips
through `types.SubmitOrder`.

---

## 5. Local type quirks worth remembering

- **Side is `Bid`/`Ask`**, not `BUY`/`SELL`.
- **Post-only is a separate boolean field**, not an order type. `toLocalOrderType` returns
  `(OrderType, postOnly bool, err)`; `types.OrderTypeLimitMaker` becomes `Limit` + `postOnly`.
- **Timestamps come in three shapes**, and two custom types were needed:
  - `MicrosecondTimestamp` — the order book engine timestamp is in **microseconds**, which no
    shared bbgo type covers.
  - `NullableMillisecondTimestamp` — `types.MillisecondTimestamp` **rejects a null literal**,
    and the order endpoints send `"triggeredAt": null` for conditional fields that do not
    apply. This broke *every* order response until it was found; it is the single highest-value
    thing on this list.
  - The **order history** endpoint encodes `createdAt` as a naive datetime string
    (`2006-01-02 15:04:05`), not a unix timestamp — which is why
    `orderHistoryEntryToGlobalOrder` exists separately from `toGlobalOrder`.
- **`1mo` is spelled `1month`.** Every other interval keeps its usual name. Backpack also
  offers `8h`, which bbgo has no constant for, and lacks bbgo's `2w`.
- **Kline `startTime` is in seconds**, not milliseconds, and is **required**.
- **No minimum notional** is published; `toGlobalMarket` fills `fixedpoint.One` like the other
  adapters. Precision comes from `TickSize.NumFractionalDigits()` (the kucoin idiom), not
  `math.Log10`.
- **No platform token.** Every market is quoted in USDC, so `PlatformFeeCurrency` returns USDC
  as the closest equivalent.

---

## 6. Order book ordering

**Backpack returns both sides in ascending price order.** `types.SliceOrderBook.BestBid()`
returns `Bids[0]`, so bids must be reversed into descending order or `BestBid()` returns the
*worst* bid — which still passes a naive `bid < ask` check and looks plausible in a log. The
live symptom was `best 0.01/98.7` where it should have been `best 98.62/98.63`.

`toGlobalPriceVolumeSlice(levels, descending)` sorts (not reverses, so it stays correct if the
exchange changes) and is the **only** place this can be fixed: `pkg/depth/buffer.go` emits the
snapshot and the updates exactly as they were handed to it, it never sorts. Both the REST
snapshot and the incremental WebSocket updates go through it, and `QueryTicker` reads
`BestBid()`/`BestAsk()` off the converted book rather than indexing the raw response, so the
rule lives in one place.

Zero-quantity levels must survive the conversion — that is how an incremental update signals a
deletion.

Pinned by `TestToGlobalOrderBookOrdering` and `TestDepthEventToGlobalOrderBook`.

---

## 7. WebSocket

Single endpoint `wss://ws.backpack.exchange` for both public and private; they are told apart
by whether the subscribe request is signed.

- **Envelope**: `{"stream": "<name>", "data": {...}}`, with `{"error": {...}}` for rejections.
  The event type is `data.e`.
- **Stream names**: `<type>.<symbol>`, except kline (`kline.<interval>.<symbol>`) and the
  account streams, which have no symbol (`account.orderUpdate`, `account.balanceUpdate`).
- **The kline interval exists only in the stream name**, not in the payload — `ParseWebsocketMessage`
  copies it in from the envelope.
- **Private subscribe** is signed with instruction `subscribe` and **no parameters**:
  `instruction=subscribe&timestamp=<ms>&window=<ms>`. The signature travels as a
  **positional array**: `[key, signature, timestamp, window]`.
- **A successful private subscribe is acknowledged with silence.** The server only sends a
  frame when it *rejects*. `EmitAuth()` is therefore optimistic, and a rejection arrives later
  as an error frame.
- **The balance stream reports only changes, never state.** `emitBalanceSnapshot` pulls the
  balances over REST after subscribing (wrapped in `retry.GeneralBackoff`).
- **Order updates spell the order type upper case** (`LIMIT`), unlike REST (`Limit`).
  `WebsocketOrderType.OrderType()` normalizes.
- **Only `orderFill` carries a trade id**; every other order update has null there. The trade
  is emitted *before* the order, which is the order `TradeCollector` expects.
- **The public trade payload has no quote quantity** — it is derived. `m` reports whether the
  *buyer* was the maker, so the taker side is its reverse.
- **Depth** uses Binance-style `U`/`u` update-id ranges, so `depth.Buffer` works directly.
  `handleDisconnect` **resets every buffer**: the next connection's update ids will not line up
  with the old snapshot.

---

## 8. Testing

Two independent record/replay mechanisms, both of which run **offline with no credentials** in
CI:

**HTTP** — `httptesting.RunHttpTestWithRecorder(t, client.HttpClient, "testdata/"+t.Name()+".json")`.
Re-record with:

```bash
TEST_BACKPACK=1 TEST_HTTP_RECORD=1 dotenv -f .env.local -- \
  go test -v ./pkg/exchange/backpack/... -run TestExchange
```

Note `MockTransport.LoadFromRecorder` keys on **method + path only**, so two calls to the same
path in one test return the same recorded response.

**WebSocket** — raw frames in `backpackapi/testdata/*.jsonl`, replayed through the real
parser and dispatcher (`TestStream_replayPublicRecord`, `TestStream_replayPrivateRecord`). The
live connection tests are gated behind `TEST_BACKPACK_WS_LIVE`.

Guidelines that were learned the hard way:

- **Record the live traffic before writing the types.** The published WebSocket docs were
  materially wrong about the envelope, the kline interval and quote volume, the trade quote
  quantity, and the order type casing. Every one of those was found by recording first.
- **Order tests trade real money.** Place far-from-market **post-only** orders, always cancel,
  and verify the account is clean afterwards. A decode failure once aborted a test before the
  cancel and left funds locked.
- **All 176 Backpack markets are USDC-quoted**, so a USDT balance can only trade `USDT_USDC`
  (selling USDT). That constraint shapes every order test.
- **Never commit credentials.** The recorder strips `X-API-Key`; `X-Signature`/`X-Timestamp`/
  `X-Window` remain in the recordings, which is acceptable (an ed25519 signature does not leak
  the private key, and bitfinex likewise commits `Bfx-Signature`). Grep the testdata for the
  literal key and secret before every commit.
- **Clear the symbol maps** in tests that depend on the fallback path — they are package-level
  `sync.Map`s and leak between tests otherwise (`clearSymbolMaps`).

---

## 9. Extending to futures / perpetuals

The seams already exist; a futures change should not need to touch call sites.

**Already in place:**
- `Exchange` embeds `types.FuturesSettings`, so it satisfies `types.FuturesExchange` for free.
- `getMarketType()` returns PERP when `IsFutures` is set.
- **Every outbound call resolves symbols through `e.getLocalSymbol()`**, never
  `toLocalSymbol()` directly — so switching market type is a single-point change.
- `perpSymbolSyncMap` and `MarketTypePerp` exist and are wired through `toLocalSymbol`.
- `backpackapi` already has `get_positions_request.go`, `get_mark_prices_request.go`,
  `get_open_interest_request.go`, and `get_collateral_request.go`.
- The `markPrice` WebSocket stream is parsed and dispatched.

**What a futures PR needs to do:**
1. `QueryMarkets` currently filters to `MarketTypeSpot` and leaves the perp map empty — make it
   query PERP when `IsFutures` and call `syncLocalSymbolMap(..., MarketTypePerp)`.
2. Implement the optional futures interfaces over the existing position and collateral
   requests: `types.ExchangeRiskService` (`SetLeverage`, `QueryPositionRisk`) and
   `types.ExchangeFundingFeeService` (`QueryFundingFeeHistory`). Note there is **no** generic
   "futures service" interface in `pkg/types` — position state reaches bbgo through
   `types.PositionRisk` / `types.FuturesPositionMap`, and `pkg/exchange/binance/futures.go`
   plus `convert_futures.go` are the reference implementations.
3. Handle `account.positionUpdate` — the event type constant exists, but **no sample payload
   was ever captured**, so record one before writing the struct (see §8).
4. Audit each endpoint for the market-type collision of §3.
5. `QueryAccount` will need collateral/margin fields that the spot path does not use.

**Other known gaps:**
- The private stream ignores `PrivateChannelSymbols` and subscribes to the account-wide
  `account.orderUpdate`.
- `QueryTickers` leaves `Buy`/`Sell` zero — the ticker endpoint has no bid/ask, and querying
  the order book of every market is not viable. `QueryTicker` (single symbol) does one extra
  depth call to fill them.
- `QueryKLines` has no server-side limit; `options.Limit` is applied by truncating.
- `QueryClosedOrders` filters the since/until window and the `lastOrderID` cursor client-side —
  the order history endpoint offers only limit/offset.
- `QueryTrades` ignores `options.LastTradeID`; the fill history has no "since trade id" cursor.
