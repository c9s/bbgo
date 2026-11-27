package indicatorv2

// Direction is the direction a candlestick pattern is looked up for.
type Direction int

const (
	// Bullish looks up the bullish variant of a pattern.
	Bullish Direction = iota + 1
	// Bearish looks up the bearish variant of a pattern.
	Bearish
)

// MaxNumOfPattern is the number of values a pattern stream keeps.
const MaxNumOfPattern = 5_000

// The values a pattern stream emits: a pattern is either absent (Neutral) or
// detected in one of the two directions.
const (
	Neutral = 0.0
	Bull    = 1.0
	Bear    = -1.0
)

const (
	// threshold is the maximum relative body size for a candle to count as
	// "open and close are at the same level", e.g. for a doji.
	threshold = 0.1

	// limit is the minimum relative shadow length for a shadow to count as long.
	limit = 0.2
)
