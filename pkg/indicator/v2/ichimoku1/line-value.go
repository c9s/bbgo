package ichimoku

import decimal "github.com/algo-boyz/decimal128"

type ValueLine struct {
	valLine decimal.Decimal
	isNil   bool
}

func (n *ValueLine) Value() interface{} {
	if n.isNil {
		return nil
	}
	return n.valLine
}

func (n *ValueLine) SetValue(v decimal.Decimal) {
	n.valLine = v
	n.isNil = false
}

func NewValue(x decimal.Decimal) ValueLine {
	return ValueLine{x, false}
}

func NewValueLineNil() ValueLine {
	return ValueLine{decimal.Zero, true}
}
