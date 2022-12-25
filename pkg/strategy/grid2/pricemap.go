package grid2

import "github.com/c9s/bbgo/pkg/fixedpoint"

type PriceMap map[string]fixedpoint.Value

func buildGridPriceMap(grid *Grid) PriceMap {
	// Add all open orders to the local order book
	gridPriceMap := make(PriceMap)
	for _, pin := range grid.Pins {
		price := fixedpoint.Value(pin)
		gridPriceMap[price.String()] = price
	}

	return gridPriceMap
}
