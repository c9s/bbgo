# `Run` Command

There are three modes:
1. `runSetup`
2. `runConfig`: the main flow, executes using the given user config (YAML)
3. `runWrapperBinary`

## `runConfig` Flow
1. Create a `bbgo.Environment` object
2. Bootstrap environment
   - Configure Database (`bbgo.Environment.ConfigureDatabase`)
   - Configure Exchange Sessions (`bbgo.Environment.ConfigureExchangeSessions`)
     - Exchange objects are generated and corresponding `bbgo.ExchangeSession` objects are created in this step
     - Exchange session objects will be initialized in subsequent processes
3. Environment Init (`bbgo.Environment.Init`)
   - Exchange session initialization (`bbgo.ExchangeSession.Init`)
4. (optional) Environment Sync
5. Execute Trader
   - Generate Trader objects
   - Initialize
   - Load state
   - Run
6. Wait for signal to stop process
7. Graceful Shutdown

# `exchangetest` Command

The `exchangetest` command executes a series of verification checks on the exchange implementation to ensure it behaves as expected.

## `Stream`: Real-Time WebSocket Feeds

A valid `Stream` must implement the `types.Stream` interface.

A `Stream` can operate in two modes:
- **Public mode**: referred to as the `Market Data Stream` throughout the `bbgo` framework.  
  - Enable this mode with the `.SetPublicOnly()` method.
- **Private mode**: referred to as the `User Data Stream` throughout the `bbgo` framework.  
  - This is the **default mode**.

### Market Data Stream

A market data stream must meet the following requirements:

- Support subscription to `types.BookChannel`  
  - Provides real-time monitoring of the order book.
- Support subscription to `types.MarketTradeChannel`  
  - Provides real-time monitoring of public market trades.
- Support subscription to `types.BookTickerChannel`  
  - Provides real-time monitoring of the best bid/ask.
- Support subscription to `types.KLineChannel`  
  - Provides real-time monitoring of candlestick (kline) data.

### User Data Stream

A user data stream must function correctly without requiring any `.Subscribe()` calls and must satisfy the following requirements:
- Receive updates to the user’s orders.
- Receive updates to trades related to the user’s orders.
- Receive both incremental updates and full snapshots of the user’s balance.
