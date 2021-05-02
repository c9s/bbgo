package types

import (
	"encoding/json"
	"strings"

	"github.com/pkg/errors"
)

// SideType define side type of order
type SideType string

const (
	SideTypeBuy  = SideType("BUY")
	SideTypeSell = SideType("SELL")
	SideTypeSelf = SideType("SELF")

	// SideTypeBoth is only used for the configuration context
	SideTypeBoth = SideType("BOTH")
)

var ErrInvalidSideType = errors.New("invalid side type")

func (side *SideType) UnmarshalJSON(data []byte) (err error) {
	var s string
	err = json.Unmarshal(data, &s)
	if err != nil {
		return err
	}

	switch strings.ToLower(s) {
	case "buy":
		*side = SideTypeBuy

	case "sell":
		*side = SideTypeSell

	case "both":
		*side = SideTypeBoth

	default:
		err = ErrInvalidSideType
		return err

	}

	return err
}

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
