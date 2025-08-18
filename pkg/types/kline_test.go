package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKLineWindow_Tail(t *testing.T) {
	var jsonWin = []byte(`[
      {"open": 11600.0, "close": 11600.0, "high": 11600.0, "low": 11600.0},
	  {"open": 11700.0, "close": 11700.0, "high": 11700.0, "low": 11700.0}
	]`)
	var win KLineWindow
	err := json.Unmarshal(jsonWin, &win)
	assert.NoError(t, err)

	/*{
		{Open: 11600.0, Close: 11600.0, High: 11600.0, Low: 11600.0},
		{Open: 11700.0, Close: 11700.0, High: 11700.0, Low: 11700.0},
	}*/

	var win2 = win.Tail(1)
	assert.Len(t, win2, 1)
	assert.ElementsMatch(t, win2, win[1:])

	var win3 = win.Tail(2)
	assert.Len(t, win3, 2)
	assert.ElementsMatch(t, win3, win)

	var win4 = win.Tail(3)
	assert.Len(t, win4, 2)
	assert.ElementsMatch(t, win4, win)
}

func TestKLineWindow_Truncate(t *testing.T) {
	var jsonWin = []byte(`[
      {"open": 11600.0, "close": 11600.0, "high": 11600.0, "low": 11600.0},
	  {"open": 11601.0, "close": 11600.0, "high": 11600.0, "low": 11600.0},
	  {"open": 11602.0, "close": 11600.0, "high": 11600.0, "low": 11600.0},
	  {"open": 11603.0, "close": 11600.0, "high": 11600.0, "low": 11600.0}
	]`)
	var win KLineWindow
	err := json.Unmarshal(jsonWin, &win)
	assert.NoError(t, err)

	win.Truncate(5)
	assert.Len(t, win, 4)
	assert.Equal(t, 11603.0, win.Last().Open.Float64())

	win.Truncate(3)
	assert.Len(t, win, 3)
	assert.Equal(t, 11603.0, win.Last().Open.Float64())

	win.Truncate(1)
	assert.Len(t, win, 1)
	assert.Equal(t, 11603.0, win.Last().Open.Float64())
}

func TestShrinkSlice(t *testing.T) {
	var slice = make([]float64, 0, 100)
	for i := 0; i < 50000; i++ {
		slice = append(slice, float64(i))
	}

	slice2 := ShrinkSlice(slice, 40000, 10000)
	assert.NotEqual(t, &slice2, &slice)
	assert.Equal(t, 10000, len(slice2))
}
