package widget

import (
	"image"

	ui "github.com/gizak/termui/v3"
)

const brailleOffset = '\u2800'

var brailleDots = [4][2]rune{
	{0x01, 0x08},
	{0x02, 0x10},
	{0x04, 0x20},
	{0x40, 0x80},
}

func DrawCircle(buffer *ui.Buffer, center image.Point, radius int, style ui.Style) {
	if radius <= 0 {
		return
	}

	cx := float64(center.X * 2)
	cy := float64(center.Y * 4)
	r2 := float64(radius * radius)

	cellRadiusX := radius/2 + 1
	cellRadiusY := radius/4 + 1

	for cellY := center.Y - cellRadiusY; cellY <= center.Y+cellRadiusY; cellY++ {
		for cellX := center.X - cellRadiusX; cellX <= center.X+cellRadiusX; cellX++ {
			var brailleRune rune
			for sy := 0; sy < 4; sy++ {
				for sx := 0; sx < 2; sx++ {
					dx := float64(cellX*2+sx) - cx
					dy := float64(cellY*4+sy) - cy
					if dx*dx+dy*dy <= r2 {
						brailleRune |= brailleDots[sy][sx]
					}
				}
			}
			if brailleRune == 0 {
				continue
			}

			pt := image.Pt(cellX, cellY)
			if existing := buffer.GetCell(pt).Rune; existing >= brailleOffset && existing < brailleOffset+0x100 {
				brailleRune |= existing - brailleOffset
			}
			buffer.SetCell(ui.Cell{
				Rune:  brailleOffset + brailleRune,
				Style: style,
			}, pt)
		}
	}
}

// DrawHorizontalBar draws a horizontal bar from xstart to xend at the given y coordinate.
func DrawHorizontalBar(buffer *ui.Buffer, xstart, xend int, y int, style ui.Style) {
	if xend < xstart {
		xstart, xend = xend, xstart
	}
	for x := xstart; x <= xend; x++ {
		buffer.SetCell(
			ui.NewCell(ui.SHADED_BLOCKS[1], style),
			image.Pt(x, y),
		)
	}
}
