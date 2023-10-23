// Copyright 2022 The Coln Group Ltd
// SPDX-License-Identifier: MIT

package trend

import (
	"github.com/thecolngroup/gou/dec"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

const (
	// DefaultALMAOffset is the default offset for the ALMA indicator.
	DefaultALMAOffset = 0.85

	// DefaultALMASigma is the default sigma for the ALMA indicator.
	DefaultALMASigma = 6
)

type ALMAStream struct {
	// embedded struct
	*types.Float64Series

	sample *types.Float64Series
	offset fixedpoint.Value
	sigma  fixedpoint.Value
	window int
}

// ALMA is a modern low lag moving average.
// Ported from https://www.tradingview.com/pine-script-reference/#fun_alma
// NewALMA creates a new ALMA indicator with default parameters.
func ALMA(source types.Float64Source, window int) *ALMAStream {
	return ALMAWithSigma(window, DefaultALMAOffset, DefaultALMASigma)
}

// NewALMAWithSigma creates a new ALMA indicator with the given offset and sigma.
func ALMAWithSigma(window int, offset, sigma float64) *ALMAStream {
	return &ALMAStream{
		window: window,
		offset: fixedpoint.NewFromFloat(offset),
		sigma:  fixedpoint.NewFromFloat(sigma),
	}
}

func (s *ALMAStream) Calculate(v float64) float64 {

	s.sample = v2.WindowAppend(s.sample, s.window-1, v)

	if s.window < 1 {
		return v
	}
	var (
		length    = fixedpoint.NewFromInt(int64(s.window))
		two       = fixedpoint.Two
		norm, sum fixedpoint.Value
		offset    = s.offset.Mul(length.Sub(fixedpoint.One))
		m         = offset.Floor()
		sig       = length.Div(s.sigma)
	)
	for i := 0; i < s.sample.Length(); i++ {
		index := fixedpoint.NewFromInt(int64(i))
		pow := index.Sub(m).Pow(two).Div(sig.Pow(two).Mul(two))
		weight := fixedpoint.Exp(dec.New(-1).Mul(pow))
		norm = norm.Add(weight)
		sum = fixedpoint.NewFromFloat(s.sample.Last(i))
		sum = sum.Mul(weight).Add(sum)
	}
	ma := sum.Div(norm)

	return ma.Float64()
}
