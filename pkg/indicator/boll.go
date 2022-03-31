package indicator

import (
	"time"

	log "github.com/sirupsen/logrus"
	"gonum.org/v1/gonum/stat"

	"github.com/c9s/bbgo/pkg/types"
)

/*
boll implements the bollinger indicator:

The Basics of Bollinger Bands
- https://www.investopedia.com/articles/technical/102201.asp

Bollinger Bands
- https://www.investopedia.com/terms/b/bollingerbands.asp

Bollinger Bands Technical indicator guide:
- https://www.fidelity.com/learning-center/trading-investing/technical-analysis/technical-indicator-guide/bollinger-bands
*/

//go:generate callbackgen -type BOLL
type BOLL struct {
	types.IntervalWindow

	// times of Std, generally it's 2
	K float64

	SMA      types.Float64Slice
	StdDev   types.Float64Slice
	UpBand   types.Float64Slice
	DownBand types.Float64Slice

	EndTime time.Time

	updateCallbacks []func(sma, upBand, downBand float64)
}

type BandType int

const (
	_SMA BandType = iota
	_StdDev
	_UpBand
	_DownBand
)

func (inc *BOLL) GetUpBand() types.Series {
	return &BollSeries{
		inc, _UpBand,
	}
}

func (inc *BOLL) GetDownBand() types.Series {
	return &BollSeries{
		inc, _DownBand,
	}
}

func (inc *BOLL) GetSMA() types.Series {
	return &BollSeries{
		inc, _SMA,
	}
}

func (inc *BOLL) GetStdDev() types.Series {
	return &BollSeries{
		inc, _StdDev,
	}
}

func (inc *BOLL) LastUpBand() float64 {
	if len(inc.UpBand) == 0 {
		return 0.0
	}

	return inc.UpBand[len(inc.UpBand)-1]
}

func (inc *BOLL) LastDownBand() float64 {
	if len(inc.DownBand) == 0 {
		return 0.0
	}

	return inc.DownBand[len(inc.DownBand)-1]
}

func (inc *BOLL) LastStdDev() float64 {
	if len(inc.StdDev) == 0 {
		return 0.0
	}

	return inc.StdDev[len(inc.StdDev)-1]
}

func (inc *BOLL) LastSMA() float64 {
	if len(inc.SMA) > 0 {
		return inc.SMA[len(inc.SMA)-1]
	}
	return 0.0
}

func (inc *BOLL) calculateAndUpdate(kLines []types.KLine) {
	if len(kLines) < inc.Window {
		return
	}

	var index = len(kLines) - 1
	var kline = kLines[index]

	if inc.EndTime != zeroTime && kline.EndTime.Before(inc.EndTime) {
		return
	}

	var recentK = kLines[index-(inc.Window-1) : index+1]
	sma, err := calculateSMA(recentK, inc.Window, KLineClosePriceMapper)
	if err != nil {
		log.WithError(err).Error("SMA error")
		return
	}

	inc.SMA.Push(sma)

	var prices []float64
	for _, k := range recentK {
		prices = append(prices, k.Close.Float64())
	}

	var std = stat.StdDev(prices, nil)
	inc.StdDev.Push(std)

	var band = inc.K * std

	var upBand = sma + band
	inc.UpBand.Push(upBand)

	var downBand = sma - band
	inc.DownBand.Push(downBand)

	// update end time
	inc.EndTime = kLines[index].EndTime.Time()

	// log.Infof("update boll: sma=%f, up=%f, down=%f", sma, upBand, downBand)

	inc.EmitUpdate(sma, upBand, downBand)
}

func (inc *BOLL) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	if inc.EndTime != zeroTime && inc.EndTime.Before(inc.EndTime) {
		return
	}

	inc.calculateAndUpdate(window)
}

func (inc *BOLL) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}

type BollSeries struct {
	*BOLL
	bandType BandType
}

func (b *BollSeries) Last() float64 {
	switch b.bandType {
	case _SMA:
		return b.LastSMA()
	case _StdDev:
		return b.LastStdDev()
	case _UpBand:
		return b.LastUpBand()
	case _DownBand:
		return b.LastDownBand()
	default:
		panic("bandType wrong")
	}
}

func (b *BollSeries) Index(i int) float64 {
	switch b.bandType {
	case _SMA:
		if len(b.SMA) <= i {
			return 0
		}
		return b.SMA[len(b.SMA)-i-1]
	case _StdDev:
		if len(b.StdDev) <= i {
			return 0
		}
		return b.StdDev[len(b.StdDev)-i-1]
	case _UpBand:
		if len(b.UpBand) <= i {
			return 0
		}
		return b.UpBand[len(b.UpBand)-i-1]
	case _DownBand:
		if len(b.DownBand) <= i {
			return 0
		}
		return b.DownBand[len(b.DownBand)-i-1]
	default:
		panic("bandType wrong")
	}
}

func (b *BollSeries) Length() int {
	switch b.bandType {
	case _SMA:
		return len(b.SMA)
	case _StdDev:
		return len(b.StdDev)
	case _UpBand:
		return len(b.UpBand)
	case _DownBand:
		return len(b.DownBand)
	default:
		panic("bandType wrong")
	}
}

var _ types.Series = &BollSeries{}
