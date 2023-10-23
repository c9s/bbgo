// Copyright 2022 The Coln Group Ltd
// SPDX-License-Identifier: MIT

package trend

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

func TestALMA(t *testing.T) {

	tests := []struct {
		name       string
		giveV      []float64
		giveLength int
		want       float64
	}{
		{
			name:       "Valid sample",
			giveV:      []float64{10, 89, 20, 43, 44, 33, 19},
			giveLength: 3,
			want:       32.680479063242394,
		},
		{
			name:       "0 length window",
			giveV:      []float64{10, 89, 20, 43, 44, 33, 19},
			giveLength: 0,
			want:       19,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := types.NewFloat64Series()
			alma := ALMA(source, tt.giveLength)

			for _, d := range tt.giveV {
				source.PushAndEmit(d)
			}
			assert.Equal(t, tt.want, alma.Last(0))
		})
	}

}
