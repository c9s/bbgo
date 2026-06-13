package widget

import (
	"fmt"
	"image"
	"math"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	ui "github.com/gizak/termui/v3"
)

const brailleOffset = '\u2800'

var brailleDots = [4][2]rune{
	{0x01, 0x08},
	{0x02, 0x10},
	{0x04, 0x20},
	{0x40, 0x80},
}

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
	levels     []PriceLevel
	indicesMap map[fixedpoint.Value]int
}

func (w *OrderFlowWidget) SetLevels(levels []PriceLevel) {
	w.levels = levels
	for i, lvl := range levels {
		w.indicesMap[lvl.Price] = i
	}
}

func (w *OrderFlowWidget) UpdateTrade(trade types.Trade) {
	idx, ok := w.indicesMap[trade.Price]
	if !ok {
		newLevel := PriceLevel{
			Price: trade.Price,
		}
		if trade.Side == types.SideTypeBuy {
			newLevel.BidVol = trade.Quantity
		} else {
			newLevel.AskVol = trade.Quantity
		}
		w.levels = append(w.levels, newLevel)
		w.indicesMap[trade.Price] = len(w.levels) - 1
	} else {
		if trade.Side == types.SideTypeBuy {
			w.levels[idx].BidVol = w.levels[idx].BidVol.Add(trade.Quantity)
		} else {
			w.levels[idx].AskVol = w.levels[idx].AskVol.Add(trade.Quantity)
		}
	}
}

func NewOrderFlowWidget() *OrderFlowWidget {
	return &OrderFlowWidget{
		Block:      *ui.NewBlock(),
		indicesMap: make(map[fixedpoint.Value]int),
	}
}

func (w *OrderFlowWidget) Draw(buf *ui.Buffer) {
	w.Block.Draw(buf)

	if len(w.levels) == 0 {
		return
	}

	maxVol := fixedpoint.Zero
	totalDelta := fixedpoint.Zero
	pocPrice := fixedpoint.Zero
	pocVol := fixedpoint.Zero
	for _, lvl := range w.levels {
		if v := lvl.TotalVol(); v.Compare(maxVol) > 0 {
			maxVol = v
			pocPrice = lvl.Price
			pocVol = v
		}
		totalDelta = totalDelta.Add(lvl.Delta())
	}
	if maxVol.IsZero() {
		return
	}

	inner := w.Inner
	numLevels := len(w.levels)

	priceColWidth := 8
	infoColWidth := 16
	circleStart := inner.Min.X + priceColWidth
	circleEnd := inner.Max.X - infoColWidth
	circleWidth := circleEnd - circleStart
	if circleWidth < 4 {
		return
	}

	availableHeight := inner.Max.Y - inner.Min.Y - 2
	rowHeight := availableHeight / numLevels
	if rowHeight < 2 {
		rowHeight = 2
	}

	maxRadius := math.Min(float64(circleWidth), float64(rowHeight*2)) * 0.85

	for i, lvl := range w.levels {
		rowTopY := inner.Min.Y + i*rowHeight
		rowCenterY := rowTopY + rowHeight/2
		if rowCenterY >= inner.Max.Y-2 {
			break
		}

		delta := lvl.Delta()
		totalVol := lvl.TotalVol()
		ratio := totalVol.Div(maxVol).Float64()
		radius := ratio * maxRadius
		if radius < 1.5 {
			radius = 1.5
		}

		color := ui.ColorGreen
		if delta < 0 {
			color = ui.ColorRed
		} else if delta == 0 {
			color = ui.ColorYellow
		}

		priceStyle := ui.NewStyle(ui.ColorWhite)
		if lvl.Price.Eq(pocPrice) {
			priceStyle = ui.NewStyle(ui.ColorYellow, ui.ColorClear, ui.ModifierBold)
		}
		buf.SetString(fmt.Sprintf("%.5f", lvl.Price.Float64()), priceStyle, image.Pt(inner.Min.X, rowCenterY))

		cxBraille := float64(circleWidth)
		cyBraille := float64(rowHeight*4) / 2.0

		for ty := 0; ty < rowHeight; ty++ {
			termY := rowTopY + ty
			if termY >= inner.Max.Y-2 {
				break
			}
			for tx := 0; tx < circleWidth; tx++ {
				termX := circleStart + tx
				var brailleRune rune
				for sy := 0; sy < 4; sy++ {
					for sx := 0; sx < 2; sx++ {
						bx := float64(tx*2 + sx)
						by := float64(ty*4 + sy)
						dx := bx - cxBraille
						dy := by - cyBraille
						if dx*dx+dy*dy <= radius*radius {
							brailleRune |= brailleDots[sy][sx]
						}
					}
				}
				if brailleRune != 0 {
					buf.SetCell(ui.Cell{
						Rune:  brailleOffset + brailleRune,
						Style: ui.NewStyle(color),
					}, image.Pt(termX, termY))
				}
			}
		}

		deltaStr := fmt.Sprintf(" %.4f", delta.Float64())
		buf.SetString(deltaStr, ui.NewStyle(color), image.Pt(circleEnd-9, rowCenterY))

		volStr := fmt.Sprintf(" V:%.5f", totalVol.Float64())
		buf.SetString(volStr, ui.NewStyle(ui.ColorCyan), image.Pt(circleEnd, rowCenterY))
	}

	summaryY := inner.Max.Y - 1
	deltaColor := ui.ColorGreen
	if totalDelta < 0 {
		deltaColor = ui.ColorRed
	}
	summary := fmt.Sprintf(
		" Total Δ: %.4f | POC: %.5f (Vol %.5f)",
		totalDelta.Float64(), pocPrice.Float64(), pocVol.Float64())
	buf.SetString(summary, ui.NewStyle(deltaColor), image.Pt(inner.Min.X, summaryY))
	buf.SetString(" [q] quit", ui.NewStyle(ui.ColorWhite), image.Pt(inner.Max.X-10, summaryY))
}
