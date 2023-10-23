// Copyright 2022 The Coln Group Ltd
// SPDX-License-Identifier: MIT

package indicatorv2

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLookback(t *testing.T) {
	tests := []struct {
		name       string
		giveSeries []float64
		giveN      int
		want       float64
	}{
		{
			name:       "Last value",
			giveSeries: []float64{0, 1, 2, 3, 4, 5},
			giveN:      0,
			want:       5,
		},
		{
			name:       "First value",
			giveSeries: []float64{0, 1, 2, 3, 4, 5},
			giveN:      6,
			want:       0,
		},
		{
			name:       "Value",
			giveSeries: []float64{0, 1, 2, 3, 4, 5},
			giveN:      2,
			want:       3,
		},
		{
			name:       "Index out of range return 0",
			giveSeries: []float64{0, 1, 2, 3, 4, 5},
			giveN:      7,
			want:       0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			act := Lookback(tt.giveSeries, tt.giveN)
			assert.Equal(t, tt.want, act)
		})
	}
}

func TestWindow(t *testing.T) {
	tests := []struct {
		name       string
		giveSeries []float64
		giveN      int
		want       []float64
	}{
		{
			name:       "Latest value only",
			giveSeries: []float64{0, 1, 2, 3, 4, 5},
			giveN:      0,
			want:       []float64{5},
		},
		{
			name:       "All values",
			giveSeries: []float64{0, 1, 2, 3, 4, 5},
			giveN:      6,
			want:       []float64{0, 1, 2, 3, 4, 5},
		},
		{
			name:       "Index out of range - returns all values",
			giveSeries: []float64{0, 1, 2, 3, 4, 5},
			giveN:      7,
			want:       []float64{0, 1, 2, 3, 4, 5},
		},
		{
			name:       "Sub window",
			giveSeries: []float64{0, 1, 2, 3, 4, 5},
			giveN:      3,
			want:       []float64{2, 3, 4, 5},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			act := Window(tt.giveSeries, tt.giveN)
			assert.Equal(t, tt.want, act)
		})
	}
}

func TestWindowAppend(t *testing.T) {
	tests := []struct {
		name       string
		giveSeries []float64
		giveN      int
		giveV      float64
		want       []float64
	}{
		{
			name:       "Append and no slice",
			giveSeries: []float64{0, 1, 2, 3, 4, 5},
			giveN:      7,
			giveV:      99,
			want:       []float64{0, 1, 2, 3, 4, 5, 99},
		},
		{
			name:       "Append and slice",
			giveSeries: []float64{0, 1, 2, 3, 4, 5},
			giveN:      2,
			giveV:      99,
			want:       []float64{4, 5, 99},
		},
		{
			name:       "Append and slice to new value only",
			giveSeries: []float64{0, 1, 2, 3, 4, 5},
			giveN:      0,
			giveV:      99,
			want:       []float64{99},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			act := WindowAppend(tt.giveSeries, tt.giveN, tt.giveV)
			assert.Equal(t, tt.want, act)
		})
	}
}
