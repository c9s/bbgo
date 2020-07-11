package types

import "github.com/adshao/go-binance"

const Green = "#228B22"
const Red = "#800000"

func SideToColorName(side binance.SideType) string {
	if side == binance.SideTypeBuy {
		return Green
	}
	if side == binance.SideTypeSell {
		return Red
	}

	return "#f0f0f0"
}
