How To Use Builtin Indicators and Create New Indicators
-------------------------------------------------------

**NOTE THAT V1 INDICATOR WILL BE DEPRECATED, USE V2 INDICATOR INSTEAD**

### Built-in Indicators

In bbgo session, we already have several indicators defined inside.
We could refer to the live-data without the worriedness of handling market data subscription.
To use the builtin ones, we could refer the `StandardIndicatorSet` type:

```go
// defined in pkg/bbgo/session.go
(*StandardIndicatorSet) BOLL(iw types.IntervalWindow, bandwidth float64) *indicator.BOLL
(*StandardIndicatorSet) SMA(iw types.IntervalWindow) *indicator.SMA
(*StandardIndicatorSet) EWMA(iw types.IntervalWindow) *indicator.EWMA
(*StandardIndicatorSet) STOCH(iw types.IntervalWindow) *indicator.STOCH
(*StandardIndicatorSet) VOLATILITY(iw types.IntervalWindow) *indicator.VOLATILITY
```

and to get the `*StandardIndicatorSet` from `ExchangeSession`, just need to call:

```go
indicatorSet, ok := session.StandardIndicatorSet("BTCUSDT") // param: symbol
```

in your strategy's `Run` function.

And in `Subscribe` function in strategy, just subscribe the `KLineChannel` on the interval window of the indicator you want to query, you should be able to acquire the latest number on the indicators.

However, what if you want to use the indicators not defined in `StandardIndicatorSet`? For example, the `AD` indicator defined in `pkg/indicators/ad.go`?

Here's a simple example in what you should write in your strategy code:
```go
import (
	"context"
	"fmt"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/indicator"
)

type Strategy struct {}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol. types.SubscribeOptions{Interval: "1m"})
}

func (s *Strategy) Run(ctx context.Context, oe bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	// first we need to get market data store(cached market data) from the exchange session
	st, ok := session.MarketDataStore(s.Symbol)
	if !ok {
		...
		return err
	}
	// setup the time frame size
	window := types.IntervalWindow{Window: 10, Interval: types.Interval1m}
	// construct AD indicator
	AD := &indicator.AD{IntervalWindow: window}
	// bind indicator to the data store, so that our callback could be triggered
	AD.Bind(st)
	AD.OnUpdate(func (ad float64) {
		fmt.Printf("now we've got ad: %f, total length: %d\n", ad, AD.Length())
	})
}
```

#### To Contribute

try to create new indicators in `pkg/indicator/` folder, and add compilation hint of go generator:
```go
// go:generate callbackgen -type StructName
type StructName struct {
	...
	updateCallbacks []func(value float64)
}

```
And implement required interface methods:
```go

func (inc *StructName) Update(value float64) {
    // indicator calculation here...
    // push value...
}

func (inc *StructName) PushK(k types.KLine) {
    inc.Update(k.Close.Float64())
}

// custom function
func (inc *StructName) CalculateAndUpdate(kLines []types.KLine) {
    if len(inc.Values) == 0 {
        // preload or initialization
       	for _, k := range allKLines {
			inc.PushK(k)
		}

		inc.EmitUpdate(inc.Last())
    } else {
        // update new value only
		k := allKLines[len(allKLines)-1]
		inc.PushK(k)
	    inc.EmitUpdate(calculatedValue) // produce data, broadcast to the subscribers
    }
}

// custom function
func (inc *StructName) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	// filter on interval
	inc.CalculateAndUpdate(window)
}

// required
func (inc *StructName) Bind(updator KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}
```

The `KLineWindowUpdater` interface is currently defined in `pkg/indicator/ewma.go` and may be moved out in the future.

Once the implementation is done, run `go generate` to generate the callback functions of the indicator.
You should be able to implement your strategy and use the new indicator in the same way as `AD`.

#### Generalize

In order to provide indicator users a lower learning curve, we've designed the `types.Series` interface. We recommend indicator developers to also implement the `types.Series` interface to provide richer functionality on the computed result. To have deeper understanding how `types.Series` works, please refer to [doc/development/series.md](./series.md)
