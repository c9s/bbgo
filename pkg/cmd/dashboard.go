package cmd

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/c9s/bbgo/pkg/cmd/widget"
	"github.com/c9s/bbgo/pkg/exchange"
	"github.com/c9s/bbgo/pkg/types"
	ui "github.com/gizak/termui/v3"
	"github.com/spf13/cobra"
)

func init() {
	orderFlowCmd.Flags().String("exchange", "", "exchange name")
	orderFlowCmd.MarkFlagRequired("exchange")
	orderFlowCmd.Flags().String("symbol", "", "trading pair symbol")
	orderFlowCmd.MarkFlagRequired("symbol")
	dashboardCmd.AddCommand(orderFlowCmd)
	RootCmd.AddCommand(dashboardCmd)
}

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "start the dashboard",
	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

var orderFlowCmd = &cobra.Command{
	Use:   "orderflow",
	Short: "start the order flow dashboard",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ui.Init(); err != nil {
			log.Fatalf("failed to initialize termui: %v", err)
		}
		defer ui.Close()

		exName, err := cmd.Flags().GetString("exchange")
		if err != nil {
			return err
		}

		symbol, err := cmd.Flags().GetString("symbol")
		if err != nil {
			return err
		}
		if len(exName) == 0 || len(symbol) == 0 {
			return errors.New("--exchange and --symbol are required")
		}
		// create a new public trade stream
		exMin, err := exchange.NewWithEnvVarPrefix(types.ExchangeName(exName), strings.ToUpper(exName))
		ex, ok := exMin.(types.Exchange)
		if !ok {
			return errors.New("exchange does not implement types.Exchange")
		}
		stream := ex.NewStream()
		stream.Subscribe(types.MarketTradeChannel, symbol, types.SubscribeOptions{})

		w := widget.NewOrderFlowWidget()
		w.Title = " Order Flow Circles (總量正規化) "
		stream.OnMarketTrade(types.TradeWith(symbol, func(trade types.Trade) {
			w.UpdateTrade(trade)
		}))
		stream.SetPublicOnly()
		// start receiving market trades
		if err := stream.Connect(context.Background()); err != nil {
			return err
		}

		w.BorderStyle = ui.NewStyle(ui.ColorCyan)
		w.TitleStyle = ui.NewStyle(ui.ColorWhite, ui.ColorClear, ui.ModifierBold)

		termWidth, termHeight := ui.TerminalDimensions()
		w.SetRect(0, 0, termWidth, termHeight)
		ui.Render(w)

		// refresh ticker
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		uiEvents := ui.PollEvents()

		for {
			select {
			case <-ticker.C:
				ui.Render(w)
			case e := <-uiEvents:
				switch e.Type {
				case ui.KeyboardEvent:
					if e.ID == "q" || e.ID == "<C-c>" {
						return nil
					}
				case ui.ResizeEvent:
					payload := e.Payload.(ui.Resize)
					w.SetRect(0, 0, payload.Width, payload.Height)
					ui.Render(w)
				}
			}
		}
	},
}
