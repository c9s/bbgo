package grid2

import (
	"fmt"
	"math"
	"sort"
	"strconv"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type PinCalculator func() []Pin

type Grid struct {
	UpperPrice fixedpoint.Value `json:"upperPrice"`
	LowerPrice fixedpoint.Value `json:"lowerPrice"`

	// Size is the number of total grids
	Size fixedpoint.Value `json:"size"`

	// TickSize is the price tick size, this is used for truncating price
	TickSize fixedpoint.Value `json:"tickSize"`

	// Spread is a immutable number
	Spread fixedpoint.Value `json:"spread"`

	// Pins are the pinned grid prices, from low to high
	Pins []Pin `json:"pins"`

	pinsCache map[Pin]struct{} `json:"-"`

	calculator PinCalculator
}

type Pin fixedpoint.Value

// roundAndTruncatePrice rounds the given price at prec-1 and then truncate the price at prec
func roundAndTruncatePrice(p fixedpoint.Value, prec int) fixedpoint.Value {
	var pow10 = math.Pow10(prec)
	pp := math.Round(p.Float64()*pow10*10.0) / 10.0
	pp = math.Trunc(pp) / pow10

	pps := strconv.FormatFloat(pp, 'f', prec, 64)
	price := fixedpoint.MustNewFromString(pps)
	return price
}

func removeDuplicatedPins(pins []Pin) []Pin {
	var buckets = map[string]struct{}{}
	var out []Pin

	for _, pin := range pins {
		p := fixedpoint.Value(pin)

		if _, exists := buckets[p.String()]; exists {
			continue
		}

		out = append(out, pin)
		buckets[p.String()] = struct{}{}
	}

	return out
}

func calculateArithmeticPins(lower, upper, spread, tickSize fixedpoint.Value) []Pin {
	var pins []Pin

	// tickSize number is like 0.01, 0.1, 0.001
	var ts = tickSize.Float64()
	var prec = int(math.Round(math.Log10(ts) * -1.0))
	for p := lower; p.Compare(upper.Sub(spread)) <= 0; p = p.Add(spread) {
		price := roundAndTruncatePrice(p, prec)
		pins = append(pins, Pin(price))
	}

	// this makes sure there is no error at the upper price
	upperPrice := roundAndTruncatePrice(upper, prec)
	pins = append(pins, Pin(upperPrice))

	return pins
}

func buildPinCache(pins []Pin) map[Pin]struct{} {
	cache := make(map[Pin]struct{}, len(pins))
	for _, pin := range pins {
		cache[pin] = struct{}{}
	}

	return cache
}

func NewGrid(lower, upper, size, tickSize fixedpoint.Value) *Grid {
	height := upper.Sub(lower)
	one := fixedpoint.NewFromInt(1)
	spread := height.Div(size.Sub(one))

	grid := &Grid{
		UpperPrice: upper,
		LowerPrice: lower,
		Size:       size,
		TickSize:   tickSize,
		Spread:     spread,
	}

	return grid
}

func (g *Grid) CalculateGeometricPins() {
	g.calculator = func() []Pin {
		// TODO: implement geometric calculator
		// return calculateArithmeticPins(g.LowerPrice, g.UpperPrice, g.Spread, g.TickSize)
		return nil
	}

	g.addPins(removeDuplicatedPins(g.calculator()))
}

func (g *Grid) CalculateArithmeticPins() {
	g.calculator = func() []Pin {
		one := fixedpoint.NewFromInt(1)
		height := g.UpperPrice.Sub(g.LowerPrice)
		spread := height.Div(g.Size.Sub(one))
		return calculateArithmeticPins(g.LowerPrice, g.UpperPrice, spread, g.TickSize)
	}

	g.addPins(g.calculator())
}

func (g *Grid) Height() fixedpoint.Value {
	return g.UpperPrice.Sub(g.LowerPrice)
}

func (g *Grid) Above(price fixedpoint.Value) bool {
	return price.Compare(g.UpperPrice) > 0
}

func (g *Grid) Below(price fixedpoint.Value) bool {
	return price.Compare(g.LowerPrice) < 0
}

func (g *Grid) OutOfRange(price fixedpoint.Value) bool {
	return price.Compare(g.LowerPrice) < 0 || price.Compare(g.UpperPrice) > 0
}

func (g *Grid) HasPin(pin Pin) (ok bool) {
	_, ok = g.pinsCache[pin]
	return ok
}

// NextHigherPin finds the next higher pin
func (g *Grid) NextHigherPin(price fixedpoint.Value) (Pin, bool) {
	i := g.SearchPin(price)
	if i < len(g.Pins) && fixedpoint.Value(g.Pins[i]).Compare(price) == 0 && i+1 < len(g.Pins) {
		return g.Pins[i+1], true
	}

	return Pin(fixedpoint.Zero), false
}

// NextLowerPin finds the next lower pin
func (g *Grid) NextLowerPin(price fixedpoint.Value) (Pin, bool) {
	i := g.SearchPin(price)
	if i < len(g.Pins) && fixedpoint.Value(g.Pins[i]).Compare(price) == 0 && i-1 >= 0 {
		return g.Pins[i-1], true
	}

	return Pin(fixedpoint.Zero), false
}

func (g *Grid) FilterOrders(orders []types.Order) (ret []types.Order) {
	for _, o := range orders {
		if !g.HasPrice(o.Price) {
			continue
		}

		ret = append(ret, o)
	}

	return ret
}

func (g *Grid) HasPrice(price fixedpoint.Value) bool {
	if _, exists := g.pinsCache[Pin(price)]; exists {
		return exists
	}

	i := g.SearchPin(price)
	if i >= 0 && i < len(g.Pins) {
		return fixedpoint.Value(g.Pins[i]).Compare(price) == 0
	}
	return false
}

func (g *Grid) SearchPin(price fixedpoint.Value) int {
	i := sort.Search(len(g.Pins), func(i int) bool {
		a := fixedpoint.Value(g.Pins[i])
		return a.Compare(price) >= 0
	})
	return i
}

func (g *Grid) ExtendUpperPrice(upper fixedpoint.Value) (newPins []Pin) {
	if upper.Compare(g.UpperPrice) <= 0 {
		return nil
	}

	newPins = calculateArithmeticPins(g.UpperPrice.Add(g.Spread), upper, g.Spread, g.TickSize)
	g.UpperPrice = upper
	g.addPins(newPins)
	return newPins
}

func (g *Grid) ExtendLowerPrice(lower fixedpoint.Value) (newPins []Pin) {
	if lower.Compare(g.LowerPrice) >= 0 {
		return nil
	}

	n := g.LowerPrice.Sub(lower).Div(g.Spread).Floor()
	lower = g.LowerPrice.Sub(g.Spread.Mul(n))
	newPins = calculateArithmeticPins(lower, g.LowerPrice.Sub(g.Spread), g.Spread, g.TickSize)

	g.LowerPrice = lower
	g.addPins(newPins)
	return newPins
}

func (g *Grid) TopPin() Pin {
	return g.Pins[len(g.Pins)-1]
}

func (g *Grid) BottomPin() Pin {
	return g.Pins[0]
}

func (g *Grid) addPins(pins []Pin) {
	g.Pins = append(g.Pins, pins...)

	sort.Slice(g.Pins, func(i, j int) bool {
		a := fixedpoint.Value(g.Pins[i])
		b := fixedpoint.Value(g.Pins[j])
		return a.Compare(b) < 0
	})

	g.updatePinsCache()
}

func (g *Grid) updatePinsCache() {
	g.pinsCache = buildPinCache(g.Pins)
}

func (g *Grid) String() string {
	return fmt.Sprintf("GRID: priceRange: %f <=> %f size: %f spread: %f tickSize: %f", g.LowerPrice.Float64(), g.UpperPrice.Float64(), g.Size.Float64(), g.Spread.Float64(), g.TickSize.Float64())
}
