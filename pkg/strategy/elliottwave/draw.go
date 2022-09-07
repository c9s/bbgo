package elliottwave

import (
	"bytes"
	"fmt"
	"os"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/interact"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/wcharczuk/go-chart/v2"
)

func (s *Strategy) InitDrawCommands(store *bbgo.SerialMarketDataStore, profit, cumProfit types.Series) {
	bbgo.RegisterCommand("/draw", "Draw Indicators", func(reply interact.Reply) {
		canvas := s.DrawIndicators(store)
		if canvas == nil {
			reply.Message("cannot render indicators")
			return
		}
		var buffer bytes.Buffer
		if err := canvas.Render(chart.PNG, &buffer); err != nil {
			log.WithError(err).Errorf("cannot render indicators in ewo")
			reply.Message(fmt.Sprintf("[error] cannot render indicators in ewo: %v", err))
			return
		}
		bbgo.SendPhoto(&buffer)
	})
	bbgo.RegisterCommand("/pnl", "Draw PNL(%) per trade", func(reply interact.Reply) {
		canvas := s.DrawPNL(profit)
		var buffer bytes.Buffer
		if err := canvas.Render(chart.PNG, &buffer); err != nil {
			log.WithError(err).Errorf("cannot render pnl in drift")
			reply.Message(fmt.Sprintf("[error] cannot render pnl in ewo: %v", err))
			return
		}
		bbgo.SendPhoto(&buffer)
	})
	bbgo.RegisterCommand("/cumpnl", "Draw Cummulative PNL(Quote)", func(reply interact.Reply) {
		canvas := s.DrawCumPNL(cumProfit)
		var buffer bytes.Buffer
		if err := canvas.Render(chart.PNG, &buffer); err != nil {
			log.WithError(err).Errorf("cannot render cumpnl in drift")
			reply.Message(fmt.Sprintf("[error] canot render cumpnl in drift: %v", err))
			return
		}
		bbgo.SendPhoto(&buffer)
	})
}

func (s *Strategy) DrawIndicators(store *bbgo.SerialMarketDataStore) *types.Canvas {
	klines, ok := store.KLinesOfInterval(types.Interval1m)
	if !ok {
		return nil
	}
	time := (*klines)[len(*klines)-1].StartTime
	canvas := types.NewCanvas(s.InstanceID(), s.Interval)
	Length := s.priceLines.Length()
	if Length > 300 {
		Length = 300
	}
	log.Infof("draw indicators with %d data", Length)
	mean := s.priceLines.Mean(Length)
	high := s.priceLines.Highest(Length)
	low := s.priceLines.Lowest(Length)
	ehigh := types.Highest(s.ewo, Length)
	elow := types.Lowest(s.ewo, Length)
	canvas.Plot("ewo", types.Add(types.Mul(s.ewo, (high-low)/(ehigh-elow)), mean), time, Length)
	canvas.Plot("zero", types.NumberSeries(mean), time, Length)
	canvas.Plot("price", s.priceLines, time, Length)
	return canvas
}

func (s *Strategy) DrawPNL(profit types.Series) *types.Canvas {
	canvas := types.NewCanvas(s.InstanceID())
	length := profit.Length()
	log.Errorf("pnl Highest: %f, Lowest: %f", types.Highest(profit, length), types.Lowest(profit, length))
	canvas.PlotRaw("pnl %", profit, length)
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

func (s *Strategy) Draw(store *bbgo.SerialMarketDataStore, profit, cumProfit types.Series) {
	canvas := s.DrawIndicators(store)
	f, err := os.Create(s.GraphIndicatorPath)
	if err != nil {
		log.WithError(err).Errorf("cannot create on path " + s.GraphIndicatorPath)
		return
	}
	defer f.Close()
	if err = canvas.Render(chart.PNG, f); err != nil {
		log.WithError(err).Errorf("cannot render elliottwave")
	}

	canvas = s.DrawPNL(profit)
	f, err = os.Create(s.GraphPNLPath)
	if err != nil {
		log.WithError(err).Errorf("cannot create on path " + s.GraphPNLPath)
		return
	}
	defer f.Close()
	if err = canvas.Render(chart.PNG, f); err != nil {
		log.WithError(err).Errorf("cannot render pnl")
		return
	}
	canvas = s.DrawCumPNL(cumProfit)
	f, err = os.Create(s.GraphCumPNLPath)
	if err != nil {
		log.WithError(err).Errorf("cannot create on path " + s.GraphCumPNLPath)
		return
	}
	defer f.Close()
	if err = canvas.Render(chart.PNG, f); err != nil {
		log.WithError(err).Errorf("cannot render cumpnl")
	}
}
