package ewoDgtrd

import (
	"fmt"
	"math"

	"github.com/c9s/bbgo/pkg/types"
)

type HeikinAshi struct {
	Close  *types.Queue
	Open   *types.Queue
	High   *types.Queue
	Low    *types.Queue
	Volume *types.Queue
}

func NewHeikinAshi(size int) *HeikinAshi {
	return &HeikinAshi{
		Close:  types.NewQueue(size),
		Open:   types.NewQueue(size),
		High:   types.NewQueue(size),
		Low:    types.NewQueue(size),
		Volume: types.NewQueue(size),
	}
}

func (s *HeikinAshi) Print() string {
	return fmt.Sprintf("Heikin c: %.3f, o: %.3f, h: %.3f, l: %.3f, v: %.3f",
		s.Close.Last(),
		s.Open.Last(),
		s.High.Last(),
		s.Low.Last(),
		s.Volume.Last())
}

func (inc *HeikinAshi) Update(kline types.KLine) {
	open := kline.Open.Float64()
	cloze := kline.Close.Float64()
	high := kline.High.Float64()
	low := kline.Low.Float64()
	newClose := (open + high + low + cloze) / 4.
	newOpen := (inc.Open.Last() + inc.Close.Last()) / 2.
	inc.Close.Update(newClose)
	inc.Open.Update(newOpen)
	inc.High.Update(math.Max(math.Max(high, newOpen), newClose))
	inc.Low.Update(math.Min(math.Min(low, newOpen), newClose))
	inc.Volume.Update(kline.Volume.Float64())
}
