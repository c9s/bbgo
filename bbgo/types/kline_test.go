package types

import ("testing"

	"github.com/stretchr/testify/assert"
)

func TestKLineWindow_Tail(t *testing.T) {
	var win = KLineWindow{
		{ Open: 11600.0, Close: 11600.0, High: 11600.0, Low: 11600.0},
		{ Open: 11600.0, Close: 11600.0, High: 11600.0, Low: 11600.0},
	}

	var win2 = win.Tail(1)
	assert.Len(t, win2, 1)

	var win3 = win.Tail(2)
	assert.Len(t, win3, 2)

	var win4 = win.Tail(3)
	assert.Len(t, win4, 2)
}

func TestKLineWindow_Truncate(t *testing.T) {
	var win = KLineWindow{
		{ Open: 11600.0, Close: 11600.0, High: 11600.0, Low: 11600.0},
		{ Open: 11601.0, Close: 11600.0, High: 11600.0, Low: 11600.0},
		{ Open: 11602.0, Close: 11600.0, High: 11600.0, Low: 11600.0},
		{ Open: 11603.0, Close: 11600.0, High: 11600.0, Low: 11600.0},
	}

	win.Truncate(5)
	assert.Len(t, win, 4)
	assert.Equal(t, 11603.0, win.Last().Open)

	win.Truncate(3)
	assert.Len(t, win, 3)
	assert.Equal(t, 11603.0, win.Last().Open)


	win.Truncate(1)
	assert.Len(t, win, 1)
	assert.Equal(t, 11603.0, win.Last().Open)
}
