package grid2

import (
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/marketdata/sources/binancecsv"
	"github.com/c9s/bbgo/pkg/types"
)

type HedgeService interface {
	Hedge(uncovered, delta fixedpoint.Value) error
}

//go:generate callbackgen -type HedgeSimulator
type HedgeSimulator struct {
	Symbol   string
	Market   types.Market
	Position *types.Position

	PositionExposure *bbgo.PositionExposure

	ProfitStats *types.ProfitStats
	KLines      []types.KLine
	FeeRate     fixedpoint.Value
	Spread      fixedpoint.Value
	Slippage    fixedpoint.Value

	currentPrice   fixedpoint.Value
	currentTime    time.Time
	lastKLineIndex int
	orderId        uint64
	tradeId        uint64

	tradeCallbacks []func(trade types.Trade)
}

func NewHedgeSimulator(symbol string, market types.Market) *HedgeSimulator {
	return &HedgeSimulator{
		Symbol:           symbol,
		Market:           market,
		Position:         types.NewPositionFromMarket(market),
		PositionExposure: bbgo.NewPositionExposure(symbol),
		ProfitStats:      types.NewProfitStats(market),
		FeeRate:          fixedpoint.Zero,
		orderId:          0,
		tradeId:          0,
	}
}

func (s *HedgeSimulator) SetFeeRate(feeRate fixedpoint.Value) {
	s.FeeRate = feeRate
}

func (s *HedgeSimulator) SetSpread(spread fixedpoint.Value) {
	s.Spread = spread
}

func (s *HedgeSimulator) SetSlippage(slippage fixedpoint.Value) {
	s.Slippage = slippage
}

func (s *HedgeSimulator) LoadKLines(dir string) error {
	klines, err := binancecsv.ReadKLineDir(dir, s.Symbol, types.Interval1m)
	if err != nil {
		return err
	}
	s.KLines = klines
	s.lastKLineIndex = 0
	return nil
}

func (s *HedgeSimulator) Move(timestamp time.Time) error {
	s.currentTime = timestamp

	if len(s.KLines) == 0 {
		return nil
	}

	if s.lastKLineIndex >= len(s.KLines) {
		s.lastKLineIndex = 0
	}

	// If the timestamp is before the current k-line, reset index to 0
	if s.KLines[s.lastKLineIndex].GetEndTime().Time().After(timestamp) {
		s.lastKLineIndex = 0
	}

	for i := s.lastKLineIndex; i < len(s.KLines); i++ {
		k := s.KLines[i]
		if timestamp.Before(k.StartTime.Time()) {
			// This timestamp is before this k-line, and we haven't found a match yet.
			// If we are at the first k-line, we can't do much.
			// But if we are between k-lines (shouldn't happen with continuous data),
			// we might want to use the previous k-line's close.
			if i > 0 {
				s.currentPrice = s.KLines[i-1].Close
				s.lastKLineIndex = i - 1
			}
			return nil
		}

		if timestamp.Before(k.GetEndTime().Time()) {
			// Inside the k-line, we only know the Open price.
			s.currentPrice = k.Open
			s.lastKLineIndex = i
			return nil
		}

		if timestamp.Equal(k.GetEndTime().Time()) || (i+1 < len(s.KLines) && timestamp.Before(s.KLines[i+1].StartTime.Time())) {
			// At the end of the k-line or between this and next.
			s.currentPrice = k.Close
			s.lastKLineIndex = i
			return nil
		}
	}

	// If timestamp is after all k-lines, use the last close price
	if len(s.KLines) > 0 {
		s.currentPrice = s.KLines[len(s.KLines)-1].Close
		s.lastKLineIndex = len(s.KLines) - 1
	}
	return nil
}

func (s *HedgeSimulator) Hedge(uncovered, delta fixedpoint.Value) error {
	if s.currentPrice.IsZero() {
		return nil
	}

	//s.PositionExposure.Open(delta)
	//s.PositionExposure.Cover(delta)
	//defer s.PositionExposure.Close(delta.Neg())

	trade := s.buildMockTrade(delta)

	pnl, netPnl, madeProfit := s.Position.AddTrade(trade)
	if madeProfit {
		s.ProfitStats.AddProfit(s.Position.NewProfit(trade, pnl, netPnl))
	}

	s.ProfitStats.AddTrade(trade)

	s.EmitTrade(trade)
	return nil
}

func (s *HedgeSimulator) buildMockTrade(delta fixedpoint.Value) types.Trade {
	side := types.SideTypeSell
	if delta.Sign() > 0 {
		side = types.SideTypeBuy
	}

	price := s.currentPrice
	if s.Spread.Sign() > 0 {
		if side == types.SideTypeBuy {
			price = price.Add(s.Spread.Div(fixedpoint.NewFromInt(2)))
		} else {
			price = price.Sub(s.Spread.Div(fixedpoint.NewFromInt(2)))
		}
	}

	if s.Slippage.Sign() > 0 {
		if side == types.SideTypeBuy {
			price = price.Add(s.currentPrice.Mul(s.Slippage))
		} else {
			price = price.Sub(s.currentPrice.Mul(s.Slippage))
		}
	}

	s.orderId++
	s.tradeId++

	trade := types.Trade{
		ID:            0,
		OrderID:       s.orderId,
		Exchange:      "HedgeSimulator",
		Price:         price,
		Quantity:      delta.Abs(),
		QuoteQuantity: delta.Abs().Mul(price),
		Symbol:        s.Symbol,
		Side:          side,
		IsBuyer:       side == types.SideTypeBuy,
		IsMaker:       false,
		Time:          types.Time(s.currentTime),
	}

	if s.FeeRate.Sign() > 0 {
		trade.Fee = trade.QuoteQuantity.Mul(s.FeeRate)
		trade.FeeCurrency = s.Market.QuoteCurrency
	}

	return trade
}
