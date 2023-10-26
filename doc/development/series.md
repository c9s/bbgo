Indicator Interface
-----------------------------------

In bbgo, we've added several interfaces to standardize the indicator protocol.  
The new interfaces will allow strategy developers switching similar indicators without checking the code.  
Signal contributors or indicator developers were also able to be benefit from the existing interface functions, such as `Add`, `Mul`, `Minus`, and `Div`, without rebuilding the wheels.

The series interface in bbgo borrows the concept of `series` type in pinescript that allow us to query data in time-based reverse order (data that created later will be the former object in series). Right now, based on the return type, we have two interfaces been defined in [pkg/types/indicator.go](../../pkg/types/indicator.go):

```go
type Series interface {
	Last() float64       // newest element
	Index(i int) float64 // i >= 0, query float64 data in reverse order using i as index
	Length() int         // length of data piped in array
}
```

and 

```go
type BoolSeries interface {
	Last() bool       // newest element
	Index(i int) bool // i >= 0, query bool data in reverse order using i as index
	Length() int      // length of data piped in array
}
```

Series were used almost everywhere in indicators to return the calculated numeric results, but the use of BoolSeries is quite limited. At this moment, we only use BoolSeries to check if some condition is fulfilled at some timepoint. For example, in `CrossOver` and `CrossUnder` functions if `Last()` returns true, then there might be a cross event happened on the curves at the moment.

#### Expected Implementation

The calculation could either be done during invoke time (lazy init, for example), or pre-calculated everytime when event happens(ex: kline close). If it's done during invoke time and the computation is CPU intensive, better to cache the result somewhere inside the struct. Also remember to always implement the Series interface on indicator's struct pointer, so that access to the indicator would always point to the same memory space.

#### Compile Time Check

We recommend developers to add the following line inside the indicator source:

```go
var _ types.Series = &INDICATOR_TYPENAME{}
// Change INDICATOR_TYPENAME to the struct name that implements types.Series
```

and if any of the method in the interface not been implemented, this would generate compile time error messages.

#### Extended Series

Instead of simple Series interface, we have `types.SeriesExtend` interface that enriches the functionality of `types.Series`. An indicator struct could simply be extended to `types.SeriesExtend` type by embedding anonymous struct `types.SeriesBase`, and instanced by `types.NewSeries()` function. The `types.SeriesExtend` interface binds commonly used functions, such as `Add`, `Reverse`, `Shift`, `Covariance` and `Entropy`, to the original `types.Series` object. Please check [pkg/types/seriesbase_imp.go](../../pkg/types/seriesbase_imp.go) for the extendable functions.

Example:

```go
a := types.NewQueue(3) // types.Queue is a SeriesExtend container type that holds limit number of floats
a.Update(100.)
a.Update(200.)
a.Update(300.)
assert.Equal(t, a.Sum(3), 600.)
```

#### Cautions

Avoid using `floats.Slice` to hold unlimited number of floats, unless you clean up the memory regularly. Manipulate large array of numbers will give huge impact on the computation speed due to long malloc/dealloc time.
