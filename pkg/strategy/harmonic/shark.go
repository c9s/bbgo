package harmonic

import (
	"math"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

// SHARK is the harmonic shark pattern score indicator.
// The values are the long score minus the short score of the shark pattern found in the last window klines,
// they are pushed once window klines have been collected.
type SHARK struct {
	*types.Float64Series

	Lows        floats.Slice
	Highs       floats.Slice
	LongScores  floats.Slice
	ShortScores floats.Slice

	window int
}

// NewSHARK creates the SHARK stream of the given kline source
func NewSHARK(source indicatorv2.KLineSubscription, window int) *SHARK {
	inc := &SHARK{
		Float64Series: types.NewFloat64Series(),
		window:        window,
	}

	source.AddSubscriber(func(k types.KLine) {
		inc.Update(k.High.Float64(), k.Low.Float64(), k.Close.Float64())
	})

	return inc
}

func (inc *SHARK) Update(high, low, price float64) {
	inc.Highs.Update(high)
	inc.Lows.Update(low)

	if inc.Highs.Length() < inc.window || inc.Lows.Length() < inc.window {
		return
	}

	longScore := inc.SharkLong(inc.Highs, inc.Lows, price, inc.window)
	shortScore := inc.SharkShort(inc.Highs, inc.Lows, price, inc.window)

	inc.LongScores.Push(longScore)
	inc.ShortScores.Push(shortScore)

	inc.PushAndEmit(longScore - shortScore)
	inc.truncate()
}

func (inc *SHARK) truncate() {
	// SharkLong and SharkShort read the last window+1 highs and lows
	threshold := max(indicatorv2.MaxSliceSize, 4*(inc.window+2))
	size := max(indicatorv2.TruncateSize, inc.window+2)

	inc.Highs = types.ShrinkSlice(inc.Highs, threshold, size)
	inc.Lows = types.ShrinkSlice(inc.Lows, threshold, size)
	inc.LongScores = types.ShrinkSlice(inc.LongScores, threshold, size)
	inc.ShortScores = types.ShrinkSlice(inc.ShortScores, threshold, size)
	inc.Slice = types.ShrinkSlice(inc.Slice, threshold, size)
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
