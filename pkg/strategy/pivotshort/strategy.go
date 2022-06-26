package pivotshort

import (
	"context"
	"fmt"
	"os"
	"sort"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "pivotshort"

var one = fixedpoint.One
var zero = fixedpoint.Zero

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type IntervalWindowSetting struct {
	types.IntervalWindow
}

// BreakLow -- when price breaks the previous pivot low, we set a trade entry
type BreakLow struct {
	// Ratio is a number less than 1.0, price * ratio will be the price triggers the short order.
	Ratio fixedpoint.Value `json:"ratio"`

	// MarketOrder is the option to enable market order short.
	MarketOrder bool `json:"marketOrder"`

	// BounceRatio is a ratio used for placing the limit order sell price
	// limit sell price = breakLowPrice * (1 + BounceRatio)
	BounceRatio fixedpoint.Value `json:"bounceRatio"`

	Quantity     fixedpoint.Value      `json:"quantity"`
	StopEMARange fixedpoint.Value      `json:"stopEMARange"`
	StopEMA      *types.IntervalWindow `json:"stopEMA"`
}

type BounceShort struct {
	Enabled bool `json:"enabled"`

	types.IntervalWindow

	MinDistance fixedpoint.Value `json:"minDistance"`
	NumLayers   int              `json:"numLayers"`
	LayerSpread fixedpoint.Value `json:"layerSpread"`
	Quantity    fixedpoint.Value `json:"quantity"`
	Ratio       fixedpoint.Value `json:"ratio"`
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

type Strategy struct {
	*bbgo.Graceful

	Environment *bbgo.Environment
	Symbol      string `json:"symbol"`
	Market      types.Market

	// pivot interval and window
	types.IntervalWindow

	// persistence fields
	Position    *types.Position    `persistence:"position"`
	ProfitStats *types.ProfitStats `persistence:"profit_stats"`
	TradeStats  *types.TradeStats  `persistence:"trade_stats"`

	BreakLow BreakLow `json:"breakLow"`

	BounceShort *BounceShort `json:"bounceShort"`

	Entry       Entry        `json:"entry"`
	ExitMethods []ExitMethod `json:"exits"`

	session       *bbgo.ExchangeSession
	orderExecutor *bbgo.GeneralOrderExecutor

	stopLossPrice           fixedpoint.Value
	lastLow                 fixedpoint.Value
	pivot                   *indicator.Pivot
	resistancePivot         *indicator.Pivot
	stopEWMA                *indicator.EWMA
	pivotLowPrices          []fixedpoint.Value
	resistancePrices        []float64
	currentBounceShortPrice fixedpoint.Value

	// StrategyController
	bbgo.StrategyController
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: types.Interval1m})

	if s.BounceShort != nil && s.BounceShort.Enabled {
		session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.BounceShort.Interval})
	}
}

func (s *Strategy) useQuantityOrBaseBalance(quantity fixedpoint.Value) fixedpoint.Value {
	if quantity.IsZero() {
		if balance, ok := s.session.Account.Balance(s.Market.BaseCurrency); ok {
			bbgo.Notify("sell quantity is not set, submitting sell with all base balance: %s", balance.Available.String())
			quantity = balance.Available
		}
	}

	if quantity.IsZero() {
		log.Errorf("quantity is zero, can not submit sell order, please check settings")
	}

	return quantity
}

func (s *Strategy) placeLimitSell(ctx context.Context, price, quantity fixedpoint.Value) {
	_, _ = s.orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
		Symbol:           s.Symbol,
		Price:            price,
		Side:             types.SideTypeSell,
		Type:             types.OrderTypeLimit,
		Quantity:         quantity,
		MarginSideEffect: types.SideEffectTypeMarginBuy,
	})
}

func (s *Strategy) placeMarketSell(ctx context.Context, quantity fixedpoint.Value) {
	_, _ = s.orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
		Symbol:           s.Symbol,
		Side:             types.SideTypeSell,
		Type:             types.OrderTypeMarket,
		Quantity:         quantity,
		MarginSideEffect: types.SideEffectTypeMarginBuy,
	})
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s:%s", ID, s.Symbol)
}

func (s *Strategy) CurrentPosition() *types.Position {
	return s.Position
}

func (s *Strategy) ClosePosition(ctx context.Context, percentage fixedpoint.Value) error {
	bbgo.Notify("Closing position", s.Position)
	return s.orderExecutor.ClosePosition(ctx, percentage)
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
		s.TradeStats = &types.TradeStats{}
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
		_ = s.ClosePosition(ctx, fixedpoint.One)
	})

	// initial required information
	s.session = session
	s.orderExecutor = bbgo.NewGeneralOrderExecutor(session, s.Symbol, ID, instanceID, s.Position)
	s.orderExecutor.BindEnvironment(s.Environment)
	s.orderExecutor.BindProfitStats(s.ProfitStats)
	s.orderExecutor.BindTradeStats(s.TradeStats)
	s.orderExecutor.TradeCollector().OnPositionUpdate(func(position *types.Position) {
		bbgo.Sync(s)
	})
	s.orderExecutor.Bind()

	store, _ := session.MarketDataStore(s.Symbol)

	s.pivot = &indicator.Pivot{IntervalWindow: s.IntervalWindow}
	s.pivot.Bind(store)

	if s.BounceShort != nil && s.BounceShort.Enabled {
		s.resistancePivot = &indicator.Pivot{IntervalWindow: s.BounceShort.IntervalWindow}
		s.resistancePivot.Bind(store)
	}

	standardIndicator, _ := session.StandardIndicatorSet(s.Symbol)
	if s.BreakLow.StopEMA != nil {
		s.stopEWMA = standardIndicator.EWMA(*s.BreakLow.StopEMA)
	}

	s.lastLow = fixedpoint.Zero

	session.UserDataStream.OnStart(func() {
		lastKLine := s.preloadPivot(s.pivot, store)

		if s.resistancePivot != nil {
			s.preloadPivot(s.resistancePivot, store)
		}

		if lastKLine == nil {
			return
		}

		if s.resistancePivot != nil {
			lows := s.resistancePivot.Lows
			minDistance := s.BounceShort.MinDistance.Float64()
			closePrice := lastKLine.Close.Float64()
			s.resistancePrices = findPossibleResistancePrices(closePrice, minDistance, lows)
			log.Infof("last price: %f, possible resistance prices: %+v", closePrice, s.resistancePrices)

			if len(s.resistancePrices) > 0 {
				resistancePrice := fixedpoint.NewFromFloat(s.resistancePrices[0])
				if resistancePrice.Compare(s.currentBounceShortPrice) != 0 {
					log.Infof("updating resistance price... possible resistance prices: %+v", s.resistancePrices)

					_ = s.orderExecutor.GracefulCancel(ctx)

					s.currentBounceShortPrice = resistancePrice
					s.placeBounceSellOrders(ctx, s.currentBounceShortPrice)
				}
			}
		}
	})

	for _, method := range s.ExitMethods {
		method.Bind(session, s.orderExecutor)
	}

	// Always check whether you can open a short position or not
	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		if s.Status != types.StrategyStatusRunning {
			return
		}

		if kline.Symbol != s.Symbol || kline.Interval != types.Interval1m {
			return
		}

		isPositionOpened := !s.Position.IsClosed() && !s.Position.IsDust(kline.Close)
		if isPositionOpened && s.Position.IsShort() {
			return
		}

		if len(s.pivotLowPrices) == 0 {
			log.Infof("currently there is no pivot low prices, skip placing orders...")
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

		ratio := fixedpoint.One.Add(s.BreakLow.Ratio)
		breakPrice := previousLow.Mul(ratio)

		// if previous low is not break, skip
		if kline.Close.Compare(breakPrice) >= 0 {
			return
		}

		if !s.Position.IsClosed() && !s.Position.IsDust(kline.Close) {
			// s.Notify("skip opening %s position, which is not closed", s.Symbol, s.Position)
			return
		}

		_ = s.orderExecutor.GracefulCancel(ctx)

		quantity := s.useQuantityOrBaseBalance(s.BreakLow.Quantity)
		if s.BreakLow.MarketOrder {
			bbgo.Notify("%s price %f breaks the previous low %f with ratio %f, submitting market sell to open a short position", s.Symbol, kline.Close.Float64(), previousLow.Float64(), s.BreakLow.Ratio.Float64())
			s.placeMarketSell(ctx, quantity)
		} else {
			sellPrice := kline.Close.Mul(fixedpoint.One.Add(s.BreakLow.BounceRatio))
			s.placeLimitSell(ctx, sellPrice, quantity)
		}
	})

	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		// StrategyController
		if s.Status != types.StrategyStatusRunning {
			return
		}

		if s.BounceShort == nil || !s.BounceShort.Enabled {
			return
		}

		if kline.Symbol != s.Symbol || kline.Interval != s.BounceShort.Interval {
			return
		}

		if s.resistancePivot != nil {
			closePrice := kline.Close.Float64()
			minDistance := s.BounceShort.MinDistance.Float64()
			lows := s.resistancePivot.Lows
			s.resistancePrices = findPossibleResistancePrices(closePrice, minDistance, lows)

			if len(s.resistancePrices) > 0 {
				resistancePrice := fixedpoint.NewFromFloat(s.resistancePrices[0])
				if resistancePrice.Compare(s.currentBounceShortPrice) != 0 {
					log.Infof("updating resistance price... possible resistance prices: %+v", s.resistancePrices)

					_ = s.orderExecutor.GracefulCancel(ctx)

					s.currentBounceShortPrice = resistancePrice
					s.placeBounceSellOrders(ctx, s.currentBounceShortPrice)
				}
			}
		}
	})

	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		// StrategyController
		if s.Status != types.StrategyStatusRunning {
			return
		}

		if kline.Symbol != s.Symbol || kline.Interval != s.Interval {
			return
		}

		if s.pivot.LastLow() > 0.0 {
			lastLow := fixedpoint.NewFromFloat(s.pivot.LastLow())
			if lastLow.Compare(s.lastLow) != 0 {
				log.Infof("new pivot low detected: %f %s", s.pivot.LastLow(), kline.EndTime.Time())
			}

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

func (s *Strategy) findHigherPivotLow(price fixedpoint.Value) (fixedpoint.Value, bool) {
	for l := len(s.pivotLowPrices) - 1; l > 0; l-- {
		if s.pivotLowPrices[l].Compare(price) > 0 {
			return s.pivotLowPrices[l], true
		}
	}

	return price, false
}

func (s *Strategy) placeBounceSellOrders(ctx context.Context, resistancePrice fixedpoint.Value) {
	futuresMode := s.session.Futures || s.session.IsolatedFutures
	totalQuantity := s.BounceShort.Quantity
	numLayers := s.BounceShort.NumLayers
	if numLayers == 0 {
		numLayers = 1
	}

	numLayersF := fixedpoint.NewFromInt(int64(numLayers))

	layerSpread := s.BounceShort.LayerSpread
	quantity := totalQuantity.Div(numLayersF)

	log.Infof("placing bounce short orders: resistance price = %f, layer quantity = %f, num of layers = %d", resistancePrice.Float64(), quantity.Float64(), numLayers)

	for i := 0; i < numLayers; i++ {
		balances := s.session.GetAccount().Balances()
		quoteBalance := balances[s.Market.QuoteCurrency]
		baseBalance := balances[s.Market.BaseCurrency]

		// price = (resistance_price * (1.0 + ratio)) * ((1.0 + layerSpread) * i)
		price := resistancePrice.Mul(fixedpoint.One.Add(s.BounceShort.Ratio))
		spread := layerSpread.Mul(fixedpoint.NewFromInt(int64(i)))
		price = price.Add(spread)
		log.Infof("price = %f", price.Float64())

		log.Infof("placing bounce short order #%d: price = %f, quantity = %f", i, price.Float64(), quantity.Float64())

		if futuresMode {
			if quantity.Mul(price).Compare(quoteBalance.Available) <= 0 {
				s.placeOrder(ctx, price, quantity)
			}
		} else {
			if quantity.Compare(baseBalance.Available) <= 0 {
				s.placeOrder(ctx, price, quantity)
			}
		}
	}
}

func (s *Strategy) placeOrder(ctx context.Context, price fixedpoint.Value, quantity fixedpoint.Value) {
	_, _ = s.orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
		Symbol:   s.Symbol,
		Side:     types.SideTypeSell,
		Type:     types.OrderTypeLimitMaker,
		Price:    price,
		Quantity: quantity,
	})
}

func (s *Strategy) preloadPivot(pivot *indicator.Pivot, store *bbgo.MarketDataStore) *types.KLine {
	klines, ok := store.KLinesOfInterval(pivot.Interval)
	if !ok {
		return nil
	}

	last := (*klines)[len(*klines)-1]
	log.Infof("last %s price: %f", s.Symbol, last.Close.Float64())
	log.Debugf("updating pivot indicator: %d klines", len(*klines))

	for i := pivot.Window; i < len(*klines); i++ {
		pivot.Update((*klines)[0 : i+1])
	}

	log.Infof("found %s %v previous lows: %v", s.Symbol, pivot.IntervalWindow, pivot.Lows)
	log.Infof("found %s %v previous highs: %v", s.Symbol, pivot.IntervalWindow, pivot.Highs)
	return &last
}

func findPossibleResistancePrices(closePrice float64, minDistance float64, lows []float64) []float64 {
	// sort float64 in increasing order
	sort.Float64s(lows)

	var resistancePrices []float64
	for _, low := range lows {
		if low < closePrice {
			continue
		}

		last := closePrice
		if len(resistancePrices) > 0 {
			last = resistancePrices[len(resistancePrices)-1]
		}

		if (low / last) < (1.0 + minDistance) {
			continue
		}
		resistancePrices = append(resistancePrices, low)
	}

	return resistancePrices
}
