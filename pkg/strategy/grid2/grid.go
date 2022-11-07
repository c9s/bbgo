package grid2

import (
	"math"
	"sort"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type Grid struct {
	UpperPrice fixedpoint.Value `json:"upperPrice"`
	LowerPrice fixedpoint.Value `json:"lowerPrice"`

	// Size is the number of total grids
	Size fixedpoint.Value `json:"size"`

	// TickSize is the price tick size, this is used for truncating price
	TickSize fixedpoint.Value `json:"tickSize"`

	// Pins are the pinned grid prices, from low to high
	Pins []Pin `json:"pins"`

	pinsCache map[Pin]struct{} `json:"-"`
}

type Pin fixedpoint.Value

func calculateArithmeticPins(lower, upper, spread, tickSize fixedpoint.Value) []Pin {
	var pins []Pin
	for p := lower; p.Compare(upper) <= 0; p = p.Add(spread) {
		// tickSize here = 0.01
		pp := math.Trunc(p.Float64()/tickSize.Float64()) * tickSize.Float64()
		pins = append(pins, Pin(fixedpoint.NewFromFloat(pp)))
	}

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
	grid := &Grid{
		UpperPrice: upper,
		LowerPrice: lower,
		Size:       size,
		TickSize:   tickSize,
	}

	var spread = grid.Spread()
	var pins = calculateArithmeticPins(lower, upper, spread, tickSize)
	grid.addPins(pins)
	return grid
}

func (g *Grid) Height() fixedpoint.Value {
	return g.UpperPrice.Sub(g.LowerPrice)
}

// Spread returns the spread of each grid
func (g *Grid) Spread() fixedpoint.Value {
	return g.Height().Div(g.Size)
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

func (g *Grid) ExtendUpperPrice(upper fixedpoint.Value) (newPins []Pin) {
	spread := g.Spread()

	startPrice := g.UpperPrice.Add(spread)
	for p := startPrice; p.Compare(upper) <= 0; p = p.Add(spread) {
		newPins = append(newPins, Pin(p))
	}

	g.UpperPrice = upper
	g.addPins(newPins)
	return newPins
}

func (g *Grid) ExtendLowerPrice(lower fixedpoint.Value) (newPins []Pin) {
	spread := g.Spread()

	startPrice := g.LowerPrice.Sub(spread)
	for p := startPrice; p.Compare(lower) >= 0; p = p.Sub(spread) {
		newPins = append(newPins, Pin(p))
	}

	g.LowerPrice = lower
	g.addPins(newPins)
	return newPins
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
