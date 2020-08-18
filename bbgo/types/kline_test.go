package types

import ("testing"

	"github.com/stretchr/testify/assert"
)

func TestKLineWindow_Tail(t *testing.T) {
	var win = KLineWindow{
		{ Open: "11600.0", Close: "11600.0", High: "11600.0", Low: "11600.0"},
		{ Open: "11600.0", Close: "11600.0", High: "11600.0", Low: "11600.0"},
	}

	var win2 = win.Tail(1)
	assert.Len(t, win2, 1)

	var win3 = win.Tail(2)
	assert.Len(t, win3, 2)

	var win4 = win.Tail(3)
	assert.Len(t, win4, 2)
}
