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

Simply edit `pkg/cmd/strategy/builtin.go` and import your strategy package there.

When BBGO starts, the strategy will be imported as a package, and register its struct to the engine.

You can also create a new file called `pkg/cmd/strategy/short.go` and import your strategy package.

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

```go
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

```go
package short

type Strategy struct {
    Symbol string `json:"symbol"`
}

```

To use the Symbol setting, you can get the value from the Run method of the strategy:

```go
func (s *Strategy) Run(ctx context.Context, session *bbgo.ExchangeSession) error {
    // you need to import the "log" package
    log.Println("%s", s.Symbol)
    return nil
}
```

Now you have the Go struct and the Go package, but BBGO does not know your strategy, so you need to register your
strategy.

Define an ID const in your package:

```go
const ID = "short"
```

Then call bbgo.RegisterStrategy with the ID you just defined and a struct reference:

```go
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

```go
func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: "1m"})
}
```

Since the back-test engine is a kline-based engine, to subscribe market trades, you need to check if you're in the
back-test environment:

```go
func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
    if !bbgo.IsBackTesting {
        session.Subscribe(types.MarketTradeChannel, s.Symbol, types.SubscribeOptions{})
    }
}
```

To receive the market data from the market data stream, you need to register the event callback:

```go
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
using the same exchange session, so you might receive kline with different symbol or interval.

It's better to add a condition to filter the kline events:

```go
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

```go
func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	session.MarketDataStream.OnKLineClosed(types.KLineWith("BTCUSDT", types.Interval1m, func(kline types.KLine) {
	    // handle your kline here
	})
}
```

Note that, when the Run() method is executed, the user data stream and market data stream are not connected yet.

## Submitting Orders

To place an order, you can call `SubmitOrders` exchange API:

```go
createdOrders, err := session.Exchange.SubmitOrders(ctx, types.SubmitOrder{
    Symbol:   "BTCUSDT",
    Type:     types.OrderTypeLimit,
    Price:    fixedpoint.NewFromFloat(18000.0),
    Quantity: fixedpoint.NewFromFloat(1.0),
})
if err != nil {
    log.WithError(err).Errorf("can not submit orders")
}

log.Infof("createdOrders: %+v", createdOrders)
```

There are some pre-defined order types you can use:

- `types.OrderTypeLimit`
- `types.OrderTypeMarket`
- `types.OrderTypeStopMarket`
- `types.OrderTypeStopLimit`
- `types.OrderTypeLimitMaker` - forces the order to be a maker.

Although it's crypto market, the above order types are actually derived from the stock market:

A limit order is an order to buy or sell a stock with a restriction on the maximum price to be paid or the minimum price
to be received (the "limit price"). If the order is filled, it will only be at the specified limit price or better.
However, there is no assurance of execution. A limit order may be appropriate when you think you can buy at a price
lower than--or sell at a price higher than -- the current quote.

A market order is an order to buy or sell a stock at the market's current best available price. A market order typically
ensures an execution, but it does not guarantee a specified price. Market orders are optimal when the primary goal is to
execute the trade immediately. A market order is generally appropriate when you think a stock is priced right, when you
are sure you want a fill on your order, or when you want an immediate execution.

A stop order is an order to buy or sell a stock at the market price once the stock has traded at or through a specified
price (the "stop price"). If the stock reaches the stop price, the order becomes a market order and is filled at the
next available market price.

## UserDataStream

UserDataStream is an authenticated connection to the crypto exchange. You can receive the following data type from the
user data stream:

- OrderUpdate
- TradeUpdate
- BalanceUpdate

When you submit an order to the exchange, you might want to know when the order is filled or not, user data stream is
the real time notification let you receive the order update event.

To get the order update from the user data stream:

```go
session.UserDataStream.OnOrderUpdate(func(order types.Order) {
	if order.Status == types.OrderStatusFilled {
		log.Infof("your order is filled: %+v", order)
    }
})
```

However, order update only contains status, price, quantity of the order, if you're submitting market order, you won't know
the actual price of the order execution.

One order can be filled by different size trades from the market, by collecting the trades, you can calculate the
average price of the order execution and the total trading fee that you used for the order.

If you need to get the details of the trade execution. you need the trade update event:

```go
session.UserDataStream.OnTrade(func(trade types.Trade) {
    log.Infof("trade price %f, fee %f %s", trade.Price.Float64(), trade.Fee.Float64(), trade.FeeCurrency)
})
```

To monitor your balance change, you can use the balance update event callback:

```go
session.UserDataStream.OnBalanceUpdate(func(balances types.BalanceMap) {
    log.Infof("balance update: %+v", balances)
})
```

Note that, as we mentioned above, the user data stream is a session-wide stream, that means you might receive the order update event for other strategies.

To prevent that, you need to manage your active order for your strategy:

```go
activeBook := bbgo.NewActiveOrderBook("BTCUSDT")
activeBook.Bind(session.UserDataStream)
```

Then, when you create some orders, you can register your order to the active order book, so that it can manage the order
update:

```go
createdOrders, err := session.Exchange.SubmitOrders(ctx, types.SubmitOrder{
    Symbol:   "BTCUSDT",
    Type:     types.OrderTypeLimit,
    Price:    fixedpoint.NewFromFloat(18000.0),
    Quantity: fixedpoint.NewFromFloat(1.0),
})
if err != nil {
    log.WithError(err).Errorf("can not submit orders")
}

activeBook.Add(createdOrders...)
```

## Notification

You can use the notification API to send notification to Telegram or Slack:

```go
bbgo.Notify(message)
bbgo.Notify(message, objs...)
bbgo.Notify(format, arg1, arg2, arg3, objs...)
bbgo.Notify(object, object2, object3)
```

Note that, if you're using the third format, simple arguments (float, bool, string... etc) will be used for calling the
fmt.Sprintf, and the extra arguments will be rendered as attachments.

For example:

```go
bbgo.Notify("%s found support price: %f", "BTCUSDT", 19000.0, kline)
```

The above call will render the first format string with the given float number 19000, and then attach the kline object as the attachment.

## Handling Trades and Profit

In order to manage the trades and orders for each strategy, BBGO designed an order executor API that helps you collect
the related trades and orders from the strategy, so trades from other strategies won't bother your logics.

To do that, you can use the *bbgo.GeneralOrderExecutor:

```go
var profitStats = types.NewProfitStats(s.Market)
var position = types.NewPositionFromMarket(s.Market)
var tradeStats = &types.TradeStats{}
orderExecutor := bbgo.NewGeneralOrderExecutor(session, s.Symbol, ID, instanceID, position)

// bind the trade events to update the profit stats
orderExecutor.BindProfitStats(profitStats)

// bind the trade events to update the trade stats
orderExecutor.BindTradeStats(tradeStats)
orderExecutor.Bind()
```

## Graceful Shutdown

When BBGO shuts down, you might want to clean up your open orders for your strategy, to do that, you can use the
OnShutdown API to register your handler.

```go
bbgo.OnShutdown(func(ctx context.Context, wg *sync.WaitGroup) {
    defer wg.Done()

    _, _ = fmt.Fprintln(os.Stderr, s.TradeStats.String())
    if err := s.orderExecutor.GracefulCancel(ctx) ; err != nil {
		log.WithError(err).Error("graceful cancel order error")
    }
})
```

## Persistence

When you need to adjust the parameters and restart BBGO process, everything in the memory will be reset after the
restart, how can we keep these data?

Although BBGO is written in Golang, BBGO provides a useful dynamic system to help you persist your data.

If you have some state needs to preserve before shutting down, you can simply add the `persistence` struct tag to the field,
and BBGO will automatically save and restore your state. For example,

```go
type Strategy struct {
	Position    *types.Position    `persistence:"position"`
	ProfitStats *types.ProfitStats `persistence:"profit_stats"`
	TradeStats  *types.TradeStats  `persistence:"trade_stats"`
}
```

And remember to add the `persistence` section in your bbgo.yaml config:

```yaml
persistence:
  redis:
    host: 127.0.0.1
    port: 6379
    db: 0
```

In the Run method of your strategy, you need to check if these fields are nil, and you need to initialize them:

```go
	if s.Position == nil {
		s.Position = types.NewPositionFromMarket(s.Market)
	}

	if s.ProfitStats == nil {
		s.ProfitStats = types.NewProfitStats(s.Market)
	}

	if s.TradeStats == nil {
		s.TradeStats = types.NewTradeStats(s.Symbol)
	}
```

That's it. Hit Ctrl-C and you should see BBGO saving your strategy states.


## Exit Method Set

To integrate the built-in exit methods into your strategy, simply add a field with type bbgo.ExitMethodSet:

```go
type Strategy struct {
	ExitMethods bbgo.ExitMethodSet `json:"exits"`
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	s.ExitMethods.SetAndSubscribe(session, s)
}

func (s *Strategy) Run() {
    s.ExitMethods.Bind(session, s.orderExecutor)
}
```

And then you can use the following config structure to configure your exit settings like this:

```yaml
- on: binance
  pivotshort:
    exits:
    # (0) roiStopLoss is the stop loss percentage of the position ROI (currently the price change)
    - roiStopLoss:
        percentage: 0.8%

    # (1) roiTakeProfit is used to force taking profit by percentage of the position ROI (currently the price change)
    # force to take the profit ROI exceeded the percentage.
    - roiTakeProfit:
        percentage: 35%

    # (2) protective stop loss -- short term
    - protectiveStopLoss:
        activationRatio: 0.6%
        stopLossRatio: 0.1%
        placeStopOrder: false

    # (3) protective stop loss -- long term
    - protectiveStopLoss:
        activationRatio: 5%
        stopLossRatio: 1%
        placeStopOrder: false

    # (4) lowerShadowTakeProfit is used to taking profit when the (lower shadow height / low price) > lowerShadowRatio
    # you can grab a simple stats by the following SQL:
    # SELECT ((close - low) / close) AS shadow_ratio FROM binance_klines WHERE symbol = 'ETHUSDT' AND `interval` = '5m' AND start_time > '2022-01-01' ORDER BY shadow_ratio DESC LIMIT 20;
    - lowerShadowTakeProfit:
        interval: 30m
        window: 99
        ratio: 3%

    # (5) cumulatedVolumeTakeProfit is used to take profit when the cumulated quote volume from the klines exceeded a threshold
    - cumulatedVolumeTakeProfit:
        interval: 5m
        window: 2
        minQuoteVolume: 200_000_000


```
