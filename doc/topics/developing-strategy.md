# Developing Strategy

There are two types of strategies in BBGO:

1. built-in strategy: like grid, bollmaker, pricealert strategies, which are included in the pre-compiled binary.
2. external strategy: custom or private strategies that you don't want to expose to public.

For built-in strategies, they are placed in `pkg/strategy` of the BBGO source repository.

For external strategies, you can create a private repository as an isolated go package and place your strategy inside
it.

In general, strategies are Go struct, defined in the Go package.

## Quick Start

To add your first strategy, the fastest way is to add it as a built-in strategy.

Simply edit `pkg/cmd/builtin.go` and import your strategy package there.

When BBGO starts, the strategy will be imported as a package, and register its struct to the engine.

You can also create a new file called `pkg/cmd/builtin_short.go` and import your strategy package.

```
import (
    _ "github.com/c9s/bbgo/pkg/strategy/short"
)
```

Create a directory for your new strategy in the BBGO source code repository:

```shell
mkdir -p pkg/strategy/short
```

Open a new file at `pkg/strategy/short/strategy.go` and paste the simplest strategy code:

```
package short

import (
	"context"
	"fmt"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "short"

func init() {
	// Register our struct type to BBGO
	// Note that you don't need to field the fields.
	// BBGO uses reflect to parse your type information.
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
    Symbol string `json:"symbol"`
    Interval types.Interval `json:"interval"`
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	session.MarketDataStream.OnKLineClosed(func(k types.KLine) {
	    fmt.Println(k)
	})
    return nil
}
```

This is the most simple strategy with only ~30 lines code, it subscribes to the kline channel with the given symbol from
the config, And when the kline is closed, it prints the kline to the console.

Note that, when Run() is executed, the user data stream is not connected to the exchange yet, but the history market
data is already loaded, so if you need to submit an order on start, be sure to write your order submit code inside the
event closures like `OnKLineClosed` or `OnStart`.

Now you can prepare your config file, create a file called `bbgo.yaml` with the following content:

```yaml
exchangeStrategies:
- on: binance
  short:
    symbol: ETHUSDT
    interval: 1m
```

And then, you should be able to run this strategy by running the following command:

```shell
go run ./cmd/bbgo run
```

## The Strategy Struct

BBGO loads the YAML config file and re-unmarshal the settings into your struct as JSON string, so you can define the
json tag to get the settings from the YAML config.

For example, if you're writing a strategy in a package called `short`, to load the following config:

```yaml
externalStrategies:
- on: binance
  short:
    symbol: BTCUSDT
```

You can write the following struct to load the symbol setting:

```
package short

type Strategy struct {
    Symbol string `json:"symbol"`
}

```

To use the Symbol setting, you can get the value from the Run method of the strategy:

```
func (s *Strategy) Run(ctx context.Context, session *bbgo.ExchangeSession) error {
    // you need to import the "log" package
    log.Println("%s", s.Symbol)
    return nil
}
```

Now you have the Go struct and the Go package, but BBGO does not know your strategy, so you need to register your
strategy.

Define an ID const in your package:

```
const ID = "short"
```

Then call bbgo.RegisterStrategy with the ID you just defined and a struct reference:

```
func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}
```

Note that you don't need to fill the fields in the struct, BBGO just need to know the type of struct.

(BBGO use reflect to parse the fields from the given struct and allocate a new struct object from the given struct type
internally)

## Exchange Session

The `*bbgo.ExchangeSession` represents a connectivity to a crypto exchange, it's also a hub that connects to everything
you need, for example, standard indicators, account information, balance information, market data stream, user data
stream, exchange APIs, and so on.

By default, BBGO checks the environment variables that you defined to detect which exchange session to be created.

For example, environment variables like `BINANCE_API_KEY`, `BINANCE_API_SECRET` will be transformed into an exchange
session that connects to Binance.

You can not only connect to multiple different crypt exchanges, but also create multiple sessions to the same crypto
exchange with few different options.

To do that, add the following section to your `bbgo.yaml` config file:

```yaml
---
sessions:
  binance:
    exchange: binance
    envVarPrefix: binance
  binance_cross_margin:
    exchange: binance
    envVarPrefix: binance
    margin: true
  binance_margin_ethusdt:
    exchange: binance
    envVarPrefix: binance
    margin: true
    isolatedMargin: true
    isolatedMarginSymbol: ETHUSDT
  okex1:
    exchange: okex
    envVarPrefix: okex
  okex2:
    exchange: okex
    envVarPrefix: okex
```

You can specify which exchange session you want to mount for each strategy in the config file, it's quiet simple:

```yaml
exchangeStrategies:
  
- on: binance_margin_ethusdt
  short:
    symbol: ETHUSDT

- on: binance_margin
  foo:
    symbol: ETHUSDT

- on: binance
  bar:
    symbol: ETHUSDT
```

## Market Data Stream and User Data Stream

When BBGO connects to the exchange, it allocates two stream objects for different purposes.

They are:

- MarketDataStream receives market data from the exchange, for example, KLine data (candlestick, or bars), market public
  trades.
- UserDataStream receives your personal trading data, for example, orders, executed trades, balance updates and other
  private information.

To add your market data subscription to the `MarketDataStream`, you can register your subscription in the `Subscribe` of
the strategy code, for example:

```
func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: "1m"})
}
```

Since the back-test engine is a kline-based engine, to subscribe market trades, you need to check if you're in the
back-test environment:

```
func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
    if !bbgo.IsBackTesting {
        session.Subscribe(types.MarketTradeChannel, s.Symbol, types.SubscribeOptions{})
    }
}
```

To receive the market data from the market data stream, you need to register the event callback:

```
func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
	    // handle closed kline event here
	})
	session.MarketDataStream.OnMarketTrade(func(trade types.Trade) {
	    // handle market trade event here
	})
}
```

In the above example, we register our event callback to the market data stream of the current exchange session, The
market data stream object here is a session-wide market data stream, so it's shared with other strategies that are also
using the same exchange session, you might receive kline with different symbol or interval.

so it's better to add a condition to filter the kline events:

```
func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
        if kline.Symbol != s.Symbol || kline.Interval != s.Interval {
            return
        }
        
        // handle your kline here
	})
}
```

You can also use the KLineWith method to wrap your kline closure with the filter condition:

```
func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	session.MarketDataStream.OnKLineClosed(types.KLineWith("BTCUSDT", types.Interval1m, func(kline types.KLine) {
	    // handle your kline here
	})
}
```

Note that, when the Run() method is executed, the user data stream and market data stream are not connected yet.










