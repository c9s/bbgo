package pivotshort

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

type TradeStats struct {
	WinningRatio        fixedpoint.Value   `json:"winningRatio" yaml:"winningRatio"`
	NumOfLossTrade      int                `json:"numOfLossTrade" yaml:"numOfLossTrade"`
	NumOfProfitTrade    int                `json:"numOfProfitTrade" yaml:"numOfProfitTrade"`
	GrossProfit         fixedpoint.Value   `json:"grossProfit" yaml:"grossProfit"`
	GrossLoss           fixedpoint.Value   `json:"grossLoss" yaml:"grossLoss"`
	Profits             []fixedpoint.Value `json:"profits" yaml:"profits"`
	Losses              []fixedpoint.Value `json:"losses" yaml:"losses"`
	MostProfitableTrade fixedpoint.Value   `json:"mostProfitableTrade" yaml:"mostProfitableTrade"`
	MostLossTrade       fixedpoint.Value   `json:"mostLossTrade" yaml:"mostLossTrade"`
}

func (s *TradeStats) Add(pnl fixedpoint.Value) {
	if pnl.Sign() > 0 {
		s.NumOfProfitTrade++
		s.Profits = append(s.Profits, pnl)
		s.GrossProfit = s.GrossProfit.Add(pnl)
		s.MostProfitableTrade = fixedpoint.Max(s.MostProfitableTrade, pnl)
	} else {
		s.NumOfLossTrade++
		s.Losses = append(s.Losses, pnl)
		s.GrossLoss = s.GrossLoss.Add(pnl)
		s.MostLossTrade = fixedpoint.Min(s.MostLossTrade, pnl)
	}

	if s.NumOfLossTrade == 0 && s.NumOfProfitTrade > 0 {
		s.WinningRatio = fixedpoint.One
	} else {
		s.WinningRatio = fixedpoint.NewFromFloat(float64(s.NumOfProfitTrade) / float64(s.NumOfLossTrade))
	}
}

func (s *TradeStats) String() string {
	out, _ := yaml.Marshal(s)
	return string(out)
}

const ID = "pivotshort"

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type IntervalWindowSetting struct {
	types.IntervalWindow
}

// BreakLow -- when price breaks the previous pivot low, we set a trade entry
type BreakLow struct {
	Ratio        fixedpoint.Value      `json:"ratio"`
	MarketOrder  bool                  `json:"marketOrder"`
	BounceRatio  fixedpoint.Value      `json:"bounceRatio"`
	Quantity     fixedpoint.Value      `json:"quantity"`
	StopEMARange fixedpoint.Value      `json:"stopEMARange"`
	StopEMA      *types.IntervalWindow `json:"stopEMA"`
}

type Entry struct {
	CatBounceRatio fixedpoint.Value `json:"catBounceRatio"`
	NumLayers      int              `json:"numLayers"`
	TotalQuantity  fixedpoint.Value `json:"totalQuantity"`

	Quantity         fixedpoint.Value                `json:"quantity"`
	MarginSideEffect types.MarginOrderSideEffectType `json:"marginOrderSideEffect"`
}

type CumulatedVolume struct {
	Enabled        bool             `json:"enabled"`
	MinQuoteVolume fixedpoint.Value `json:"minQuoteVolume"`
	Window         int              `json:"window"`
}

type Exit struct {
	RoiStopLossPercentage      fixedpoint.Value `json:"roiStopLossPercentage"`
	RoiTakeProfitPercentage    fixedpoint.Value `json:"roiTakeProfitPercentage"`
	RoiMinTakeProfitPercentage fixedpoint.Value `json:"roiMinTakeProfitPercentage"`

	LowerShadowRatio fixedpoint.Value `json:"lowerShadowRatio"`

	CumulatedVolume *CumulatedVolume `json:"cumulatedVolume"`

	MarginSideEffect types.MarginOrderSideEffectType `json:"marginOrderSideEffect"`
}

type Strategy struct {
	*bbgo.Graceful
	*bbgo.Notifiability
	*bbgo.Persistence

	Environment *bbgo.Environment
	Symbol      string `json:"symbol"`
	Market      types.Market

	// pivot interval and window
	types.IntervalWindow

	// persistence fields
	Position    *types.Position    `json:"position,omitempty" persistence:"position"`
	ProfitStats *types.ProfitStats `json:"profitStats,omitempty" persistence:"profit_stats"`
	TradeStats  *TradeStats        `persistence:"trade_stats"`

	BreakLow BreakLow `json:"breakLow"`
	Entry    Entry    `json:"entry"`
	Exit     Exit     `json:"exit"`

	activeMakerOrders *bbgo.ActiveOrderBook
	orderStore        *bbgo.OrderStore
	tradeCollector    *bbgo.TradeCollector

	session *bbgo.ExchangeSession

	lastLow        fixedpoint.Value
	pivot          *indicator.Pivot
	stopEWMA       *indicator.EWMA
	pivotLowPrices []fixedpoint.Value

	// StrategyController
	bbgo.StrategyController
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	log.Infof("subscribe %s", s.Symbol)
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: types.Interval1m})
}

func (s *Strategy) submitOrders(ctx context.Context, orderExecutor bbgo.OrderExecutor, submitOrders ...types.SubmitOrder) {
	createdOrders, err := orderExecutor.SubmitOrders(ctx, submitOrders...)
	if err != nil {
		log.WithError(err).Errorf("can not place orders")
	}

	s.orderStore.Add(createdOrders...)
	s.activeMakerOrders.Add(createdOrders...)
	s.tradeCollector.Process()
}

func (s *Strategy) useQuantityOrBaseBalance(quantity fixedpoint.Value) fixedpoint.Value {
	if quantity.IsZero() {
		if balance, ok := s.session.Account.Balance(s.Market.BaseCurrency); ok {
			s.Notify("sell quantity is not set, submitting sell with all base balance: %s", balance.Available.String())
			quantity = balance.Available
		}
	}

	if quantity.IsZero() {
		log.Errorf("quantity is zero, can not submit sell order, please check settings")
	}

	return quantity
}

func (s *Strategy) placeLimitSell(ctx context.Context, orderExecutor bbgo.OrderExecutor, price, quantity fixedpoint.Value) {
	s.submitOrders(ctx, orderExecutor, types.SubmitOrder{
		Symbol:           s.Symbol,
		Price:            price,
		Side:             types.SideTypeSell,
		Type:             types.OrderTypeLimit,
		Quantity:         quantity,
		MarginSideEffect: types.SideEffectTypeMarginBuy,
	})
}

func (s *Strategy) placeMarketSell(ctx context.Context, orderExecutor bbgo.OrderExecutor, quantity fixedpoint.Value) {
	s.submitOrders(ctx, orderExecutor, types.SubmitOrder{
		Symbol:           s.Symbol,
		Side:             types.SideTypeSell,
		Type:             types.OrderTypeMarket,
		Quantity:         quantity,
		MarginSideEffect: types.SideEffectTypeMarginBuy,
	})
}

func (s *Strategy) ClosePosition(ctx context.Context, percentage fixedpoint.Value) error {
	submitOrder := s.Position.NewMarketCloseOrder(percentage) // types.SubmitOrder{
	if submitOrder == nil {
		return nil
	}

	if s.session.Margin {
		submitOrder.MarginSideEffect = s.Exit.MarginSideEffect
	}

	s.Notify("Closing %s position by %f", s.Symbol, percentage.Float64())

	createdOrders, err := s.session.Exchange.SubmitOrders(ctx, *submitOrder)
	if err != nil {
		log.WithError(err).Errorf("can not place position close order")
	}

	s.orderStore.Add(createdOrders...)
	s.activeMakerOrders.Add(createdOrders...)
	s.tradeCollector.Process()
	return err
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s:%s", ID, s.Symbol)
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	// initial required information
	s.session = session

	s.activeMakerOrders = bbgo.NewActiveOrderBook(s.Symbol)
	s.activeMakerOrders.BindStream(session.UserDataStream)

	s.orderStore = bbgo.NewOrderStore(s.Symbol)
	s.orderStore.BindStream(session.UserDataStream)

	if s.Position == nil {
		s.Position = types.NewPositionFromMarket(s.Market)
	}

	if s.ProfitStats == nil {
		s.ProfitStats = types.NewProfitStats(s.Market)
	}

	if s.TradeStats == nil {
		s.TradeStats = &TradeStats{}
	}

	instanceID := s.InstanceID()

	// Always update the position fields
	s.Position.Strategy = ID
	s.Position.StrategyInstanceID = instanceID

	s.tradeCollector = bbgo.NewTradeCollector(s.Symbol, s.Position, s.orderStore)
	s.tradeCollector.OnTrade(func(trade types.Trade, profit, netProfit fixedpoint.Value) {
		s.Notifiability.Notify(trade)
		s.ProfitStats.AddTrade(trade)

		if profit.Compare(fixedpoint.Zero) == 0 {
			s.Environment.RecordPosition(s.Position, trade, nil)
		} else {
			log.Infof("%s generated profit: %v", s.Symbol, profit)

			p := s.Position.NewProfit(trade, profit, netProfit)
			p.Strategy = ID
			p.StrategyInstanceID = instanceID
			s.Notify(&p)

			s.ProfitStats.AddProfit(p)
			s.Notify(&s.ProfitStats)

			s.TradeStats.Add(profit)

			s.Environment.RecordPosition(s.Position, trade, &p)
		}
	})

	s.tradeCollector.OnPositionUpdate(func(position *types.Position) {
		log.Infof("position changed: %s", s.Position)
		s.Notify(s.Position)
	})
	s.tradeCollector.BindStream(session.UserDataStream)

	store, _ := session.MarketDataStore(s.Symbol)
	s.pivot = &indicator.Pivot{IntervalWindow: s.IntervalWindow}
	s.pivot.Bind(store)

	standardIndicator, _ := session.StandardIndicatorSet(s.Symbol)
	if s.BreakLow.StopEMA != nil {
		s.stopEWMA = standardIndicator.EWMA(*s.BreakLow.StopEMA)
	}

	s.lastLow = fixedpoint.Zero

	session.UserDataStream.OnStart(func() {
		if klines, ok := store.KLinesOfInterval(s.Interval); ok {
			last := (*klines)[len(*klines)-1]
			log.Debugf("updating pivot indicator: %d klines", len(*klines))
			for i := s.pivot.Window; i < len(*klines); i++ {
				s.pivot.Update((*klines)[0 : i+1])
			}

			log.Infof("current %s price: %f", s.Symbol, last.Close.Float64())
			log.Infof("found %s previous lows: %v", s.Symbol, s.pivot.Lows)
			log.Infof("found %s previous highs: %v", s.Symbol, s.pivot.Highs)
		}
		// s.placeBounceSellOrders(ctx, limitPrice, price, orderExecutor)
	})

	// Always check whether you can open a short position or not
	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		if kline.Symbol != s.Symbol || kline.Interval != types.Interval1m {
			return
		}

		isPositionOpened := !s.Position.IsClosed() && !s.Position.IsDust(kline.Close)

		if isPositionOpened && s.Position.IsShort() {
			// calculate return rate
			// TODO: apply quantity to this formula
			roi := s.Position.AverageCost.Sub(kline.Close).Div(s.Position.AverageCost)
			if roi.Compare(s.Exit.RoiStopLossPercentage.Neg()) < 0 {
				// stop loss
				s.Notify("%s ROI StopLoss triggered at price %f: Loss %s", s.Symbol, kline.Close.Float64(), roi.Percentage())
				s.closePosition(ctx)
				return
			} else {
				// take profit
				if roi.Compare(s.Exit.RoiTakeProfitPercentage) > 0 { // force take profit
					s.Notify("%s TakeProfit triggered at price %f: by ROI percentage %s", s.Symbol, kline.Close.Float64(), roi.Percentage(), kline)
					s.closePosition(ctx)
					return
				} else if !s.Exit.RoiMinTakeProfitPercentage.IsZero() && roi.Compare(s.Exit.RoiMinTakeProfitPercentage) > 0 {
					if !s.Exit.LowerShadowRatio.IsZero() && kline.GetLowerShadowHeight().Div(kline.Close).Compare(s.Exit.LowerShadowRatio) > 0 {
						s.Notify("%s TakeProfit triggered at price %f: by shadow ratio %f",
							s.Symbol,
							kline.Close.Float64(),
							kline.GetLowerShadowRatio().Float64(), kline)
						s.closePosition(ctx)
						return
					} else if s.Exit.CumulatedVolume != nil && s.Exit.CumulatedVolume.Enabled {
						if klines, ok := store.KLinesOfInterval(s.Interval); ok {
							var cbv = fixedpoint.Zero
							var cqv = fixedpoint.Zero
							for i := 0; i < s.Exit.CumulatedVolume.Window; i++ {
								last := (*klines)[len(*klines)-1-i]
								cqv = cqv.Add(last.QuoteVolume)
								cbv = cbv.Add(last.Volume)
							}

							if cqv.Compare(s.Exit.CumulatedVolume.MinQuoteVolume) > 0 {
								s.Notify("%s TakeProfit triggered at price %f: by cumulated volume (window: %d) %f > %f",
									s.Symbol,
									kline.Close.Float64(),
									s.Exit.CumulatedVolume.Window,
									cqv.Float64(),
									s.Exit.CumulatedVolume.MinQuoteVolume.Float64())
								s.closePosition(ctx)
								return
							}
						}
					}
				}
			}
		}

		if len(s.pivotLowPrices) == 0 {
			return
		}

		previousLow := s.pivotLowPrices[len(s.pivotLowPrices)-1]

		// truncate the pivot low prices
		if len(s.pivotLowPrices) > 10 {
			s.pivotLowPrices = s.pivotLowPrices[len(s.pivotLowPrices)-10:]
		}

		if s.stopEWMA != nil && !s.BreakLow.StopEMARange.IsZero() {
			ema := fixedpoint.NewFromFloat(s.stopEWMA.Last())
			if ema.IsZero() {
				return
			}

			emaStopShortPrice := ema.Mul(fixedpoint.One.Sub(s.BreakLow.StopEMARange))
			if kline.Close.Compare(emaStopShortPrice) < 0 {
				return
			}
		}

		ratio := fixedpoint.One.Sub(s.BreakLow.Ratio)
		breakPrice := previousLow.Mul(ratio)
		if kline.Close.Compare(breakPrice) > 0 {
			return
		}

		if !s.Position.IsClosed() && !s.Position.IsDust(kline.Close) {
			// s.Notify("skip opening %s position, which is not closed", s.Symbol, s.Position)
			return
		}

		if err := s.activeMakerOrders.GracefulCancel(ctx, s.session.Exchange); err != nil {
			log.WithError(err).Errorf("graceful cancel order error")
		}

		quantity := s.useQuantityOrBaseBalance(s.BreakLow.Quantity)
		if s.BreakLow.MarketOrder {
			s.Notify("%s price %f breaks the previous low %f with ratio %f, submitting market sell to open a short position", s.Symbol, kline.Close.Float64(), previousLow.Float64(), s.BreakLow.Ratio.Float64())
			s.placeMarketSell(ctx, orderExecutor, quantity)
		} else {
			sellPrice := kline.Close.Mul(fixedpoint.One.Add(s.BreakLow.BounceRatio))
			s.placeLimitSell(ctx, orderExecutor, sellPrice, quantity)
		}
	})

	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		if kline.Symbol != s.Symbol || kline.Interval != s.Interval {
			return
		}

		if s.pivot.LastLow() > 0.0 {
			log.Debugf("pivot low detected: %f %s", s.pivot.LastLow(), kline.EndTime.Time())
			lastLow := fixedpoint.NewFromFloat(s.pivot.LastLow())
			s.lastLow = lastLow
			s.pivotLowPrices = append(s.pivotLowPrices, s.lastLow)
		}
	})

	s.Graceful.OnShutdown(func(ctx context.Context, wg *sync.WaitGroup) {
		_, _ = fmt.Fprintln(os.Stderr, s.TradeStats.String())
		wg.Done()
	})

	return nil
}

func (s *Strategy) closePosition(ctx context.Context) {
	if err := s.activeMakerOrders.GracefulCancel(ctx, s.session.Exchange); err != nil {
		log.WithError(err).Errorf("graceful cancel order error")
	}

	if err := s.ClosePosition(ctx, fixedpoint.One); err != nil {
		log.WithError(err).Errorf("close position error")
	}
}

func (s *Strategy) findHigherPivotLow(price fixedpoint.Value) (fixedpoint.Value, bool) {
	for l := len(s.pivotLowPrices) - 1; l > 0; l-- {
		if s.pivotLowPrices[l].Compare(price) > 0 {
			return s.pivotLowPrices[l], true
		}
	}

	return price, false
}

func (s *Strategy) placeBounceSellOrders(ctx context.Context, lastLow fixedpoint.Value, limitPrice fixedpoint.Value, currentPrice fixedpoint.Value, orderExecutor bbgo.OrderExecutor) {
	futuresMode := s.session.Futures || s.session.IsolatedFutures
	numLayers := fixedpoint.NewFromInt(int64(s.Entry.NumLayers))
	d := s.Entry.CatBounceRatio.Div(numLayers)
	q := s.Entry.Quantity
	if !s.Entry.TotalQuantity.IsZero() {
		q = s.Entry.TotalQuantity.Div(numLayers)
	}

	for i := 0; i < s.Entry.NumLayers; i++ {
		balances := s.session.GetAccount().Balances()
		quoteBalance, _ := balances[s.Market.QuoteCurrency]
		baseBalance, _ := balances[s.Market.BaseCurrency]

		p := limitPrice.Mul(fixedpoint.One.Add(s.Entry.CatBounceRatio.Sub(fixedpoint.NewFromFloat(d.Float64() * float64(i)))))

		if futuresMode {
			if q.Mul(p).Compare(quoteBalance.Available) <= 0 {
				s.placeOrder(ctx, lastLow, p, currentPrice, q, orderExecutor)
			}
		} else if s.Environment.IsBackTesting() {
			if q.Compare(baseBalance.Available) <= 0 {
				s.placeOrder(ctx, lastLow, p, currentPrice, q, orderExecutor)
			}
		} else {
			if q.Compare(baseBalance.Available) <= 0 {
				s.placeOrder(ctx, lastLow, p, currentPrice, q, orderExecutor)
			}
		}
	}
}

func (s *Strategy) placeOrder(ctx context.Context, lastLow fixedpoint.Value, limitPrice fixedpoint.Value, currentPrice fixedpoint.Value, qty fixedpoint.Value, orderExecutor bbgo.OrderExecutor) {
	submitOrder := types.SubmitOrder{
		Symbol:   s.Symbol,
		Side:     types.SideTypeSell,
		Type:     types.OrderTypeLimit,
		Price:    limitPrice,
		Quantity: qty,
	}

	if !lastLow.IsZero() && lastLow.Compare(currentPrice) <= 0 {
		submitOrder.Type = types.OrderTypeMarket
	}

	s.submitOrders(ctx, orderExecutor, submitOrder)
}
