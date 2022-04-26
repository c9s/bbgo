Indicator Interface
-----------------------------------

In bbgo, we've added several interfaces to standardize the indicator protocol.  
The new interfaces will allow strategy developers switching similar indicators without checking the code.  
Signal contributors or indicator developers were also able to be benefit from the existing interface functions, such as `Add`, `Mul`, `Minus`, and `Div`, without rebuilding the wheels.

The series interface in bbgo borrows the concept of `series` type in pinescript that allow us to query data in time-based reverse order (data that created later will be the former object in series). Right now, based on the return type, we have two interfaces been defined in `pkg/types/indicator.go`:

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

Series were used almost everywhere in indicators to return the calculated numeric results, while the use of BoolSeries is quite limited. At this moment, we only use BoolSeries to check if some condition is fullfilled at some timepoint. For example, in `CrossOver` and `CrossUnder` functions if `Last()` returns true, then there might be a cross event happend on the curves at the moment.



