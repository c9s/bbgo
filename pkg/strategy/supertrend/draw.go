package supertrend

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

	canvas := s.DrawPNL(profit)
	f, err := os.Create(s.GraphPNLPath)
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
