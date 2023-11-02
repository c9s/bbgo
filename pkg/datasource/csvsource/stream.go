package csvsource

import (
	"encoding/csv"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate callbackgen -type Stream
type Stream struct {
	types.StandardStream
	config                    *CsvStreamConfig
	converter                 ICSVTickConverter
	marketTradeEventCallbacks []func(e []CsvTick)
	kLineEventCallbacks       []func(e []types.KLine)
	orderEventCallbacks       []func(e []types.Order)
	tradeEventCallbacks       []func(e []types.Trade)
}

type CsvStreamConfig struct {
	Interval     types.Interval     `json:"interval"`
	RateLimit    time.Duration      `json:"rateLimit"`
	StrategyID   string             `json:"strategyID"`
	CsvPath      string             `json:"csvPath"`
	Exchange     types.ExchangeName `json:"exchange"`
	Symbol       string             `json:"symbol"`
	BaseCoin     string             `json:"baseCoin"`
	QuoteCoin    string             `json:"quoteCoin"`
	TakerFeeRate fixedpoint.Value   `json:"takerFeeRate"`
	MakerFeeRate fixedpoint.Value   `json:"makerFeeRate"`
}

func NewStream(cfg *CsvStreamConfig) *Stream {
	stream := &Stream{
		StandardStream: types.NewStandardStream(),
		converter:      NewCSVTickConverter(),
		config:         cfg,
	}

	// stream.SetParser(stream.simulateEvents)
	// stream.SetDispatcher(stream.dispatchEvent)
	// stream.OnMarketTradeEvent(stream.handleMarketTradeEvent)
	// stream.OnKLineEvent(stream.handleKLineEvent)
	// stream.OnOrderEvent(stream.handleOrderEvent)
	// stream.OnTradeEvent(stream.handleTradeEvent)
	return stream
}

func (s *Stream) Simulate() error {
	var i int
	// iterate equity series at csv path and stream
	err := filepath.WalkDir(s.config.CsvPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".csv" {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		//nolint:errcheck // Read ops only so safe to ignore err return
		defer file.Close()
		reader := NewBybitCSVTickReader(csv.NewReader(file))
		tick, err := reader.Read(i)
		if err != nil {
			return err
		}
		tick.Symbol = s.config.Symbol // not every csv provides symbol information

		trade, err := tick.toGlobalTrade()
		if err != nil {
			return err
		}
		trade.Fee = s.config.TakerFeeRate
		trade.FeeCurrency = s.config.QuoteCoin
		if tick.IsBuyerMaker { // if supported by exchange csv format
			trade.Fee = s.config.MakerFeeRate
		}
		s.StandardStream.EmitMarketTrade(*trade)

		kline := s.converter.LatestKLine()
		closesKline := s.converter.CsvTickToKLine(tick, s.config.Interval)
		if closesKline {
			s.StandardStream.EmitKLineClosed(*kline)
		} else {
			kline = s.converter.LatestKLine() // overwrite with newer KLine
			s.StandardStream.EmitKLine(*kline)
		}

		// allow for execution time of indicators and strategy
		time.Sleep(s.config.RateLimit) // Max execution time for tradingview strategy is 250ms
		// to optimize exec time consider callback channel once a strategy has finished running another tick is emitted

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

// func (s *Stream) dispatchEvent(event interface{}) {
// 	switch e := event.(type) {
// 	case []CsvTick:
// 		s.EmitMarketTradeEvent(e)
// 	case []types.KLine:
// 		s.EmitKLineEvent(e)
// 	case []types.Order:
// 		s.EmitOrderEvent(e)
// 	case []types.Trade:
// 		s.EmitTradeEvent(e)
// 	}
// }

// func (s *Stream) handleMarketTradeEvent(events []CsvTick) {
// 	for _, event := range events {
// 		trade, err := event.toGlobalTrade()
// 		if err != nil {
// 			log.WithError(err).Error("failed to convert to market trade")
// 			continue
// 		}

// 		s.StandardStream.EmitMarketTrade(trade)
// 	}
// }

// func (s *Stream) handleOrderEvent(events []types.Order) {
// 	for _, event := range events {
// 		s.StandardStream.EmitOrderUpdate(event)
// 	}
// }

// func (s *Stream) handleKLineEvent(klines []types.KLine) {
// 	for _, kline := range klines {
// 		if kline.Closed {
// 			s.EmitKLineClosed(kline)
// 		} else {
// 			s.EmitKLine(kline)
// 		}
// 	}
// }

// func (s *Stream) handleTradeEvent(events []types.Trade) {
// 	for _, event := range events {
// 		s.StandardStream.EmitTradeUpdate(event)
// 	}
// }
