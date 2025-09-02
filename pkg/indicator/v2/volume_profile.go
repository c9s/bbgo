package indicatorv2

import (
	"math"

	"golang.org/x/exp/slices"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/stat"

	bbgofloats "github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

// DefaultValueAreaPercentage is the percentage of the total volume used to calculate the value area.
const DefaultValueAreaPercentage = 0.68

type VolumeProfileStream struct {
	*types.Float64Series
	VP     VolumeProfile
	window int
}

// VolumeProfile is a histogram of market price and volume.
// Intent is to show the price points with most volume during a period.
// The profile gives key features such as:
//
// Point of control (POC)
//
// Value area high (VAH)
//
// Value area low (VAL)
//
// Session High/Low
type VolumeProfile struct {

	// Bins is the histogram bins.
	Bins []float64

	// Hist is the histogram values.
	Hist []float64

	// POC is the point of control.
	POC float64

	// VAH is the value area high.
	VAH float64

	// VAL is the value area low.
	VAL float64

	// High is the highest price in the profile.
	High float64

	// Low is the lowest price in the profile.
	Low float64
}

// VolumeLevel is a price and volume pair used to build a volume profile.
type VolumeLevel struct {

	// Price is the market price, typically the high/low average of the kline.
	Price float64

	// Volume is the total buy and sell volume at the price.
	Volume float64
}

func NewVolumeProfile(source KLineSubscription, window int) *VolumeProfileStream {
	prices := HLC3(source)
	volumes := Volumes(source)

	s := &VolumeProfileStream{
		Float64Series: types.NewFloat64Series(),
		window:        window,
	}

	source.AddSubscriber(func(v types.KLine) {
		if source.Length() < window {
			s.PushAndEmit(0)
			return
		}
		var nBins = 10
		// nBins = int(math.Floor((prices.Slice.Max()-prices.Slice.Min())/binWidth)) + 1
		s.VP.High = prices.Slice.Max()
		s.VP.Low = prices.Slice.Min()
		sortedPrices, sortedVolumes := buildVolumeLevel(prices.Slice, volumes.Slice)
		s.VP.Bins = make([]float64, nBins)
		s.VP.Bins = floats.Span(s.VP.Bins, s.VP.Low, s.VP.High+1)
		s.VP.Hist = stat.Histogram(nil, s.VP.Bins, sortedPrices, sortedVolumes)

		pocIdx := floats.MaxIdx(s.VP.Hist)
		s.VP.POC = midBin(s.VP.Bins, pocIdx)

		// TODO the results are of by small difference whereas it is expected they work the same
		// vaTotalVol := volumes.Sum() * DefaultValueAreaPercentage
		// Calculate Value Area with POC as the centre point\
		vaTotalVol := floats.Sum(volumes.Slice) * DefaultValueAreaPercentage

		vaCumVol := s.VP.Hist[pocIdx]
		var vahVol, valVol float64
		vahIdx, valIdx := pocIdx+1, pocIdx-1
		stepVAH, stepVAL := true, true

		for (vaCumVol <= vaTotalVol) &&
			(vahIdx <= len(s.VP.Hist)-1 && valIdx >= 0) {

			if stepVAH {
				vahVol = 0
				for vahVol == 0 && vahIdx+1 < len(s.VP.Hist)-1 {
					vahVol = s.VP.Hist[vahIdx] + s.VP.Hist[vahIdx+1]
					vahIdx += 2
				}
				stepVAH = false
			}

			if stepVAL {
				valVol = 0
				for valVol == 0 && valIdx-1 >= 0 {
					valVol = s.VP.Hist[valIdx] + s.VP.Hist[valIdx-1]
					valIdx -= 2
				}
				stepVAL = false
			}

			switch {
			case vahVol > valVol:
				vaCumVol += vahVol
				stepVAH, stepVAL = true, false
			case vahVol < valVol:
				vaCumVol += valVol
				stepVAH, stepVAL = false, true
			case vahVol == valVol:
				vaCumVol += valVol + vahVol
				stepVAH, stepVAL = true, true
			}

			if vahIdx >= len(s.VP.Hist)-1 {
				stepVAH = false
			}

			if valIdx <= 0 {
				stepVAL = false
			}
		}

		s.VP.VAH = midBin(s.VP.Bins, vahIdx)
		s.VP.VAL = midBin(s.VP.Bins, valIdx)

	})

	return s
}

func (s *VolumeProfileStream) Truncate() {
	s.Slice = s.Slice.Truncate(5000)
}

func buildVolumeLevel(p, v bbgofloats.Slice) (sortedp, sortedv bbgofloats.Slice) {
	var levels []VolumeLevel
	for i := range p {
		levels = append(levels, VolumeLevel{
			Price:  p[i],
			Volume: v[i],
		})
	}

	slices.SortStableFunc(levels, func(i, j VolumeLevel) bool {
		return i.Price < j.Price
	})

	for _, v := range levels {
		sortedp.Append(v.Price)
		sortedv.Append(v.Volume)
	}

	return
}

func midBin(bins []float64, idx int) float64 {

	if len(bins) == 0 {
		return math.NaN()
	}

	if idx >= len(bins)-1 {
		return bins[len(bins)-1]
	}

	if idx < 0 {
		return bins[0]
	}

	return stat.Mean([]float64{bins[idx], bins[idx+1]}, nil)
}
