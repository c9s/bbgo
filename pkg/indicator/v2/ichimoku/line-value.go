package ichimoku

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type ValueLine struct {
	valLine fixedpoint.Value
	isNil   bool
}

func (n *ValueLine) Value() interface{} {
	if n.isNil {
		return nil
	}
	return n.valLine
}

func (n *ValueLine) SetValue(v fixedpoint.Value) {
	n.valLine = v
	n.isNil = false
}

func NewValue(x fixedpoint.Value) ValueLine {
	return ValueLine{x, false}
}

func NewValueLineNil() ValueLine {
	return ValueLine{fixedpoint.Zero, true}
}
