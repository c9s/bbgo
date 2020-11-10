package types

// SideType define side type of order
type SideType string

const (
	SideTypeBuy  = SideType("BUY")
	SideTypeSell = SideType("SELL")
	SideTypeSelf = SideType("SELF")
)

func (side SideType) Reverse() SideType {
	switch side {
	case SideTypeBuy:
		return SideTypeSell

	case SideTypeSell:
		return SideTypeBuy
	}

	return side
}

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
