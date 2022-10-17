package irr

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/data/tsv"
	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/sirupsen/logrus"
	"go.uber.org/atomic"
)

const ID = "irr"

var one = fixedpoint.One
var zero = fixedpoint.Zero
var Fee = 0.000 // taker fee % * 2, for upper bound

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	Environment *bbgo.Environment
	Symbol      string `json:"symbol"`
	Market      types.Market

	types.IntervalWindow

	// persistence fields
	Position    *types.Position    `persistence:"position"`
	ProfitStats *types.ProfitStats `persistence:"profit_stats"`
	TradeStats  *types.TradeStats  `persistence:"trade_stats"`

	activeOrders *bbgo.ActiveOrderBook

	ExitMethods bbgo.ExitMethodSet `json:"exits"`

	session       *bbgo.ExchangeSession
	orderExecutor *bbgo.GeneralOrderExecutor

	bbgo.QuantityOrAmount

	Interval int  `json:"hftInterval"`
	NR       bool `json:"NR"`
	MR       bool `json:"MR"`

	// for back-test
	Nrr *NRR
	Ma  *indicator.SMA
	// realtime book ticker to submit order
	obBuyPrice  *atomic.Float64
	obSellPrice *atomic.Float64
	// for getting close price
	currentTradePrice *atomic.Float64
	// for negative return rate
	openPrice  float64
	closePrice float64
	rtNr       *types.Queue
	// for moving average reversion
	rtMaFast *types.Queue
	rtMaSlow *types.Queue
	rtMr     *types.Queue

	// for final alpha (Nr+Mr)/2
	rtWeight *types.Queue
	stopC    chan struct{}

	// StrategyController
	bbgo.StrategyController

	AccountValueCalculator *bbgo.AccountValueCalculator

	// whether to draw graph or not by the end of backtest
	DrawGraph       bool   `json:"drawGraph"`
	GraphPNLPath    string `json:"graphPNLPath"`
	GraphCumPNLPath string `json:"graphCumPNLPath"`

	// for position
	buyPrice     float64 `persistence:"buy_price"`
	sellPrice    float64 `persistence:"sell_price"`
	highestPrice float64 `persistence:"highest_price"`
	lowestPrice  float64 `persistence:"lowest_price"`

	// Accumulated profit report
	AccumulatedProfitReport *AccumulatedProfitReport `json:"accumulatedProfitReport"`
}

// AccumulatedProfitReport For accumulated profit report output
type AccumulatedProfitReport struct {
	// AccumulatedProfitMAWindow Accumulated profit SMA window, in number of trades
	AccumulatedProfitMAWindow int `json:"accumulatedProfitMAWindow"`

	// IntervalWindow interval window, in days
	IntervalWindow int `json:"intervalWindow"`

	// NumberOfInterval How many intervals to output to TSV
	NumberOfInterval int `json:"NumberOfInterval"`

	// TsvReportPath The path to output report to
	TsvReportPath string `json:"tsvReportPath"`

	// AccumulatedDailyProfitWindow The window to sum up the daily profit, in days
	AccumulatedDailyProfitWindow int `json:"accumulatedDailyProfitWindow"`

	// Accumulated profit
	accumulatedProfit         fixedpoint.Value
	accumulatedProfitPerDay   floats.Slice
	previousAccumulatedProfit fixedpoint.Value

	// Accumulated profit MA
	accumulatedProfitMA       *indicator.SMA
	accumulatedProfitMAPerDay floats.Slice

	// Daily profit
	dailyProfit floats.Slice

	// Accumulated fee
	accumulatedFee       fixedpoint.Value
	accumulatedFeePerDay floats.Slice

	// Win ratio
	winRatioPerDay floats.Slice

	// Profit factor
	profitFactorPerDay floats.Slice

	// Trade number
	dailyTrades               floats.Slice
	accumulatedTrades         int
	previousAccumulatedTrades int
}

func (r *AccumulatedProfitReport) Initialize() {
	if r.AccumulatedProfitMAWindow <= 0 {
		r.AccumulatedProfitMAWindow = 60
	}
	if r.IntervalWindow <= 0 {
		r.IntervalWindow = 7
	}
	if r.AccumulatedDailyProfitWindow <= 0 {
		r.AccumulatedDailyProfitWindow = 7
	}
	if r.NumberOfInterval <= 0 {
		r.NumberOfInterval = 1
	}
	r.accumulatedProfitMA = &indicator.SMA{IntervalWindow: types.IntervalWindow{Interval: types.Interval1d, Window: r.AccumulatedProfitMAWindow}}
}

func (r *AccumulatedProfitReport) RecordProfit(profit fixedpoint.Value) {
	r.accumulatedProfit = r.accumulatedProfit.Add(profit)
}

func (r *AccumulatedProfitReport) RecordTrade(fee fixedpoint.Value) {
	r.accumulatedFee = r.accumulatedFee.Add(fee)
	r.accumulatedTrades += 1
}

func (r *AccumulatedProfitReport) DailyUpdate(tradeStats *types.TradeStats) {
	// Daily profit
	r.dailyProfit.Update(r.accumulatedProfit.Sub(r.previousAccumulatedProfit).Float64())
	r.previousAccumulatedProfit = r.accumulatedProfit

	// Accumulated profit
	r.accumulatedProfitPerDay.Update(r.accumulatedProfit.Float64())

	// Accumulated profit MA
	r.accumulatedProfitMA.Update(r.accumulatedProfit.Float64())
	r.accumulatedProfitMAPerDay.Update(r.accumulatedProfitMA.Last())

	// Accumulated Fee
	r.accumulatedFeePerDay.Update(r.accumulatedFee.Float64())

	// Win ratio
	r.winRatioPerDay.Update(tradeStats.WinningRatio.Float64())

	// Profit factor
	r.profitFactorPerDay.Update(tradeStats.ProfitFactor.Float64())

	// Daily trades
	r.dailyTrades.Update(float64(r.accumulatedTrades - r.previousAccumulatedTrades))
	r.previousAccumulatedTrades = r.accumulatedTrades
}

// Output Accumulated profit report to a TSV file
func (r *AccumulatedProfitReport) Output(symbol string) {
	if r.TsvReportPath != "" {
		tsvwiter, err := tsv.AppendWriterFile(r.TsvReportPath)
		if err != nil {
			panic(err)
		}
		defer tsvwiter.Close()
		// Output symbol, total acc. profit, acc. profit 60MA, interval acc. profit, fee, win rate, profit factor
		_ = tsvwiter.Write([]string{"#", "Symbol", "accumulatedProfit", "accumulatedProfitMA", fmt.Sprintf("%dd profit", r.AccumulatedDailyProfitWindow), "accumulatedFee", "winRatio", "profitFactor", "60D trades"})
		for i := 0; i <= r.NumberOfInterval-1; i++ {
			accumulatedProfit := r.accumulatedProfitPerDay.Index(r.IntervalWindow * i)
			accumulatedProfitStr := fmt.Sprintf("%f", accumulatedProfit)
			accumulatedProfitMA := r.accumulatedProfitMAPerDay.Index(r.IntervalWindow * i)
			accumulatedProfitMAStr := fmt.Sprintf("%f", accumulatedProfitMA)
			intervalAccumulatedProfit := r.dailyProfit.Tail(r.AccumulatedDailyProfitWindow+r.IntervalWindow*i).Sum() - r.dailyProfit.Tail(r.IntervalWindow*i).Sum()
			intervalAccumulatedProfitStr := fmt.Sprintf("%f", intervalAccumulatedProfit)
			accumulatedFee := fmt.Sprintf("%f", r.accumulatedFeePerDay.Index(r.IntervalWindow*i))
			winRatio := fmt.Sprintf("%f", r.winRatioPerDay.Index(r.IntervalWindow*i))
			profitFactor := fmt.Sprintf("%f", r.profitFactorPerDay.Index(r.IntervalWindow*i))
			trades := r.dailyTrades.Tail(60+r.IntervalWindow*i).Sum() - r.dailyTrades.Tail(r.IntervalWindow*i).Sum()
			tradesStr := fmt.Sprintf("%f", trades)

			_ = tsvwiter.Write([]string{fmt.Sprintf("%d", i+1), symbol, accumulatedProfitStr, accumulatedProfitMAStr, intervalAccumulatedProfitStr, accumulatedFee, winRatio, profitFactor, tradesStr})
		}
	}
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	if !bbgo.IsBackTesting {
		session.Subscribe(types.AggTradeChannel, s.Symbol, types.SubscribeOptions{})
		session.Subscribe(types.BookTickerChannel, s.Symbol, types.SubscribeOptions{})
	}
	//session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s:%s", ID, s.Symbol)
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	var instanceID = s.InstanceID()

	if s.Position == nil {
		s.Position = types.NewPositionFromMarket(s.Market)
	}

	if s.ProfitStats == nil {
		s.ProfitStats = types.NewProfitStats(s.Market)
	}

	if s.TradeStats == nil {
		s.TradeStats = types.NewTradeStats(s.Symbol)
	}

	// StrategyController
	s.Status = types.StrategyStatusRunning

	s.OnSuspend(func() {
		// Cancel active orders
		_ = s.orderExecutor.GracefulCancel(ctx)
	})

	s.OnEmergencyStop(func() {
		// Cancel active orders
		_ = s.orderExecutor.GracefulCancel(ctx)
		// Close 100% position
		_ = s.orderExecutor.ClosePosition(ctx, fixedpoint.One)
	})

	// initial required information
	s.session = session

	// Set fee rate
	if s.session.MakerFeeRate.Sign() > 0 || s.session.TakerFeeRate.Sign() > 0 {
		s.Position.SetExchangeFeeRate(s.session.ExchangeName, types.ExchangeFee{
			MakerFeeRate: s.session.MakerFeeRate,
			TakerFeeRate: s.session.TakerFeeRate,
		})
	}

	s.orderExecutor = bbgo.NewGeneralOrderExecutor(session, s.Symbol, ID, instanceID, s.Position)
	s.orderExecutor.BindEnvironment(s.Environment)
	s.orderExecutor.BindProfitStats(s.ProfitStats)
	s.orderExecutor.BindTradeStats(s.TradeStats)

	// AccountValueCalculator
	s.AccountValueCalculator = bbgo.NewAccountValueCalculator(s.session, s.Market.QuoteCurrency)

	// Accumulated profit report
	if bbgo.IsBackTesting {
		if s.AccumulatedProfitReport == nil {
			s.AccumulatedProfitReport = &AccumulatedProfitReport{}
		}
		s.AccumulatedProfitReport.Initialize()
		s.orderExecutor.TradeCollector().OnProfit(func(trade types.Trade, profit *types.Profit) {
			if profit == nil {
				return
			}

			s.AccumulatedProfitReport.RecordProfit(profit.Profit)
		})
		session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, types.Interval1d, func(kline types.KLine) {
			s.AccumulatedProfitReport.DailyUpdate(s.TradeStats)
		}))
	}

	// For drawing
	profitSlice := floats.Slice{1., 1.}
	price, _ := session.LastPrice(s.Symbol)
	initAsset := s.CalcAssetValue(price).Float64()
	cumProfitSlice := floats.Slice{initAsset, initAsset}
	profitDollarSlice := floats.Slice{0, 0}
	cumProfitDollarSlice := floats.Slice{0, 0}

	s.orderExecutor.TradeCollector().OnTrade(func(trade types.Trade, profit fixedpoint.Value, netProfit fixedpoint.Value) {
		if bbgo.IsBackTesting {
			s.AccumulatedProfitReport.RecordTrade(trade.Fee)
		}

		// For drawing/charting
		price := trade.Price.Float64()
		if s.buyPrice > 0 {
			profitSlice.Update(price / s.buyPrice)
			cumProfitSlice.Update(s.CalcAssetValue(trade.Price).Float64())
		} else if s.sellPrice > 0 {
			profitSlice.Update(s.sellPrice / price)
			cumProfitSlice.Update(s.CalcAssetValue(trade.Price).Float64())
		}
		profitDollarSlice.Update(profit.Float64())
		cumProfitDollarSlice.Update(profitDollarSlice.Sum())
		if s.Position.IsDust(trade.Price) {
			s.buyPrice = 0
			s.sellPrice = 0
			s.highestPrice = 0
			s.lowestPrice = 0
		} else if s.Position.IsLong() {
			s.buyPrice = price
			s.sellPrice = 0
			s.highestPrice = s.buyPrice
			s.lowestPrice = 0
		} else {
			s.sellPrice = price
			s.buyPrice = 0
			s.highestPrice = 0
			s.lowestPrice = s.sellPrice
		}
	})

	s.InitDrawCommands(&profitSlice, &cumProfitSlice, &cumProfitDollarSlice)

	s.orderExecutor.TradeCollector().OnPositionUpdate(func(position *types.Position) {
		bbgo.Sync(ctx, s)
	})
	s.orderExecutor.Bind()
	s.activeOrders = bbgo.NewActiveOrderBook(s.Symbol)

	//back-test only, because 1s delayed a lot
	//kLineStore, _ := s.session.MarketDataStore(s.Symbol)
	//s.Nrr = &NRR{IntervalWindow: types.IntervalWindow{Window: 2, Interval: s.Interval}, RankingWindow: s.Window}
	//s.Nrr.BindK(s.session.MarketDataStream, s.Symbol, s.Interval)
	//if klines, ok := kLineStore.KLinesOfInterval(s.Nrr.Interval); ok {
	//	s.Nrr.LoadK((*klines)[0:])
	//}
	//s.Ma = &indicator.SMA{IntervalWindow: types.IntervalWindow{Window: s.Window, Interval: s.Interval}}
	//s.Ma.BindK(s.session.MarketDataStream, s.Symbol, s.Interval)
	//if klines, ok := kLineStore.KLinesOfInterval(s.Ma.Interval); ok {
	//	s.Ma.LoadK((*klines)[0:])
	//}

	s.rtNr = types.NewQueue(100)

	s.rtMaFast = types.NewQueue(1)
	s.rtMaSlow = types.NewQueue(5)
	s.rtMr = types.NewQueue(100)

	s.rtWeight = types.NewQueue(100)

	s.currentTradePrice = atomic.NewFloat64(0)

	if !bbgo.IsBackTesting {

		s.session.MarketDataStream.OnBookTickerUpdate(func(bt types.BookTicker) {
			// quote order book price
			s.obBuyPrice = atomic.NewFloat64(bt.Buy.Float64())
			s.obSellPrice = atomic.NewFloat64(bt.Sell.Float64())
		})

		s.session.MarketDataStream.OnAggTrade(func(trade types.Trade) {
			s.currentTradePrice = atomic.NewFloat64(trade.Price.Float64())
		})

		go func() {
			intervalCloseTicker := time.NewTicker(time.Duration(s.Interval) * time.Millisecond)
			defer intervalCloseTicker.Stop()

			for {
				select {
				case <-intervalCloseTicker.C:
					if s.currentTradePrice.Load() > 0 {
						s.closePrice = s.currentTradePrice.Load()
						//log.Infof("Close Price: %f", s.closePrice)
						// calculate real-time Negative Return
						s.rtNr.Update((s.openPrice - s.closePrice) / s.openPrice)
						// calculate real-time Negative Return Rank
						rtNrRank := 0.
						if s.rtNr.Length() > 2 {
							rtNrRank = s.rtNr.Rank(s.rtNr.Length()).Last() / float64(s.rtNr.Length())
						}
						// calculate real-time Mean Reversion
						s.rtMaFast.Update(s.closePrice)
						s.rtMaSlow.Update(s.closePrice)
						s.rtMr.Update((s.rtMaSlow.Mean() - s.rtMaFast.Mean()) / s.rtMaSlow.Mean())
						// calculate real-time Mean Reversion Rank
						rtMrRank := 0.
						if s.rtMr.Length() > 2 {
							rtMrRank = s.rtMr.Rank(s.rtMr.Length()).Last() / float64(s.rtMr.Length())
						}
						alpha := 0.
						if s.NR && s.MR {
							alpha = (rtNrRank + rtMrRank) / 2
						} else if s.NR && !s.MR {
							alpha = rtNrRank
						} else if !s.NR && s.MR {
							alpha = rtMrRank
						}
						s.rtWeight.Update(alpha)
						log.Infof("Alpha: %f/1.0", s.rtWeight.Last())
						s.rebalancePosition(s.obBuyPrice.Load(), s.obSellPrice.Load(), s.rtWeight.Last())
						s.orderExecutor.CancelNoWait(context.Background())
					}
				case <-s.stopC:
					log.Warnf("%s goroutine stopped, due to the stop signal", s.Symbol)
					return

				case <-ctx.Done():
					log.Warnf("%s goroutine stopped, due to the cancelled context", s.Symbol)
					return
				}
			}
		}()

		go func() {
			intervalOpenTicker := time.NewTicker(time.Duration(s.Interval) * time.Millisecond)
			defer intervalOpenTicker.Stop()
			for {
				select {
				case <-intervalOpenTicker.C:
					time.Sleep(200 * time.Microsecond)
					if s.currentTradePrice.Load() > 0 {
						s.openPrice = s.currentTradePrice.Load()
						//log.Infof("Open Price: %f", s.openPrice)
					}
				case <-s.stopC:
					log.Warnf("%s goroutine stopped, due to the stop signal", s.Symbol)
					return

				case <-ctx.Done():
					log.Warnf("%s goroutine stopped, due to the cancelled context", s.Symbol)
					return
				}
			}
		}()
	}

	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()
		// Output accumulated profit report
		if bbgo.IsBackTesting {
			defer s.AccumulatedProfitReport.Output(s.Symbol)

			if s.DrawGraph {
				if err := s.Draw(&profitSlice, &cumProfitSlice); err != nil {
					log.WithError(err).Errorf("cannot draw graph")
				}
			}
		} else {
			close(s.stopC)
		}
		_, _ = fmt.Fprintln(os.Stderr, s.TradeStats.String())
		_ = s.orderExecutor.GracefulCancel(ctx)
	})

	return nil
}

func (s *Strategy) CalcAssetValue(price fixedpoint.Value) fixedpoint.Value {
	balances := s.session.GetAccount().Balances()
	return balances[s.Market.BaseCurrency].Total().Mul(price).Add(balances[s.Market.QuoteCurrency].Total())
}

func (s *Strategy) rebalancePosition(bestBid, bestAsk float64, w float64) {
	// alpha-weighted assets (inventory and capital)
	position := s.orderExecutor.Position()
	p := fixedpoint.NewFromFloat((bestBid + bestAsk) / 2)

	targetBase := s.QuantityOrAmount.CalculateQuantity(p).Mul(fixedpoint.NewFromFloat(w))

	// to buy/sell quantity
	diffQty := targetBase.Sub(position.Base)
	log.Infof("Target Position Diff: %f", diffQty.Float64())

	// ignore small changes
	if diffQty.Abs().Float64() < 0.0005 {
		return
	}

	if diffQty.Sign() > 0 {
		_, err := s.orderExecutor.SubmitOrders(context.Background(), types.SubmitOrder{
			Symbol:   s.Symbol,
			Side:     types.SideTypeBuy,
			Quantity: diffQty.Abs(),
			Type:     types.OrderTypeLimit,
			Price:    fixedpoint.NewFromFloat(bestBid),
			Tag:      "irr re-balance: buy",
		})
		if err != nil {
			log.WithError(err)
		}
	} else if diffQty.Sign() < 0 {
		_, err := s.orderExecutor.SubmitOrders(context.Background(), types.SubmitOrder{
			Symbol:   s.Symbol,
			Side:     types.SideTypeSell,
			Quantity: diffQty.Abs(),
			Type:     types.OrderTypeLimit,
			Price:    fixedpoint.NewFromFloat(bestAsk),
			Tag:      "irr re-balance: sell",
		})
		if err != nil {
			log.WithError(err)
		}
	}
}
