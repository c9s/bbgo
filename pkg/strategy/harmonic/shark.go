package harmonic

import (
	"math"
	"time"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

var zeroTime time.Time

//go:generate callbackgen -type SHARK
type SHARK struct {
	types.IntervalWindow
	types.SeriesBase

	Lows        floats.Slice
	Highs       floats.Slice
	LongScores  floats.Slice
	ShortScores floats.Slice

	Values floats.Slice

	EndTime time.Time

	updateCallbacks []func(value float64)
}

var _ types.SeriesExtend = &SHARK{}

func (inc *SHARK) Update(high, low, price float64) {
	if inc.SeriesBase.Series == nil {
		inc.SeriesBase.Series = inc
	}
	inc.Highs.Update(high)
	inc.Lows.Update(low)

	if inc.Highs.Length() < inc.Window || inc.Lows.Length() < inc.Window {
		return
	}

	longScore := inc.SharkLong(inc.Highs, inc.Lows, price, inc.Window)
	shortScore := inc.SharkShort(inc.Highs, inc.Lows, price, inc.Window)

	inc.LongScores.Push(longScore)
	inc.ShortScores.Push(shortScore)

	inc.Values.Push(longScore - shortScore)

}

func (inc *SHARK) Last(i int) float64 {
	return inc.Values.Last(i)
}

func (inc *SHARK) Index(i int) float64 {
	return inc.Last(i)
}

func (inc *SHARK) Length() int {
	return len(inc.Values)
}

func (inc *SHARK) BindK(target indicator.KLineClosedEmitter, symbol string, interval types.Interval) {
	target.OnKLineClosed(types.KLineWith(symbol, interval, inc.PushK))
}

func (inc *SHARK) PushK(k types.KLine) {
	if inc.EndTime != zeroTime && k.EndTime.Before(inc.EndTime) {
		return
	}

	inc.Update(types.KLineHighPriceMapper(k), types.KLineLowPriceMapper(k), types.KLineClosePriceMapper(k))
	inc.EndTime = k.EndTime.Time()
	inc.EmitUpdate(inc.Last(0))
}

func (inc *SHARK) LoadK(allKLines []types.KLine) {
	for _, k := range allKLines {
		inc.PushK(k)
	}
	inc.EmitUpdate(inc.Last(0))
}

func (inc SHARK) SharkLong(highs, lows floats.Slice, p float64, lookback int) float64 {
	score := 0.
	for x := 5; x < lookback; x++ {
		if lows.Index(x-1) > lows.Index(x) && lows.Index(x) < lows.Index(x+1) {
			X := lows.Index(x)
			for a := 4; a < x; a++ {
				if highs.Index(a-1) < highs.Index(a) && highs.Index(a) > highs.Index(a+1) {
					A := highs.Index(a)
					XA := math.Abs(X - A)
					hB := A - 0.382*XA
					lB := A - 0.618*XA
					for b := 3; b < a; b++ {
						if lows.Index(b-1) > lows.Index(b) && lows.Index(b) < lows.Index(b+1) {
							B := lows.Index(b)
							if hB > B && B > lB {
								// log.Infof("got point B:%f", B)
								AB := math.Abs(A - B)
								hC := B + 1.618*AB
								lC := B + 1.13*AB
								for c := 2; c < b; c++ {
									if highs.Index(c-1) < highs.Index(c) && highs.Index(c) > highs.Index(c+1) {
										C := highs.Index(c)
										if hC > C && C > lC {
											// log.Infof("got point C:%f", C)
											XC := math.Abs(X - C)
											hD := C - 0.886*XC
											lD := C - 1.13*XC
											// for d := 1; d < c; d++ {
											// if lows.Index(d-1) > lows.Index(d) && lows.Index(d) < lows.Index(d+1) {
											D := p // lows.Index(d)
											if hD > D && D > lD {
												BC := math.Abs(B - C)
												hD2 := C - 1.618*BC
												lD2 := C - 2.24*BC
												if hD2 > D && D > lD2 {
													// log.Infof("got point D:%f", D)
													score++
												}
											}
											// }
											// }
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}
	return score
}

func (inc SHARK) SharkShort(highs, lows floats.Slice, p float64, lookback int) float64 {
	score := 0.
	for x := 5; x < lookback; x++ {
		if highs.Index(x-1) < highs.Index(x) && highs.Index(x) > highs.Index(x+1) {
			X := highs.Index(x)
			for a := 4; a < x; a++ {
				if lows.Index(a-1) > lows.Index(a) && lows.Index(a) < lows.Index(a+1) {
					A := lows.Index(a)
					XA := math.Abs(X - A)
					lB := A + 0.382*XA
					hB := A + 0.618*XA
					for b := 3; b < a; b++ {
						if highs.Index(b-1) > highs.Index(b) && highs.Index(b) < highs.Index(b+1) {
							B := highs.Index(b)
							if hB > B && B > lB {
								// log.Infof("got point B:%f", B)
								AB := math.Abs(A - B)
								lC := B - 1.618*AB
								hC := B - 1.13*AB
								for c := 2; c < b; c++ {
									if lows.Index(c-1) < lows.Index(c) && lows.Index(c) > lows.Index(c+1) {
										C := lows.Index(c)
										if hC > C && C > lC {
											// log.Infof("got point C:%f", C)
											XC := math.Abs(X - C)
											lD := C + 0.886*XC
											hD := C + 1.13*XC
											// for d := 1; d < c; d++ {
											// if lows.Index(d-1) > lows.Index(d) && lows.Index(d) < lows.Index(d+1) {
											D := p // lows.Index(d)
											if hD > D && D > lD {
												BC := math.Abs(B - C)
												lD2 := C + 1.618*BC
												hD2 := C + 2.24*BC
												if hD2 > D && D > lD2 {
													// log.Infof("got point D:%f", D)
													score++
												}
											}
											// }
											// }
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}
	return score
}
