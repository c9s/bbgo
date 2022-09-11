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

func (s *Strategy) InitDrawCommands(profit, cumProfit types.Series) {
	bbgo.RegisterCommand("/pnl", "Draw PNL(%) per trade", func(reply interact.Reply) {
		canvas := DrawPNL(s.InstanceID(), profit)
		var buffer bytes.Buffer
		if err := canvas.Render(chart.PNG, &buffer); err != nil {
			log.WithError(err).Errorf("cannot render pnl in drift")
			reply.Message(fmt.Sprintf("[error] cannot render pnl in ewo: %v", err))
			return
		}
		bbgo.SendPhoto(&buffer)
	})
	bbgo.RegisterCommand("/cumpnl", "Draw Cummulative PNL(Quote)", func(reply interact.Reply) {
		canvas := DrawCumPNL(s.InstanceID(), cumProfit)
		var buffer bytes.Buffer
		if err := canvas.Render(chart.PNG, &buffer); err != nil {
			log.WithError(err).Errorf("cannot render cumpnl in drift")
			reply.Message(fmt.Sprintf("[error] canot render cumpnl in drift: %v", err))
			return
		}
		bbgo.SendPhoto(&buffer)
	})
}

func (s *Strategy) Draw(profit, cumProfit types.Series) error {

	canvas := DrawPNL(s.InstanceID(), profit)
	f, err := os.Create(s.GraphPNLPath)
	if err != nil {
		return fmt.Errorf("cannot create on path " + s.GraphPNLPath)
	}
	defer f.Close()
	if err = canvas.Render(chart.PNG, f); err != nil {
		return fmt.Errorf("cannot render pnl")
	}
	canvas = DrawCumPNL(s.InstanceID(), cumProfit)
	f, err = os.Create(s.GraphCumPNLPath)
	if err != nil {
		return fmt.Errorf("cannot create on path " + s.GraphCumPNLPath)
	}
	defer f.Close()
	if err = canvas.Render(chart.PNG, f); err != nil {
		return fmt.Errorf("cannot render cumpnl")
	}

	return nil
}

func DrawPNL(instanceID string, profit types.Series) *types.Canvas {
	canvas := types.NewCanvas(instanceID)
	length := profit.Length()
	log.Infof("pnl Highest: %f, Lowest: %f", types.Highest(profit, length), types.Lowest(profit, length))
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

func DrawCumPNL(instanceID string, cumProfit types.Series) *types.Canvas {
	canvas := types.NewCanvas(instanceID)
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
