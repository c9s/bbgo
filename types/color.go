package types

const Green = "#228B22"
const Red = "#800000"

// SideType define side type of order
type SideType string

const (
	SideTypeBuy = SideType("BUY")
	SideTypeSell = SideType("SELL")
)

func (side SideType) Color() string {
	if side == SideTypeBuy {
		return Green
	}
	if side == SideTypeSell {
		return Red
	}

	return "#f0f0f0"
}

func SideToColorName(side SideType) string {
	return side.Color()
}
