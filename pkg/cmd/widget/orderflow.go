package widget

import (
	"fmt"
	"image"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	ui "github.com/gizak/termui/v3"
)

// maxOrderFlowSideLevels caps how many price levels are shown above and below
// the POC. All traded prices are retained; only the display is windowed.
const maxOrderFlowSideLevels = 10

const (
	priceColWidth = 13
	deltaColWidth = 10
	volColWidth   = 16
)

type PriceLevel struct {
	Price  fixedpoint.Value
	AskVol fixedpoint.Value
	BidVol fixedpoint.Value
}

func (p *PriceLevel) Delta() fixedpoint.Value {
	return p.AskVol.Sub(p.BidVol)
}

func (p *PriceLevel) TotalVol() fixedpoint.Value {
	return p.AskVol.Add(p.BidVol)
}

type OrderFlowWidget struct {
	ui.Block
	mu        sync.Mutex
	levels    []PriceLevel
	lastTrade *types.Trade
	startTime time.Time
}

func (w *OrderFlowWidget) UpdateTrade(trade types.Trade) {
	level := PriceLevel{
		Price: trade.Price,
	}
	if trade.Side == types.SideTypeBuy {
		level.BidVol = trade.Quantity
	} else {
		level.AskVol = trade.Quantity
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	w.upsertLevel(level)
	t := trade
	w.lastTrade = &t
}

func (w *OrderFlowWidget) upsertLevel(level PriceLevel) {
	for idx := range w.levels {
		if !w.levels[idx].Price.Eq(level.Price) {
			continue
		}
		w.levels[idx].BidVol = w.levels[idx].BidVol.Add(level.BidVol)
		w.levels[idx].AskVol = w.levels[idx].AskVol.Add(level.AskVol)
		return
	}

	w.levels = append(w.levels, level)
}

func (w *OrderFlowWidget) sortedLevelsSnapshot() ([]PriceLevel, *types.Trade) {
	w.mu.Lock()
	levels := append([]PriceLevel(nil), w.levels...)
	lastTrade := w.lastTrade
	w.mu.Unlock()

	sort.Slice(levels, func(i, j int) bool {
		return levels[i].Price.Compare(levels[j].Price) > 0
	})

	return levels, lastTrade
}

func NewOrderFlowWidget() *OrderFlowWidget {
	return &OrderFlowWidget{
		Block: *ui.NewBlock(),

		startTime: time.Now(),
	}
}

func (w *OrderFlowWidget) Draw(buf *ui.Buffer) {
	w.Block.Draw(buf)

	levels, lastTrade := w.sortedLevelsSnapshot()
	if len(levels) == 0 {
		return
	}

	maxVol := fixedpoint.Zero
	totalDelta := fixedpoint.Zero
	pocPrice := fixedpoint.Zero
	pocIdx := 0
	for idx, lvl := range levels {
		if v := lvl.TotalVol(); v.Compare(maxVol) > 0 {
			maxVol = v
			pocPrice = lvl.Price
			pocIdx = idx
		}
		totalDelta = totalDelta.Add(lvl.Delta())
	}
	if maxVol.IsZero() {
		return
	}

	inner := w.Inner

	infoColWidth := deltaColWidth + volColWidth
	barStart := inner.Min.X + priceColWidth
	barEnd := inner.Max.X - infoColWidth
	barWidth := barEnd - barStart
	if barWidth < 4 {
		return
	}
	barSideGap := w.Inner.Dx() / 8
	barCenterX := barStart + barWidth/2
	maxBarHalfWidth := barWidth/2 - barSideGap
	if maxBarHalfWidth < 1 {
		maxBarHalfWidth = 1
	}

	availableHeight := inner.Max.Y - inner.Min.Y - 2

	// Show at most maxOrderFlowSideLevels above and below the POC, shrinking
	// further if the terminal cannot fit that many rows.
	const minRowHeight = 2
	maxVisible := availableHeight / minRowHeight
	if maxVisible < 1 {
		return
	}

	side := maxOrderFlowSideLevels
	if half := (maxVisible - 1) / 2; half < side {
		side = half
	}

	start := pocIdx - side
	if start < 0 {
		start = 0
	}
	end := pocIdx + side + 1
	if end > len(levels) {
		end = len(levels)
	}
	visibleLevels := levels[start:end]
	hiddenBelow := len(levels) - end

	numLevels := len(visibleLevels)
	rowHeight := availableHeight / numLevels
	if rowHeight < minRowHeight {
		rowHeight = minRowHeight
	}

	// Price range of the visible levels, used to position rows proportionally.
	minPrice := visibleLevels[0].Price
	maxPrice := visibleLevels[0].Price
	for _, lvl := range visibleLevels {
		if lvl.Price.Compare(minPrice) < 0 {
			minPrice = lvl.Price
		}
		if lvl.Price.Compare(maxPrice) > 0 {
			maxPrice = lvl.Price
		}
	}
	priceRange := maxPrice.Sub(minPrice)

	// Band the rows may occupy: half a row of margin top and bottom and three
	// rows reserved at the bottom for the summary.
	drawMinY := inner.Min.Y + rowHeight/2
	drawMaxY := inner.Max.Y - 3 - rowHeight/2
	if drawMaxY < drawMinY {
		drawMinY = inner.Min.Y
		drawMaxY = inner.Max.Y - 3
	}
	if drawMaxY < drawMinY {
		return
	}

	// Place every row at its proportional ideal, then de-overlap so clustered
	// prices that round to the same Y keep a one-row gap. maxVisible caps the
	// count so the gaps always fit: no level (the POC included) is ever dropped.
	const minGap = 1
	rowCenterYs := make([]int, numLevels)
	for i, lvl := range visibleLevels {
		y := (drawMinY + drawMaxY) / 2
		if !priceRange.IsZero() {
			priceOffset := maxPrice.Sub(lvl.Price).Div(priceRange).Float64()
			y = drawMinY + int(math.Round(priceOffset*float64(drawMaxY-drawMinY)))
		}
		rowCenterYs[i] = y
	}
	// Pass 1 (top→bottom): push colliding rows down.
	for i := 1; i < numLevels; i++ {
		if lo := rowCenterYs[i-1] + minGap; rowCenterYs[i] < lo {
			rowCenterYs[i] = lo
		}
	}
	// Pass 2 (bottom→top): if the tail overran the band, pin it and pull back up.
	if rowCenterYs[numLevels-1] > drawMaxY {
		rowCenterYs[numLevels-1] = drawMaxY
	}
	for i := numLevels - 2; i >= 0; i-- {
		if hi := rowCenterYs[i+1] - minGap; rowCenterYs[i] > hi {
			rowCenterYs[i] = hi
		}
	}

	maxAbsDelta := fixedpoint.Zero
	for _, lvl := range visibleLevels {
		if d := lvl.Delta().Abs(); d.Compare(maxAbsDelta) > 0 {
			maxAbsDelta = d
		}
	}

	for i, lvl := range visibleLevels {
		rowCenterY := rowCenterYs[i]

		delta := lvl.Delta()
		totalVol := lvl.TotalVol()

		color := ui.ColorGreen
		if delta.Sign() < 0 {
			color = ui.ColorRed
		} else if delta.IsZero() {
			color = ui.ColorWhite
		}

		halfWidth := 0
		if !maxAbsDelta.IsZero() {
			halfWidth = int(math.Round(delta.Abs().Div(maxAbsDelta).Float64() * float64(maxBarHalfWidth)))
		}
		if halfWidth < 1 && !delta.IsZero() {
			halfWidth = 1
		}
		if halfWidth > 0 {
			DrawHorizontalBar(buf, barCenterX-halfWidth, barCenterX+halfWidth, rowCenterY, ui.NewStyle(color))
		}

		priceStyle := ui.NewStyle(ui.ColorWhite)
		if lvl.Price.Eq(pocPrice) {
			priceStyle = ui.NewStyle(ui.ColorYellow, ui.ColorClear, ui.ModifierBold)
		}
		setPaddedString(buf, fmt.Sprintf("%.5f", lvl.Price.Float64()), priceStyle, image.Pt(inner.Min.X, rowCenterY), priceColWidth)

		deltaStr := fmt.Sprintf("% .4f", delta.Float64())
		setPaddedString(buf, deltaStr, ui.NewStyle(color), image.Pt(barEnd, rowCenterY), deltaColWidth)

		volStr := fmt.Sprintf(" V:%.5f", totalVol.Float64())
		setPaddedString(buf, volStr, ui.NewStyle(ui.ColorCyan), image.Pt(barEnd+deltaColWidth, rowCenterY), volColWidth)
	}

	summaryY := inner.Max.Y - 1
	deltaColor := ui.ColorGreen
	if totalDelta.Sign() < 0 {
		deltaColor = ui.ColorRed
	}
	summary := fmt.Sprintf(
		" Total Δ: %.4f | POC: %.5f (Vol %.5f)",
		totalDelta.Float64(), pocPrice.Float64(), maxVol.Float64())
	if lastTrade != nil {
		pocRel := "= POC"
		switch lastTrade.Price.Compare(pocPrice) {
		case 1:
			pocRel = "↑ POC"
		case -1:
			pocRel = "↓ POC"
		}
		summary += fmt.Sprintf(" | Last Trade: %s %s@%.5f (%s)",
			lastTrade.Side,
			lastTrade.Quantity,
			lastTrade.Price.Float64(),
			pocRel,
		)
	}
	if start > 0 || hiddenBelow > 0 {
		summary += fmt.Sprintf(" | hidden ↑%d ↓%d", start, hiddenBelow)
	}
	elapsedTime := time.Since(w.startTime).Truncate(time.Second)
	buf.SetString(fmt.Sprintf(
		" Elapsed: %s (since %s)",
		elapsedTime, w.startTime.Format(time.RFC3339)),
		ui.NewStyle(deltaColor),
		image.Pt(inner.Min.X, summaryY-1),
	)
	buf.SetString(summary, ui.NewStyle(deltaColor), image.Pt(inner.Min.X, summaryY))
	buf.SetString(" [q] quit", ui.NewStyle(ui.ColorWhite), image.Pt(inner.Max.X-10, summaryY))
}

func setPaddedString(buf *ui.Buffer, s string, style ui.Style, point image.Point, width int) {
	if len(s) > width {
		s = s[:width]
	}
	buf.SetString(fmt.Sprintf("%-*s", width, s), style, point)
}
