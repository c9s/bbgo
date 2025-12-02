# StandardStream: Reusable WebSocket client for exchanges

StandardStream is a small, composable WebSocket client used across exchanges in bbgo. It encapsulates connection lifecycle, periodic pings, reconnection signaling, message parsing/dispatch, and a rich callback/event API.

This document shows how to use StandardStream, based on a real example (Binance stream) and explains important behaviors like reconnection and thread-safety.

Key types are defined in pkg/types/stream.go and generated callbacks in pkg/types/standardstream_callbacks.go.


## Quick start

Typical setup sequence:

- Create a stream
- Provide a way to build the endpoint URL (SetEndpointCreator) or pass a URL directly to Dial/DialAndConnect
- Provide a parser to turn raw bytes into events
- Provide a dispatcher to route parsed events
- Optionally register callbacks (OnConnect/OnDisconnect/OnAuth/etc.)
- Connect with a context

Example (adapted from pkg/exchange/binance/stream.go):

```go
stream := types.NewStandardStream()

// Provide a dynamic endpoint creator
stream.SetEndpointCreator(func(ctx context.Context) (string, error) {
    return "wss://example.com/ws", nil
})

// Parse incoming JSON into a concrete event type
stream.SetParser(func(message []byte) (interface{}, error) {
    var e MyEvent
    if err := json.Unmarshal(message, &e); err != nil { return nil, err }
    return &e, nil
})

// Dispatch parsed events
stream.SetDispatcher(func(e interface{}) {
    switch v := e.(type) {
    case *MyEvent:
        // handle event
    }
})

// Optional hooks
stream.OnConnect(func() { log.Infof("connected") })
stream.OnDisconnect(func() { log.Infof("disconnected") })
stream.SetBeforeConnect(func(ctx context.Context) error {
    // perform any pre-connect work (e.g., refresh tokens)
    return nil
})

// Start the stream (spawns internal goroutines and emits OnStart)
if err := stream.Connect(ctx); err != nil {
    return err
}
```


## Public vs private (user-data) streams

- Call SetPublicOnly() before Connect() if the stream should connect only to public market-data endpoints.
- For private streams, the exchange-specific implementation usually performs authentication or listen-key creation in its SetEndpointCreator/OnConnect hooks (see Binance example).


## Subscriptions

- Use Subscribe(channel, symbol, options) to record desired subscriptions before connecting.
- The concrete exchange stream (e.g., Binance) typically reads `s.Subscriptions` inside its OnConnect handler and sends the broker-specific SUBSCRIBE command.
- Resubscribe(fn) safely replaces the subscriptions slice and triggers reconnection for you:
  - fn receives the old slice and returns a new slice.
  - Resubscribe locks internally, updates the slice, and calls Reconnect().


## Parsing and dispatching

- If no parser is set, StandardStream emits every text frame via OnRawMessage callbacks.
- If a parser is set, the Read loop calls parser(message) and:
  - On success: emits OnRawMessage (except for special internal pong events) and calls the dispatcher if set.
  - On parse error: logs the error and still emits OnRawMessage for visibility.

Make your dispatcher non-blocking or spawn its own goroutine if it may block.


## Heartbeat and pings

- StandardStream periodically sends WebSocket pings (default every 30s). Change interval via SetPingInterval.
- You can provide a custom heartbeat hook with SetHeartBeat. It’s invoked on every tick before sending the ping. Return an error from your heartbeat to force a reconnect.


## Reconnection model

- Reconnect() sends a signal into ReconnectC. A dedicated reconnector goroutine (started by Connect) cools down for 15s and then calls DialAndConnect again.
- Reconnect events are triggered on read errors, ping/heartbeat errors, and when Resubscribe is called.
- DialAndConnect tears down the previous connection contexts and (re)spawns the reader and ping goroutines.


## Thread-safety and locking

- Subscriptions are protected by an internal mutex. Use Subscribe/Resubscribe to modify them.
- The connection handle and its context/cancel are protected by ConnLock and are reset atomically by SetConn.


## Callbacks (event hub)

StandardStream exposes a broad set of callbacks (see pkg/types/standardstream_callbacks.go) including:

- Lifecycle: OnStart, OnConnect, OnDisconnect, OnAuth, OnRawMessage
- Data: OnKLine, OnKLineClosed, OnBookUpdate, OnBookSnapshot, OnBookTickerUpdate, OnMarketTrade, OnAggTrade
- Account: OnTradeUpdate, OnOrderUpdate, OnBalanceSnapshot, OnBalanceUpdate
- Futures: OnFuturesPositionUpdate, OnFuturesPositionSnapshot

Exchange-specific streams translate their native events into these standard callbacks (see Binance stream for reference).


## Minimal raw-message example

If you only need raw frames:

```go
s := types.NewStandardStream()
s.SetEndpointCreator(func(ctx context.Context) (string, error) {
    return testWsURL, nil
})
s.OnRawMessage(func(raw []byte) { fmt.Println(string(raw)) })
if err := s.Connect(ctx); err != nil { log.Fatal(err) }
```


## Shutdown

- Close() signals all internal goroutines to stop, writes a close frame, and cancels the connection context.
- After Close(), you can drop the stream instance. To reconnect later, create a new StandardStream or call Connect again after proper reinitialization.
