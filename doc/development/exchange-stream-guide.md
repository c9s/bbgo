# Exchange WebSocket Stream Development Guide

A practical, code-grounded guide for implementing a new exchange WebSocket stream in bbgo,
derived from the three reference implementations:

- `pkg/exchange/okex` — the cleanest, smallest example. Read this first.
- `pkg/exchange/binance` — the most feature-complete: depth buffering, multiple auth modes,
  listen-key/listen-token refresh, a secondary auxiliary connection.
- `pkg/exchange/coinbase` — heavy channel bridging, sequence-number gap detection,
  locally-synthesized klines, `BeforeConnect` pre-flight work.

The framework component every exchange stream embeds is `types.StandardStream`
(`pkg/types/stream.go`). See also `doc/development/exchange-api-layer-guide.md` for the REST
half (requestgen and the `<name>api` package), `doc/topics/standard-stream.md` for the shorter
end-user-facing overview, and `doc/development/adding-new-exchange.md` for the full
new-exchange checklist.

---

## 1. What `types.StandardStream` gives you

`StandardStream` is *not* an interface you implement — it is a struct you **embed**. It owns:

| Concern | Provided by StandardStream |
|---|---|
| Dial + gorilla/websocket connection handling | `Dial`, `DialAndConnect`, `Conn`, `ConnLock`, `ConnCtx`, `ConnCancel` |
| Read loop, read deadlines, close-frame handling | `Read` (started by `DialAndConnect`) |
| Ping worker + optional custom heartbeat | `ping`, `SetPingInterval`, `SetHeartBeat` |
| Reconnect signalling + cool-down loop | `Reconnect()`, `ReconnectC`, `reconnector` |
| Subscription bookkeeping (thread-safe) | `Subscribe`, `GetSubscriptions`, `Resubscribe`, `Subscriptions` |
| Public/private mode flag | `SetPublicOnly()`, `PublicOnly`, `GetPublicOnly()` |
| Message parse + dispatch pipeline | `SetParser`, `SetDispatcher` |
| The standard event hub (`OnKLine`, `EmitOrderUpdate`, …) | generated in `pkg/types/standardstream_callbacks.go` |
| Graceful shutdown | `Close()`, `CloseC` |

You supply exactly three functions plus a set of handlers:

```go
stream.SetEndpointCreator(stream.createEndpoint) // ctx -> ws url
stream.SetParser(parseWebSocketEvent)            // []byte -> typed event
stream.SetDispatcher(stream.dispatchEvent)       // typed event -> Emit*
```

Everything else is optional.

### The `types.Stream` interface

`Exchange.NewStream() types.Stream` must return something satisfying
(`pkg/types/stream.go`):

```go
type Stream interface {
    StandardStreamEventHub                                   // On*/Emit* — from the embedded struct
    Subscribe(channel Channel, symbol string, options SubscribeOptions)
    GetSubscriptions() []Subscription
    Resubscribe(func(old []Subscription) (new []Subscription, err error)) error
    SetPublicOnly()
    GetPublicOnly() bool
    Connect(ctx context.Context) error
    Reconnect()
    Close() error
}
```

Embedding `types.StandardStream` satisfies all of it for free. You only override methods
when you need extra behaviour (see §7).

Optional interfaces the framework probes for with a type assertion
(`pkg/bbgo/session.go:549`):

```go
type PrivateChannelSetter       interface { SetPrivateChannels(channels []string) }   // max
type PrivateChannelSymbolSetter interface { SetPrivateChannelSymbols(symbols []string) } // coinbase
type Unsubscriber               interface { Unsubscribe() }                          // okex
```

Add a compile-time assertion, as coinbase does (`pkg/exchange/coinbase/stream.go:23`):

```go
var (
    _ types.PrivateChannelSymbolSetter = (*Stream)(nil)
    _ types.Stream                     = (*Stream)(nil)
)
```

---

## 2. Lifecycle — exactly what happens and when

```
Connect(ctx)
 ├─ beforeConnect(ctx)                       ← SetBeforeConnect; may return error to abort
 ├─ DialAndConnect(ctx)
 │   ├─ ConnLock.Lock()
 │   ├─ cancel previous ConnCancel + sg.WaitAndClear()   ← old goroutines drained
 │   ├─ Dial(connCtx)
 │   │   └─ endpointCreator(ctx) -> url; sets Ping/Pong handlers
 │   ├─ EmitConnect()                        ← OnConnect: send SUBSCRIBE / LOGIN here
 │   ├─ go Read(connCtx, conn, cancel)
 │   └─ go ping(connCtx, conn, cancel)
 ├─ go reconnector(ctx)                      ← started once, outlives individual connections
 └─ EmitStart()                              ← OnStart, once per Connect() call
```

Read loop (per message):

```
conn.SetReadDeadline(now + 2min)
conn.ReadMessage()
 ├─ error → close conn, Reconnect(), return   (goroutine exits; reconnector re-dials)
 ├─ non-text frame → skip
 ├─ no parser → EmitRawMessage, continue
 └─ parser(msg)
     ├─ error → log, EmitRawMessage, continue  (never kills the loop)
     └─ ok → EmitRawMessage (unless *WebsocketPongEvent) → dispatcher(e)
```

On exit `Read` calls `cancel()` and `EmitDisconnect()`.

Reconnect loop: `Reconnect()` pushes into the buffered (size-1) `ReconnectC`; duplicate signals
coalesce. The reconnector sleeps `reconnectCoolDownPeriod` (15s), then `DialAndConnect`. On error
it re-signals itself, so it retries forever until ctx/CloseC.

Key timing constants (`pkg/types/stream.go`):

```go
pingInterval            = 30 * time.Second   // override with SetPingInterval
readTimeout             = 2 * time.Minute    // doubled on pong receipt
writeTimeout            = 10 * time.Second
reconnectCoolDownPeriod = 15 * time.Second
```

**Important consequences to design around:**

1. `OnConnect` fires on **every** connection, including reconnects. All subscribe/login
   commands must be idempotent and must be (re)sent from `OnConnect`, never from `NewStream`.
2. `OnDisconnect` fires on every disconnect. Reset per-connection state there
   (binance resets depth buffers, coinbase clears sequence numbers and working orders).
3. `endpointCreator` is called on **every** dial. Anything expensive or credential-related
   placed there re-runs per reconnect — binance exploits this to mint a fresh listen key
   (`createUserDataStreamEndpointWithListenKey`).
4. `beforeConnect` runs **once per `Connect()`**, not per reconnect. Put one-time
   validation/setup there (coinbase builds its channel→symbol map; binance loads margin risk
   balances).

---

## 3. Anatomy of a stream package

Recommended file layout (coinbase's split is the most readable at scale):

```
pkg/exchange/<name>/
  exchange.go            // types.Exchange impl; NewStream() types.Stream
  stream.go              // Stream struct, NewStream(), wiring, dispatchEvent, endpoint
  stream_messages.go     // websocket message structs + channel constants (coinbase)
  stream_handlers.go     // handleConnect/handleDisconnect + per-message handlers (coinbase)
  stream_before_connect.go
  stream_callbacks.go    // GENERATED by callbackgen — do not edit
  parse.go               // parseWebSocketEvent: []byte -> typed event
  convert.go             // toGlobalOrder/Trade/Balance/KLine + convertSubscription
  <name>api/             // REST client, generated with requestgen
```

Smaller exchanges (okex) collapse messages/handlers into `stream.go` + `parse.go`.

### The Stream struct

```go
//go:generate callbackgen -type Stream -interface
type Stream struct {
    types.StandardStream           // MUST be embedded by value

    client   *xapi.RestClient      // for REST-assisted work (snapshots, balances)
    exchange *Exchange             // back-reference for QueryDepth / QueryAccountBalances

    // exchange-native callbacks, one slice per native event type.
    // callbackgen turns each `xxxCallbacks []func(T)` field into OnXxx/EmitXxx.
    bookEventCallbacks        []func(book BookEvent)
    marketTradeEventCallbacks []func(e []MarketTradeEvent)
}
```

The `//go:generate callbackgen -type Stream -interface` directive generates
`stream_callbacks.go`. Run `go generate ./pkg/exchange/<name>/...` and commit the output.
Field naming is load-bearing: `fooBarCallbacks []func(x T)` → `OnFooBar(cb func(x T))` and
`EmitFooBar(x T)`.

### The constructor

Follow okex (`pkg/exchange/okex/stream.go:52`) — construct, then wire in a fixed order:

```go
func NewStream(client *xapi.RestClient, ex *Exchange) *Stream {
    s := &Stream{
        StandardStream: types.NewStandardStream(),   // never zero-value the struct
        client:         client,
        exchange:       ex,
    }

    // 1. pipeline
    s.SetParser(parseWebSocketEvent)
    s.SetDispatcher(s.dispatchEvent)
    s.SetEndpointCreator(s.createEndpoint)
    s.SetPingInterval(pingInterval)      // only if the exchange needs != 30s

    // 2. native event -> global event handlers
    s.OnBookEvent(s.handleBookEvent)
    s.OnMarketTradeEvent(s.handleMarketTradeEvent)
    s.OnOrderTradesEvent(s.handleOrderDetailsEvent)

    // 3. lifecycle hooks
    s.SetBeforeConnect(s.beforeConnect)   // optional
    s.OnConnect(s.handleConnect)
    s.OnDisconnect(s.handleDisconnect)
    s.OnAuth(s.subscribePrivateChannels(s.emitBalanceSnapshot))

    return s
}
```

`types.NewStandardStream()` allocates `ReconnectC`, `CloseC`, the `SyncGroup` and the default
ping interval. Forgetting it produces a stream that deadlocks or nil-panics.

---

## 4. The three functions you must write

### 4.1 `endpointCreator` — which URL

Signature `func(ctx context.Context) (string, error)`. Branch on `s.PublicOnly` and on
exchange variants (spot/futures/margin/testnet).

okex, minimal:

```go
func (s *Stream) createEndpoint(ctx context.Context) (string, error) {
    if s.PublicOnly {
        return okexapi.PublicWebSocketURL, nil
    }
    return okexapi.PrivateWebSocketURL, nil
}
```

binance shows the advanced form: the creator performs a REST call to mint a listen key,
starts a keep-alive goroutine, and returns a URL containing that key. Because the creator
runs on every dial, reconnects transparently get a fresh key.

If the URL is a constant, an inline closure is fine
(`pkg/exchange/okex/kline_stream.go:28`).

### 4.2 `parser` — bytes to typed event

Signature `func(message []byte) (interface{}, error)`. It must be a **pure function of the
message** (no stream state), because it also runs in unit tests against recorded fixtures.

Typical shape — decode a discriminator, then decode the payload into the concrete type:

```go
func parseWebSocketEvent(in []byte) (interface{}, error) {
    var event WebSocketEvent
    if err := json.Unmarshal(in, &event); err != nil {
        return nil, err
    }
    if event.Event != "" {          // control frame: subscribe ack / login / error
        return &event, nil
    }
    switch event.Arg.Channel {
    case ChannelBooks, ChannelBooks5:
        var e BookEvent
        if err := json.Unmarshal(event.Data, &e.Data); err != nil {
            return nil, fmt.Errorf("unmarshal BookEvent %s: %w", event.Data, err)
        }
        e.Symbol  = toGlobalSymbol(event.Arg.InstId)   // enrich from the envelope
        e.channel = event.Arg.Channel
        e.Action  = event.ActionType
        return &e, nil
    ...
    }
    return nil, nil   // unknown event: dispatcher must tolerate nil
}
```

Rules:

- Return **pointers** to event structs; the dispatcher's type switch depends on it
  (mixing `BookEvent` and `*BookEvent` silently drops events).
- Returning `(nil, nil)` for unknown messages is accepted — the dispatcher's `default`
  branch should log at debug/warn (coinbase logs `skip dispatching msg due to unknown
  message type: %T`).
- Copy envelope fields (symbol, channel, action) into the event, since the dispatcher
  only sees the event.
- Coinbase's variant discriminates on a `type` string
  (`pkg/exchange/coinbase/parse.go`); binance's uses `fastjson` on `"e"` plus structural
  sniffing for events Binance ships without a type field (`isBookTicker`, `isPartialDepth`).
- Use `types.MillisecondTimestamp` for ms timestamps and `fixedpoint.Value` for all numbers.

### 4.3 `dispatcher` — typed event to native callback

Keep it a dumb type switch that only calls `Emit*`. Business logic belongs in the handlers
registered in `NewStream`. This keeps the fan-out point in one place and lets tests attach
their own `On*` handlers.

```go
func (s *Stream) dispatchEvent(e interface{}) {
    switch et := e.(type) {
    case *WebSocketEvent:            // control frames
        if err := et.IsValid(); err != nil {
            log.Errorf("invalid event: %v", err)
            return
        }
        if et.IsAuthenticated() {
            s.EmitAuth()             // gate private subscriptions on this
        }
    case *BookEvent:
        s.EmitBookEvent(*et)
        s.EmitBookTickerUpdate(et.BookTicker())
    case []MarketTradeEvent:
        s.EmitMarketTradeEvent(et)
    }
}
```

The dispatcher runs **on the read goroutine**. Anything blocking (REST calls, sleeps) must be
moved into a goroutine, or the socket stops being drained and the read deadline trips.

---

## 5. Subscriptions

### Channel vocabulary

Strategies subscribe with bbgo-generic channels (`pkg/types/channel.go`):

```
BookChannel, KLineChannel, TickerChannel, BookTickerChannel, MarketTradeChannel,
AggTradeChannel, ForceOrderChannel, MarkPriceChannel, IndexPriceKLineChannel,
LiquidationOrderChannel, ContractInfoChannel
```

plus `types.SubscribeOptions{Interval, Depth, Speed}`. Your job is to map these onto the
exchange's native channel names in a `convertSubscription` function
(`pkg/exchange/okex/convert.go:101`):

```go
func convertSubscription(s types.Subscription, instType okexapi.InstrumentType) (WebsocketSubscription, error) {
    switch s.Channel {
    case types.KLineChannel:
        return WebsocketSubscription{
            Channel:      Channel(convertIntervalToCandle(s.Options.Interval)),
            InstrumentID: toLocalSymbol(s.Symbol, instType),
        }, nil
    case types.BookChannel:
        ch := ChannelBooks
        switch s.Options.Depth {
        case types.DepthLevelFull:   ch = ChannelBooks
        case types.DepthLevelMedium: ch = ChannelBooks50
        case types.DepthLevel5:      ch = ChannelBooks5
        case types.DepthLevel1:      ch = ChannelBooks1
        }
        return WebsocketSubscription{Channel: ch, InstrumentID: toLocalSymbol(s.Symbol, instType)}, nil
    ...
    }
    return WebsocketSubscription{}, fmt.Errorf("unsupported public stream channel %s", s.Channel)
}
```

Return an error (don't panic, don't silently drop) for unsupported channels; the caller logs
and skips. Coinbase instead logs a warning per unsupported channel and continues
(`buildMapForMarketStream`).

Map depth levels **conservatively**: `DepthLevelFull` should be the deepest incremental
channel, `DepthLevel1` the best-bid/ask channel.

### Sending SUBSCRIBE

Always inside `OnConnect`, reading `s.Subscriptions`:

```go
func (s *Stream) handleConnect() {
    if s.PublicOnly {
        var subs []WebsocketSubscription
        for _, sub := range s.Subscriptions {
            nativeSub, err := convertSubscription(sub, instType)
            if err != nil {
                log.WithError(err).Errorf("subscription convert error")
                continue
            }
            subs = append(subs, nativeSub)
        }
        subscribe(s.Conn, subs)      // conn.WriteJSON({op: "subscribe", args: subs})
    } else {
        // private: send the login/auth command; subscribe from OnAuth
    }
}
```

Write with `s.Conn.WriteJSON(...)`. Batch into as few frames as the exchange allows — most
exchanges rate-limit inbound control frames (Binance: 5 messages/sec).

### Resubscribe / Unsubscribe

`Resubscribe(fn)` swaps the subscription slice under `subLock` and triggers `Reconnect()`.
Implement `Unsubscribe()` (the `types.Unsubscriber` interface) if the exchange supports an
unsubscribe op, mirroring okex:

```go
func (s *Stream) Unsubscribe() {
    _ = syncSubscriptions(s.Conn, s.Subscriptions, WsEventTypeUnsubscribe, instType)
    _ = s.Resubscribe(func(old []types.Subscription) ([]types.Subscription, error) {
        return []types.Subscription{}, nil     // clear
    })
}
```

---

## 6. Authentication and the private (user data) stream

Each exchange session creates **two** streams (`pkg/bbgo/session.go:288`):

```go
userDataStream := exchange.NewStream()     // private; PublicOnly == false
marketDataStream := exchange.NewStream()
marketDataStream.SetPublicOnly()           // public
```

So a single `Stream` type must serve both roles, branching on `s.PublicOnly`.

Three auth patterns appear in the reference code:

**(a) Login frame after connect, then `EmitAuth` on the ack — okex.** The most common.

```go
// OnConnect (private branch)
msTimestamp := ...
sign := okexapi.Sign(msTimestamp+"GET"+"/users/self/verify", s.client.Secret)
s.Conn.WriteJSON(WebsocketOp{Op: "login", Args: []WebsocketLogin{{...}}})

// dispatcher: on the login ack -> s.EmitAuth()
// NewStream: s.OnAuth(s.subscribePrivateChannels(s.emitBalanceSnapshot))
```

The `OnAuth` handler subscribes to the private channels and *then* pulls a REST balance
snapshot and calls `EmitBalanceSnapshot`. Note the wrapping idiom
`subscribePrivateChannels(next func()) func()` — subscribe, then chain.

**(b) Auth in the subscribe frame — coinbase.** Signature/key/passphrase/timestamp are fields
on the subscribe message itself (`authMsg` embedded in `subscribeMsgType1/2`). There is no
auth ack, so coinbase emits `EmitAuth()` optimistically from `handleConnect`.

**(c) URL-embedded credential — binance listen key / listen token.** The credential is minted
via REST inside `createEndpoint` and appended to the URL, with a keep-alive goroutine
refreshing it every 15 minutes. Binance also supports ed25519 `session.logon` and HMAC
`userDataStream.subscribe.signature` over the WS API endpoint, selected by
`detectUserDataStreamType()`.

**Always emit `EmitAuth()`** — `pkg/bbgo` and `types.Connectivity` gate "the session is ready
to trade" on it. If the exchange gives no ack, emit it from `handleConnect` (spawn a goroutine
if you need to avoid blocking the connect path, as binance does with `go s.EmitAuth()`).

**Never log secrets.** Coinbase defines `String()` on its auth-bearing message types that
deliberately omit key and passphrase, with a test asserting the redaction
(`TestStreamSubCmdString`). Do the same.

### Balance snapshot on connect

Websocket balance channels are usually delta-only. Every reference exchange fetches a REST
snapshot right after auth and emits `EmitBalanceSnapshot`:

```go
func (s *Stream) emitBalanceSnapshot() {
    ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
    defer cancel()

    var balances types.BalanceMap
    err := retry.GeneralBackoff(ctx, func() (err error) {
        balances, err = s.exchange.QueryAccountBalances(ctx)
        return err
    })
    if err != nil {
        log.WithError(err).Error("no more attempts to retrieve balances")
        return
    }
    s.EmitBalanceSnapshot(balances)
}
```

Use `pkg/exchange/retry` helpers rather than hand-rolled retry loops. Coinbase additionally
runs a 5-second balance polling loop because its balance channel is documented as lossy.

---

## 7. When to override `Connect` / `Close`

Override only to manage **extra** resources, always delegating to the embedded method:

```go
func (s *Stream) Connect(ctx context.Context) error {
    if err := s.StandardStream.Connect(ctx); err != nil {
        return err
    }
    return s.kLineStream.Connect(ctx)     // okex: klines live on a different endpoint
}

func (s *Stream) Close() error {
    if aux := s.futuresAuxStream; aux != nil {
        _ = aux.Close()                   // binance
    }
    return s.StandardStream.Close()
}
```

### Multiple connections for one logical stream

Two established patterns for exchanges that split channels across endpoints:

- **okex**: a full second `KLineStream` type embedding its own `types.StandardStream`, with
  its own parser/dispatcher, forwarding into the parent via
  `kLineStream.OnKLineClosed(stream.EmitKLineClosed)`. `Stream.Subscribe` routes
  `types.KLineChannel` to the child. The child's `Connect` returns early when it has no
  subscriptions.
- **binance futures**: a bare `*types.StandardStream` auxiliary
  (`initFuturesAuxStream`) reusing the parent's parser and dispatcher, dialed from the
  parent's `handleConnect`. Cheaper when the event handling is identical.

---

## 8. Order book handling

Three tiers, in increasing order of work:

1. **Full snapshot per message** (okex `books5`, coinbase level2 snapshot) — convert and
   `EmitBookSnapshot`.
2. **Snapshot + increments with sequence numbers** — track a per-symbol sequence and drop
   stale/duplicated messages. Coinbase does this with
   `checkAndUpdateSequenceNumber(msgType, productId, seq)` guarded by a mutex, cleared on
   disconnect.
3. **Increments requiring a REST snapshot to bootstrap** (binance depth) — use
   `pkg/depth.Buffer`, which buffers updates, fetches the snapshot, verifies update-id
   contiguity and replays:

```go
f := depth.NewBuffer(func() (types.SliceOrderBook, int64, error) {
    return ex.QueryDepth(context.Background(), e.Symbol)
}, 3*time.Second)
f.OnReady(func(snapshot types.SliceOrderBook, updates []depth.Update) {
    stream.EmitBookSnapshot(snapshot)
    for _, u := range updates { stream.EmitBookUpdate(u.Object) }
})
f.OnPush(func(u depth.Update) { stream.EmitBookUpdate(u.Object) })
// per message:
f.AddUpdate(types.SliceOrderBook{...}, e.FirstUpdateID, e.FinalUpdateID, e.PreviousUpdateID)
```

Reset every buffer in `OnDisconnect` (`f.Reset()`), otherwise the post-reconnect stream is
validated against a stale sequence and never becomes ready.

Emission contract: `EmitBookSnapshot` replaces the book; `EmitBookUpdate` merges into it.
Emitting an update before any snapshot leaves consumers with a partial book.

## 9. KLines the exchange does not provide

Coinbase's WS API has no candle channel on the Exchange endpoint, so it synthesizes klines
from the trade feed via `pkg/core/klinedriver`:

```go
driver := klinedriver.NewTickKLineDriver(symbol, 5*time.Second)
for _, opt := range options {
    if err := driver.AddInterval(opt.Interval); err != nil { ... }
}
s.OnMarketTrade(func(trade types.Trade) { driver.AddTrade(trade) })
driver.SetKLineEmitter(s)             // driver calls EmitKLine / EmitKLineClosed
s.klineDrivers = append(s.klineDrivers, driver)
// started in handleConnect: driver.Run(s.klineCtx); cancelled in handleDisconnect
```

Note that subscribing to `KLineChannel` must then also force a subscription to the underlying
trade channel — coinbase merges the kline symbols into `matchesChannel` in
`buildMapForMarketStream`.

---

## 10. Converting to global types

Conversion lives in `convert.go`, never in the parser. Naming convention is fixed:

```go
func toGlobalSymbol(local string) string
func toLocalSymbol(global string) string
func toGlobalOrder(o *xapi.Order) (types.Order, error)
func toGlobalTrade(t xapi.Trade) (types.Trade, error)
func toGlobalBalance(a *xapi.Account) types.BalanceMap
func toGlobalSide(s xapi.SideType) types.SideType
func toGlobalOrderStatus(...) types.OrderStatus
```

Conversion errors must never abort the read loop. The idiom is log-and-continue, rate-limited
so a persistently malformed feed cannot flood the log
(`pkg/exchange/okex/stream.go:19`):

```go
var marketTradeLogLimiter = rate.NewLimiter(rate.Every(time.Minute), 1)

trade, err := event.toGlobalTrade()
if err != nil {
    if marketTradeLogLimiter.Allow() {
        log.WithError(err).Error("failed to convert to market trade")
    }
    continue
}
s.EmitMarketTrade(trade)
```

### Which standard event to emit

| Situation | Emit |
|---|---|
| Public trade tape | `EmitMarketTrade(types.Trade)` |
| Own fill (private stream) | `EmitTradeUpdate(types.Trade)` |
| Own order state change | `EmitOrderUpdate(types.Order)` |
| Full balance state | `EmitBalanceSnapshot(types.BalanceMap)` |
| Balance delta | `EmitBalanceUpdate(types.BalanceMap)` |
| Full book | `EmitBookSnapshot(types.SliceOrderBook)` |
| Book delta | `EmitBookUpdate(types.SliceOrderBook)` |
| Best bid/ask | `EmitBookTickerUpdate(types.BookTicker)` |
| Unclosed candle | `EmitKLine(types.KLine)` |
| Closed candle | `EmitKLineClosed(types.KLine)` |
| Authenticated | `EmitAuth()` |

Note coinbase's `handleMatchMessage`: the *same* native message becomes a market trade on the
public stream and a trade update on the private stream, chosen by `s.PublicOnly`. When a fill
arrives, emit the trade update first, then the order update — bbgo's `TradeCollector` expects
that order.

Binance's `handleExecutionReportEvent` shows the standard execution-type mapping:
`NEW/CANCELED/REJECTED/EXPIRED/REPLACED` → `EmitOrderUpdate`; `TRADE` → `EmitTradeUpdate`
plus `EmitOrderUpdate` when the resulting order status is `types.OrderStatusFilled`.

---

## 11. Heartbeat, ping and keep-alive

- Default: a websocket ping control frame every 30s, plus a pong handler that extends the read
  deadline to 4 minutes. No work needed if the exchange follows RFC 6455 ping/pong.
- Different interval: `SetPingInterval(20 * time.Second)` — okex must ping faster than its
  30s idle timeout.
- Exchange wants an application-level ping (a JSON `{"op":"ping"}` or a literal `ping` text
  frame): `SetHeartBeat(fn)`. It runs on each tick *before* the control-frame ping; returning
  an error triggers `Reconnect()`.
- If the exchange replies with an application-level pong, have the parser return
  `&types.WebsocketPongEvent{}` so the read loop skips `EmitRawMessage` and the logs stay clean.
- Credential keep-alive (listen keys, tokens) belongs in its own goroutine started from the
  endpoint creator or `BeforeConnect`, bound to the connection context so it dies with the
  connection.

---

## 12. Reconnect-safety checklist

For each piece of state in your Stream, decide where it is reset:

| State | Reset in |
|---|---|
| depth buffers / order book sequence numbers | `OnDisconnect` |
| cached working orders | `OnDisconnect` (coinbase clears; re-queried in `OnConnect`) |
| kline driver contexts | `OnDisconnect` cancel, `OnConnect` recreate |
| subscriptions | never (they are the durable intent) |
| auth token / listen key | `endpointCreator` (per dial) |

Anti-patterns to avoid:

- Sending subscribe commands from `NewStream` or `Connect` instead of `OnConnect` — they are
  lost after the first reconnect.
- Starting goroutines bound to `context.Background()` in `OnConnect` — they leak one instance
  per reconnect. Bind to `s.ConnCtx`, or start them once from `Connect` bound to the caller's
  ctx.
- Blocking inside the dispatcher or an `On*` handler — it stalls the read loop and causes a
  spurious read-deadline reconnect.
- Calling `s.Conn` without holding `ConnLock` from outside the connect path — the pointer is
  swapped on reconnect.

---

## 13. Testing

Unit-test the parser against captured JSON (no network):

```go
func TestParseWebSocketEvent(t *testing.T) {
    in := []byte(`{"arg":{"channel":"books","instId":"BTC-USDT"},"action":"snapshot","data":[...]}`)
    e, err := parseWebSocketEvent(in)
    assert.NoError(t, err)
    book, ok := e.(*BookEvent)
    assert.True(t, ok)
    assert.Equal(t, "BTCUSDT", book.Symbol)
}
```

Put fixtures under `pkg/exchange/<name>/testdata/`. Coinbase also uses the HTTP record/replay
helper (`pkg/testing/httptesting`, `TEST_HTTP_RECORD=1`) plus a websocket record/replay
harness (`getTestStreamOrSkip`) so live-capture tests degrade to replay in CI. Integration
tests must `t.Skip` when credentials are absent.

Manual smoke tests (`doc/development/adding-new-exchange.md`):

```bash
godotenv -f .env.local -- go run ./cmd/bbgo orderbook --config config/bbgo.yaml --session <name> --symbol BTCUSDT
godotenv -f .env.local -- go run ./cmd/bbgo userdatastream --config config/bbgo.yaml --session <name>
```

Dump every raw frame while developing:

```bash
DEBUG_WEBSOCKET_RAW_MESSAGE=true go run ./cmd/bbgo ...
```

`StandardStream.Read` reads the viper key `debug-websocket-raw-message`; there is no CLI flag
for it, but `viper.AutomaticEnv()` with the `-`→`_` key replacer (`pkg/cmd/root.go:217`) makes
the env var above work. Or register
`s.OnRawMessage(func(raw []byte) { log.Info(string(raw)) })` behind a package-level debug flag,
as binance does.

---

## 14. Registering a new exchange

1. `pkg/types/exchange.go` — add `ExchangeBackpack ExchangeName = "backpack"` to the const
   block **and** to `SupportedExchanges`.
2. `pkg/exchange/factory.go` — add a `Factory` entry with `DefaultEnvVarLoader` and a
   constructor reading `OptionKeyAPIKey` / `OptionKeyAPISecret` / `OptionKeyAPIPassphrase`.
   Env vars are then `BACKPACK_API_KEY`, `BACKPACK_API_SECRET`, …
3. `pkg/exchange/backpack/exchange.go` — implement `types.Exchange`
   (`Name`, `PlatformFeeCurrency`, `QueryMarkets`, `QueryTickers`, `QueryOpenOrders`,
   `SubmitOrder`, `CancelOrders`, `QueryAccount`, `QueryAccountBalances`, `NewStream`).
4. `NewStream() types.Stream { return NewStream(e.client, e) }`.
5. Migrations for the kline table (MySQL + SQLite) if backtesting support is wanted —
   see `.claude/skills/add-migration` and `make migrations`.
6. Run `go generate ./pkg/exchange/backpack/...` and commit the generated
   `stream_callbacks.go`.

---

## 15. Implementation order for a new stream

1. REST client with `requestgen` (`<name>api/`), plus `QueryMarkets` / `QueryAccountBalances` —
   the stream needs them for snapshots. See `doc/development/exchange-api-layer-guide.md`.
2. `convert.go`: symbol mapping and `toGlobal*` functions, with table tests.
3. Message structs + `parseWebSocketEvent`, with fixture tests. No network yet.
4. Stream skeleton: struct, `NewStream`, `createEndpoint`, `dispatchEvent` — public only.
   Verify with `bbgo orderbook`.
5. `convertSubscription` + `handleConnect` subscribe path; wire book / book ticker /
   market trade / kline.
6. Private stream: auth, private subscriptions, `EmitAuth`, balance snapshot,
   `EmitOrderUpdate` / `EmitTradeUpdate`. Verify with `bbgo userdatastream`.
7. Robustness: sequence checking or `depth.Buffer`, `OnDisconnect` resets, rate-limited error
   logs, heartbeat tuning.

## Quick reference: files to read

| Question | Read |
|---|---|
| What does StandardStream do? | `pkg/types/stream.go` |
| Smallest complete example | `pkg/exchange/okex/stream.go`, `parse.go` |
| Second endpoint for a channel | `pkg/exchange/okex/kline_stream.go` |
| Depth buffering | `pkg/exchange/binance/stream.go` (`OnDepthEvent`), `pkg/depth/buffer.go` |
| Listen key / token refresh | `pkg/exchange/binance/stream_connect.go` |
| Pre-connect setup, channel bridging | `pkg/exchange/coinbase/stream_before_connect.go` |
| Sequence gap detection | `pkg/exchange/coinbase/stream_handlers.go` |
| Synthesizing klines from trades | `pkg/core/klinedriver/tick_klinedriver.go` |
| How sessions consume streams | `pkg/bbgo/session.go` (`NewExchangeSession`, `Init`) |
