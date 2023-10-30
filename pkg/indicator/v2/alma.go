// Copyright 2022 The Coln Group Ltd
// SPDX-License-Identifier: MIT

package indicatorv2

import (
	"math"

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

	sample []float64
	offset float64
	sigma  float64
	window int
}

// ALMA is a modern low lag moving average.
// Ported from https://www.tradingview.com/pine-script-reference/#fun_alma
// NewALMA creates a new ALMA indicator with default parameters.
func ALMA(source types.Float64Source, window int) *ALMAStream {
	return ALMAWithSigma(source, window, DefaultALMAOffset, DefaultALMASigma)
}

// NewALMAWithSigma creates a new ALMA indicator with the given offset and sigma.
func ALMAWithSigma(source types.Float64Source, window int, offset, sigma float64) *ALMAStream {
	s := &ALMAStream{
		Float64Series: types.NewFloat64Series(),
		window:        window,
		offset:        offset,
		sigma:         sigma,
	}
	s.Bind(source, s)
	return s
}

func (s *ALMAStream) Calculate(v float64) float64 {

	s.sample = WindowAppend(s.sample, s.window-1, v)

	if s.window < 1 {
		return v
	}
	var (
		length    = float64(s.window)
		norm, sum float64
		offset    = s.offset * (length - 1)
		m         = math.Floor(offset)
		sig       = length / s.sigma
	)
	for i := 0; i < len(s.sample); i++ {
		pow := math.Pow(float64(i)-m, 2) / (math.Pow(sig, 2) * 2)
		weight := math.Exp(-1 * pow)
		norm += weight
		sum += s.sample[i] * weight
	}
	ma := sum / norm

	return ma
}

func (s *ALMAStream) Truncate() {
	s.Slice = s.Slice.Truncate(MaxNumOfMA)
}
