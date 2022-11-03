package grid2

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type Grid struct {
	UpperPrice fixedpoint.Value `json:"upperPrice"`
	LowerPrice fixedpoint.Value `json:"lowerPrice"`

	// Spread is the spread of each grid
	Spread fixedpoint.Value `json:"spread"`

	// Size is the number of total grids
	Size fixedpoint.Value `json:"size"`

	// Pins are the pinned grid prices, from low to high
	Pins []fixedpoint.Value `json:"pins"`

	pinsCache map[fixedpoint.Value]struct{} `json:"-"`
}

func NewGrid(lower, upper, density fixedpoint.Value) *Grid {
	var height = upper - lower
	var size = height.Div(density)
	var pins []fixedpoint.Value

	for p := lower; p <= upper; p += size {
		pins = append(pins, p)
	}

	grid := &Grid{
		UpperPrice: upper,
		LowerPrice: lower,
		Size:       density,
		Spread:     size,
		Pins:       pins,
		pinsCache:  make(map[fixedpoint.Value]struct{}, len(pins)),
	}
	grid.updatePinsCache()
	return grid
}

func (g *Grid) Above(price fixedpoint.Value) bool {
	return price > g.UpperPrice
}

func (g *Grid) Below(price fixedpoint.Value) bool {
	return price < g.LowerPrice
}

func (g *Grid) OutOfRange(price fixedpoint.Value) bool {
	return price < g.LowerPrice || price > g.UpperPrice
}

func (g *Grid) updatePinsCache() {
	for _, pin := range g.Pins {
		g.pinsCache[pin] = struct{}{}
	}
}

func (g *Grid) HasPin(pin fixedpoint.Value) (ok bool) {
	_, ok = g.pinsCache[pin]
	return ok
}

func (g *Grid) ExtendUpperPrice(upper fixedpoint.Value) (newPins []fixedpoint.Value) {
	g.UpperPrice = upper

	// since the grid is extended, the size should be updated as well
	g.Size = (g.UpperPrice - g.LowerPrice).Div(g.Spread).Floor()

	lastPin := g.Pins[len(g.Pins)-1]
	for p := lastPin + g.Spread; p <= g.UpperPrice; p += g.Spread {
		newPins = append(newPins, p)
	}

	g.Pins = append(g.Pins, newPins...)
	g.updatePinsCache()
	return newPins
}

func (g *Grid) ExtendLowerPrice(lower fixedpoint.Value) (newPins []fixedpoint.Value) {
	g.LowerPrice = lower

	// since the grid is extended, the size should be updated as well
	g.Size = (g.UpperPrice - g.LowerPrice).Div(g.Spread).Floor()

	firstPin := g.Pins[0]
	numToAdd := (firstPin - g.LowerPrice).Div(g.Spread).Floor()
	if numToAdd == 0 {
		return newPins
	}

	for p := firstPin - g.Spread.Mul(numToAdd); p < firstPin; p += g.Spread {
		newPins = append(newPins, p)
	}

	g.Pins = append(newPins, g.Pins...)
	g.updatePinsCache()
	return newPins
}
