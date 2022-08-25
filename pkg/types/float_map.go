package types

import "github.com/c9s/bbgo/pkg/datatype/floats"

var _ Series = floats.Slice([]float64{}).Addr()
