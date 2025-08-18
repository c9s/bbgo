How To Use Builtin Indicators and Add New Indicators (V2)
=========================================================

## Using Built-in Indicators

In bbgo session, we already have several built-in indicators defined.

We could refer to the live-data without the worriedness of handling market data subscription.
To use the builtin indicators, call `Indicators(symbol)` method to get the indicator set of a symbol,

```go
session.Indicators(symbol string) *IndicatorSet
```

IndicatorSet is a helper that helps you construct the indicator instance.
Each indicator is a stream that subscribes to
an upstream through the callback.

We will explain how the indicator works from scratch in the following section.

The following code will create a kLines stream that subscribes to specific klines from a websocket stream instance:

```go
kLines := indicatorv2.KLines(stream, "BTCUSDT", types.Interval1m)
```

The kLines stream is a special indicator stream that subscribes to the kLine event from the websocket stream. It
registers a callback on `OnKLineClosed`, and when there is a kLine event triggered, it pushes the kLines into its
series, and then it triggers all the subscribers that subscribe to it.

To get the closed prices from the kLines stream, simply pass the kLines stream instance to a ClosedPrice indicator:

```go
closePrices := indicatorv2.ClosePrices(kLines)
```

When the kLine indicator pushes a new kline, the ClosePrices stream receives a kLine object then gets the closed price
from the kLine object.

to get the latest value of an indicator (closePrices), use Last(n) method, where n starts from 0:

```go
lastClosedPrice := closePrices.Last(0)
secondClosedPrice := closePrices.Last(1)
```

To create an EMA indicator instance, again, simply pass the closePrice indicator to the SMA stream constructor:

```go
ema := indicatorv2.EMA(closePrices, 17)
```

If you want to listen to the EMA value events, add a callback on the indicator instance:

```go
ema.OnUpdate(func(v float64) { .... })
```


To combine these techniques together:

```go

// use dot import to use the constructors as helpers
import . "github.com/c9s/bbgo/pkg/indicator/v2"

func main() {
    // you should get the correct stream instance from the *bbgo.ExchangeSession instance
    stream := &types.Stream{}
    
    ema := EMA(
        ClosePrices(
            KLines(stream, types.Interval1m)), 14)
    _ = ema
}
```


## Adding New Indicator

Adding a new indicator is pretty straightforward. Simply create a new struct and insert the necessary parameters as
struct fields.

The indicator algorithm will be implemented in the `Calculate(v float64) float64` method.

You can think of it as a simple input-output model: it takes a float64 number as input, calculates the value, and
returns a float64 number as output.

```
[input float64] -> [Calculate] -> [output float64]
```

Since it will be a float64 value indicator, we will use `*types.Float64Series` here to store our values,
`types.Float64Series` is a struct that contains a slice to store the float64 values, it is implemented as follows:

```go
type Float64Series struct {
	SeriesBase
	Float64Updater
	Slice floats.Slice
}
```

The `Slice` field is a []float64 slice,
which provides some helper methods that helps you do some calculation on the float64 slice.

And Float64Updater provides a way to let indicators subscribe to the updated value:

```go
u := Float64Updater{}
u.OnUpdate(func(v float64) { ... })
u.OnUpdate(func(v float64) { ... })

// to emit the callbacks
u.EmitUpdate(10.0)
```

Now you have a basic concept about the Float64Series, 
we can now start to implement our first indicator structure:

```go
package indicatorv2

type EWMAStream struct {
    // embedded struct to inherit Float64Series methods
	*types.Float64Series

    // parameters we need
	window     int
	multiplier float64
}
```

Again, since it is a float64 value indicator, we use `*types.Float64Series` here to store our values.

And then, add the constructor of the indicator stream:

```go
// the "source" here is your value source
func EWMA(source types.Float64Source, window int) *EWMAStream {
	s := &EWMAStream{
		Float64Series: types.NewFloat64Series(),
		window:        window,
		multiplier:    2.0 / float64(1+window),
	}
	s.Bind(source, s)
	return s
}
```

Where the source refers to your upstream value, such as closedPrices, openedPrices, or any type of float64 series. For
example, Volume could also serve as the source.

The Bind method invokes the `Calculate()` method to obtain the updated value from a callback of the upstream source.
Subsequently, it calls EmitUpdate to activate the callbacks of its subscribers,
thereby passing the updated value to all of them.

Next, write your algorithm within the Calculate method:

```go
func (s *EWMAStream) Calculate(v float64) float64 {
    // if you need the last number to calculate the next value
    // call s.Slice.Last(0)
    //
	last := s.Slice.Last(0)
	if last == 0.0 {
		return v
	}

	m := s.multiplier
	return (1.0-m)*last + m*v
}
```

Sometimes you might need to store the intermediate values inside your indicator, you can add the extra field with type Float64Series like this:

```go
type EWMAStream struct {
    // embedded struct to inherit Float64Series methods
	*types.Float64Series
	
	A *types.Float64Series
	B *types.Float64Series

    // parameters we need
	window     int
	multiplier float64
}
```

In your `Calculate()` method, you can push the values into these float64 series, for example:

```go
func (s *EWMAStream) Calculate(v float64) float64 {
    // if you need the last number to calculate the next value
    // call s.Slice.Last(0)
	last := s.Slice.Last(0)
	if last == 0.0 {
		return v
	}
	
	// If you need to trigger callbacks, use PushAndEmit
	s.A.Push(last / 2)
	s.B.Push(last / 3)

	m := s.multiplier
	return (1.0-m)*last + m*v
}
```


