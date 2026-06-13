package widget

import (
	"fmt"
	"image"
	"strings"
	"sync"
	"testing"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	ui "github.com/gizak/termui/v3"
	"github.com/stretchr/testify/assert"
)

func TestOrderFlowWidget_UpdateTradeAggregatesByPriceAndSide(t *testing.T) {
	w := NewOrderFlowWidget()

	const n = 50
	for price := int64(1); price <= n; price++ {
		w.UpdateTrade(types.Trade{
			Price:    fixedpoint.NewFromInt(price),
			Quantity: fixedpoint.One,
			Side:     types.SideTypeBuy,
		})
	}

	// A repeated price merges into the existing level instead of adding one.
	w.UpdateTrade(types.Trade{
		Price:    fixedpoint.NewFromInt(15),
		Quantity: fixedpoint.One,
		Side:     types.SideTypeBuy,
	})
	w.UpdateTrade(types.Trade{
		Price:    fixedpoint.NewFromInt(15),
		Quantity: fixedpoint.NewFromInt(3),
		Side:     types.SideTypeSell,
	})

	levels := w.sortedLevelsSnapshot()
	assert.Len(t, levels, n)
	assert.True(t, hasPriceLevel(levels, fixedpoint.NewFromInt(1)))
	assert.True(t, hasPriceLevel(levels, fixedpoint.NewFromInt(n)))

	level := requirePriceLevel(t, levels, fixedpoint.NewFromInt(15))
	assert.Equal(t, fixedpoint.NewFromInt(2), level.BidVol)
	assert.Equal(t, fixedpoint.NewFromInt(3), level.AskVol)

	// Sorting is a render-time concern; the sorted snapshot is ordered by
	// descending price without mutating the collected levels.
	for idx := 1; idx < len(levels); idx++ {
		assert.GreaterOrEqual(t, levels[idx-1].Price.Compare(levels[idx].Price), 0)
	}
}

// At most maxOrderFlowSideLevels levels are shown above and below the POC.
func TestOrderFlowWidget_DrawWindowsAroundPOC(t *testing.T) {
	w := NewOrderFlowWidget()

	const pocIdx = 20
	for i := 0; i < 41; i++ {
		qty := fixedpoint.One
		if i == pocIdx {
			qty = fixedpoint.NewFromInt(50) // unambiguous POC
		}
		w.UpdateTrade(types.Trade{
			Price:    priceFromIndex(i),
			Quantity: qty,
			Side:     types.SideTypeBuy,
		})
	}

	// Tall enough that height is not the limiter (21 rows × 2).
	w.SetRect(0, 0, 80, 46)
	buf := ui.NewBuffer(image.Rect(0, 0, 80, 46))
	w.Draw(buf)

	text := bufferText(buf)
	assert.Contains(t, text, priceLabelFromIndex(pocIdx-maxOrderFlowSideLevels))
	assert.Contains(t, text, priceLabelFromIndex(pocIdx+maxOrderFlowSideLevels))
	assert.NotContains(t, text, priceLabelFromIndex(pocIdx-maxOrderFlowSideLevels-1))
	assert.NotContains(t, text, priceLabelFromIndex(pocIdx+maxOrderFlowSideLevels+1))

	// 41 levels, POC centered: 10 windowed off each side of the visible range.
	assert.Contains(t, text, "hidden ↑10 ↓10")
}

func TestOrderFlowWidget_DrawEmptyDoesNotPanic(t *testing.T) {
	w := NewOrderFlowWidget()
	w.SetRect(0, 0, 80, 40)

	buf := ui.NewBuffer(image.Rect(0, 0, 80, 40))
	assert.NotPanics(t, func() { w.Draw(buf) })
}

// UpdateTrade is fed from a stream callback while Draw runs on the render loop;
// the mutex exists to make that safe. Run under -race to exercise it.
func TestOrderFlowWidget_ConcurrentUpdateAndDraw(t *testing.T) {
	w := NewOrderFlowWidget()
	w.SetRect(0, 0, 80, 40)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			w.UpdateTrade(types.Trade{
				Price:    priceFromIndex(i % 40),
				Quantity: fixedpoint.One,
				Side:     types.SideTypeBuy,
			})
		}
	}()

	go func() {
		defer wg.Done()
		buf := ui.NewBuffer(image.Rect(0, 0, 80, 40))
		for i := 0; i < 1000; i++ {
			w.Draw(buf)
		}
	}()

	wg.Wait()
}

// The POC must stay on screen when the terminal cannot fit every level and
// prices are clustered.
func TestOrderFlowWidget_DrawKeepsPOCVisible(t *testing.T) {
	w := NewOrderFlowWidget()

	pocPrice := priceFromIndex(15)
	for i := 0; i < 30; i++ {
		price := priceFromIndex(i)
		qty := fixedpoint.One
		if price.Eq(pocPrice) {
			qty = fixedpoint.NewFromInt(50) // unambiguous POC
		}
		w.UpdateTrade(types.Trade{Price: price, Quantity: qty, Side: types.SideTypeBuy})
	}

	// Too short for 30 levels at 2 rows each.
	w.SetRect(0, 0, 80, 14)

	buf := ui.NewBuffer(image.Rect(0, 0, 80, 14))
	w.Draw(buf)

	assert.Contains(t, bufferText(buf), priceLabelFromIndex(15),
		"POC price must be rendered even when the terminal cannot fit every level")
}

func priceFromIndex(i int) fixedpoint.Value {
	return fixedpoint.MustNewFromString(fmt.Sprintf("100.%02d", i))
}

func priceLabelFromIndex(i int) string {
	return fmt.Sprintf("100.%02d000", i)
}

// bufferText flattens a termui buffer into one line per row.
func bufferText(buf *ui.Buffer) string {
	r := buf.Rectangle
	var lines []string
	for y := r.Min.Y; y < r.Max.Y; y++ {
		var sb strings.Builder
		for x := r.Min.X; x < r.Max.X; x++ {
			cell := buf.GetCell(image.Pt(x, y))
			if cell.Rune == 0 {
				sb.WriteRune(' ')
				continue
			}
			sb.WriteRune(cell.Rune)
		}
		lines = append(lines, sb.String())
	}
	return strings.Join(lines, "\n")
}

func hasPriceLevel(levels []PriceLevel, price fixedpoint.Value) bool {
	for _, lvl := range levels {
		if lvl.Price.Eq(price) {
			return true
		}
	}
	return false
}

func requirePriceLevel(t *testing.T, levels []PriceLevel, price fixedpoint.Value) PriceLevel {
	t.Helper()

	for _, lvl := range levels {
		if lvl.Price.Eq(price) {
			return lvl
		}
	}

	t.Fatalf("missing price level %s", price.String())
	return PriceLevel{}
}
