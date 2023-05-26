package drift

import (
	"bytes"
	"fmt"
	"os"

	"github.com/wcharczuk/go-chart/v2"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/interact"
	"github.com/c9s/bbgo/pkg/types"
)

func (s *Strategy) InitDrawCommands(profit, cumProfit types.Series) {
	bbgo.RegisterCommand("/draw", "Draw Indicators", func(reply interact.Reply) {
		go func() {
			canvas := s.DrawIndicators(s.frameKLine.StartTime)
			var buffer bytes.Buffer
			if err := canvas.Render(chart.PNG, &buffer); err != nil {
				log.WithError(err).Errorf("cannot render indicators in drift")
				return
			}
			bbgo.SendPhoto(&buffer)
		}()
	})

	bbgo.RegisterCommand("/pnl", "Draw PNL(%) per trade", func(reply interact.Reply) {
		go func() {
			canvas := s.DrawPNL(profit)
			var buffer bytes.Buffer
			if err := canvas.Render(chart.PNG, &buffer); err != nil {
				log.WithError(err).Errorf("cannot render pnl in drift")
				return
			}
			bbgo.SendPhoto(&buffer)
		}()
	})

	bbgo.RegisterCommand("/cumpnl", "Draw Cummulative PNL(Quote)", func(reply interact.Reply) {
		go func() {
			canvas := s.DrawCumPNL(cumProfit)
			var buffer bytes.Buffer
			if err := canvas.Render(chart.PNG, &buffer); err != nil {
				log.WithError(err).Errorf("cannot render cumpnl in drift")
				return
			}
			bbgo.SendPhoto(&buffer)
		}()
	})

	bbgo.RegisterCommand("/elapsed", "Draw Elapsed time for handlers for each kline close event", func(reply interact.Reply) {
		go func() {
			canvas := s.DrawElapsed()
			var buffer bytes.Buffer
			if err := canvas.Render(chart.PNG, &buffer); err != nil {
				log.WithError(err).Errorf("cannot render elapsed in drift")
				return
			}
			bbgo.SendPhoto(&buffer)
		}()
	})
}

func (s *Strategy) DrawIndicators(time types.Time) *types.Canvas {
	canvas := types.NewCanvas(s.InstanceID(), s.Interval)
	length := s.priceLines.Length()
	if length > 300 {
		length = 300
	}
	log.Infof("draw indicators with %d data", length)
	mean := s.priceLines.Mean(length)
	highestPrice := s.priceLines.Minus(mean).Abs().Highest(length)
	highestDrift := s.drift.Abs().Highest(length)
	hi := s.drift.drift.Abs().Highest(length)
	ratio := highestPrice / highestDrift

	// canvas.Plot("upband", s.ma.Add(s.stdevHigh), time, length)
	canvas.Plot("ma", s.ma, time, length)
	// canvas.Plot("downband", s.ma.Sub(s.stdevLow), time, length)
	fmt.Printf("%f %f\n", highestPrice, hi)

	canvas.Plot("trend", s.trendLine, time, length)
	canvas.Plot("drift", s.drift.Mul(ratio).Add(mean), time, length)
	canvas.Plot("driftOrig", s.drift.drift.Mul(highestPrice/hi).Add(mean), time, length)
	canvas.Plot("zero", types.NumberSeries(mean), time, length)
	canvas.Plot("price", s.priceLines, time, length)
	return canvas
}

func (s *Strategy) DrawPNL(profit types.Series) *types.Canvas {
	canvas := types.NewCanvas(s.InstanceID())
	log.Errorf("pnl Highest: %f, Lowest: %f", types.Highest(profit, profit.Length()), types.Lowest(profit, profit.Length()))
	length := profit.Length()
	if s.GraphPNLDeductFee {
		canvas.PlotRaw("pnl % (with Fee Deducted)", profit, length)
	} else {
		canvas.PlotRaw("pnl %", profit, length)
	}
	canvas.YAxis = chart.YAxis{
		ValueFormatter: func(v interface{}) string {
			if vf, isFloat := v.(float64); isFloat {
				return fmt.Sprintf("%.4f", vf)
			}
			return ""
		},
	}
	canvas.PlotRaw("1", types.NumberSeries(1), length)
	return canvas
}

func (s *Strategy) DrawCumPNL(cumProfit types.Series) *types.Canvas {
	canvas := types.NewCanvas(s.InstanceID())
	canvas.PlotRaw("cummulative pnl", cumProfit, cumProfit.Length())
	canvas.YAxis = chart.YAxis{
		ValueFormatter: func(v interface{}) string {
			if vf, isFloat := v.(float64); isFloat {
				return fmt.Sprintf("%.4f", vf)
			}
			return ""
		},
	}
	return canvas
}

func (s *Strategy) DrawElapsed() *types.Canvas {
	canvas := types.NewCanvas(s.InstanceID())
	canvas.PlotRaw("elapsed time(ms)", s.elapsed, s.elapsed.Length())
	return canvas
}

func (s *Strategy) Draw(time types.Time, profit types.Series, cumProfit types.Series) {
	canvas := s.DrawIndicators(time)
	f, err := os.Create(s.CanvasPath)
	if err != nil {
		log.WithError(err).Errorf("cannot create on %s", s.CanvasPath)
		return
	}
	if err := canvas.Render(chart.PNG, f); err != nil {
		log.WithError(err).Errorf("cannot render in drift")
	}
	f.Close()

	canvas = s.DrawPNL(profit)
	f, err = os.Create(s.GraphPNLPath)
	if err != nil {
		log.WithError(err).Errorf("open pnl")
		return
	}
	if err := canvas.Render(chart.PNG, f); err != nil {
		log.WithError(err).Errorf("render pnl")
	}
	f.Close()

	canvas = s.DrawCumPNL(cumProfit)
	f, err = os.Create(s.GraphCumPNLPath)
	if err != nil {
		log.WithError(err).Errorf("open cumpnl")
		return
	}
	if err := canvas.Render(chart.PNG, f); err != nil {
		log.WithError(err).Errorf("render cumpnl")
	}
	f.Close()

	canvas = s.DrawElapsed()
	f, err = os.Create(s.GraphElapsedPath)
	if err != nil {
		log.WithError(err).Errorf("open elapsed")
		return
	}
	if err := canvas.Render(chart.PNG, f); err != nil {
		log.WithError(err).Errorf("render elapsed")
	}
	f.Close()
}
