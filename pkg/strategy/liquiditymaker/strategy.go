package liquiditymaker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/dbg"
	"github.com/c9s/bbgo/pkg/dynamic"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/strategy/common"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
	"github.com/c9s/bbgo/pkg/util/tradingutil"
)

const ID = "liquiditymaker"
const IDAlias = "liqmaker"

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
	bbgo.RegisterStrategy(IDAlias, &Strategy{})
}

// Strategy is the strategy struct of LiquidityMaker
// liquidity maker does not care about the current price, it tries to place liquidity orders (limit maker orders)
// around the current mid price
// liquidity maker's target:
// - place enough total liquidity amount on the order book, for example, 20k USDT value liquidity on both sell and buy
// - ensure the spread by placing the orders from the mid price (or the last trade price)
type Strategy struct {
	*common.Strategy

	Environment *bbgo.Environment
	Market      types.Market

	Symbol string `json:"symbol"`

	LiquidityUpdateInterval types.Interval `json:"liquidityUpdateInterval"`

	AdjustmentUpdateInterval   types.Interval   `json:"adjustmentUpdateInterval"`
	AdjustmentOrderMaxQuantity fixedpoint.Value `json:"adjustmentOrderMaxQuantity"`
	AdjustmentOrderPriceType   types.PriceType  `json:"adjustmentOrderPriceType"`

	NumOfLiquidityLayers int             `json:"numOfLiquidityLayers"`
	LiquiditySlideRule   *bbgo.SlideRule `json:"liquidityScale"`

	LiquidityPriceRange    fixedpoint.Value `json:"liquidityPriceRange"`
	AskLiquidityPriceRange fixedpoint.Value `json:"askLiquidityPriceRange"`
	BidLiquidityPriceRange fixedpoint.Value `json:"bidLiquidityPriceRange"`

	AskLiquidityAmount fixedpoint.Value `json:"askLiquidityAmount"`
	BidLiquidityAmount fixedpoint.Value `json:"bidLiquidityAmount"`

	StopBidPrice fixedpoint.Value `json:"stopBidPrice"`
	StopAskPrice fixedpoint.Value `json:"stopAskPrice"`

	MidPriceEMA *struct {
		Enabled      bool             `json:"enabled"`
		MaxBiasRatio fixedpoint.Value `json:"maxBiasRatio"`
		types.IntervalWindow
	} `json:"midPriceEMA"`

	midPriceEMA *indicatorv2.EWMAStream

	StopEMA *struct {
		Enabled bool `json:"enabled"`
		types.IntervalWindow
	} `json:"stopEMA"`

	stopEMA *indicatorv2.EWMAStream

	UseProtectedPriceRange bool `json:"useProtectedPriceRange"`

	UseLastTradePrice bool             `json:"useLastTradePrice"`
	Spread            fixedpoint.Value `json:"spread"`
	MaxPrice          fixedpoint.Value `json:"maxPrice"`
	MinPrice          fixedpoint.Value `json:"minPrice"`

	MaxPositionExposure fixedpoint.Value `json:"maxPositionExposure"`

	MinProfit fixedpoint.Value `json:"minProfit"`

	common.ProfitFixerBundle

	liquidityOrderBook, adjustmentOrderBook *bbgo.ActiveOrderBook

	liquidityScale bbgo.Scale

	orderGenerator *LiquidityOrderGenerator

	logger        log.FieldLogger
	metricsLabels prometheus.Labels
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s:%s", ID, s.Symbol)
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.AdjustmentUpdateInterval})
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.LiquidityUpdateInterval})
	session.Subscribe(types.BookChannel, s.Symbol, types.SubscribeOptions{
		Depth: types.DepthLevelFull,
	})

	if s.StopEMA != nil && s.StopEMA.Interval != "" {
		session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.StopEMA.Interval})
	}

	if s.MidPriceEMA != nil && s.MidPriceEMA.Interval != "" {
		session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.MidPriceEMA.Interval})
	}
}

func (s *Strategy) Defaults() error {
	if s.AdjustmentOrderPriceType == "" {
		s.AdjustmentOrderPriceType = types.PriceTypeMaker
	}

	if s.LiquidityUpdateInterval == "" {
		s.LiquidityUpdateInterval = types.Interval1h
	}

	if s.AdjustmentUpdateInterval == "" {
		s.AdjustmentUpdateInterval = types.Interval5m
	}

	if s.AskLiquidityPriceRange.IsZero() {
		s.AskLiquidityPriceRange = s.LiquidityPriceRange
	}

	if s.BidLiquidityPriceRange.IsZero() {
		s.BidLiquidityPriceRange = s.LiquidityPriceRange
	}

	return nil
}

func (s *Strategy) Initialize() error {
	if s.Strategy == nil {
		s.Strategy = &common.Strategy{}
	}

	s.logger = log.WithFields(log.Fields{
		"symbol":      s.Symbol,
		"strategy":    ID,
		"strategy_id": s.InstanceID(),
	})

	s.metricsLabels = prometheus.Labels{
		"strategy_type": ID,
		"strategy_id":   s.InstanceID(),
		"exchange":      "", // FIXME
		"symbol":        s.Symbol,
	}

	return nil
}

func (s *Strategy) updateMarketMetrics(ctx context.Context) error {
	ticker, err := s.Session.Exchange.QueryTicker(ctx, s.Symbol)
	if err != nil {
		return err
	}

	currentSpread := ticker.Sell.Sub(ticker.Buy)

	tickerBidMetrics.With(s.metricsLabels).Set(ticker.Buy.Float64())
	tickerAskMetrics.With(s.metricsLabels).Set(ticker.Sell.Float64())
	spreadMetrics.With(s.metricsLabels).Set(currentSpread.Float64())
	return nil
}

func (s *Strategy) liquidityWorker(ctx context.Context, interval types.Interval) {
	s.logger.Infof("starting liquidity worker with interval %v", interval)

	metricsTicker := time.NewTicker(5 * time.Second)
	defer metricsTicker.Stop()

	ticker := time.NewTicker(interval.Duration())
	defer ticker.Stop()

	s.placeLiquidityOrders(ctx)
	for {
		select {
		case <-ctx.Done():
			return

		case <-metricsTicker.C:
			if err := s.updateMarketMetrics(ctx); err != nil {
				s.logger.WithError(err).Errorf("unable to update market metrics")
			}

		case <-ticker.C:
			s.placeLiquidityOrders(ctx)
		}
	}
}

func (s *Strategy) Run(ctx context.Context, _ bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	if s.ProfitFixerBundle.ProfitFixerConfig != nil {
		market, _ := session.Market(s.Symbol)
		s.Position = types.NewPositionFromMarket(market)
		s.ProfitStats = types.NewProfitStats(market)

		if err := s.ProfitFixerBundle.Fix(ctx, s.Symbol, s.Position, s.ProfitStats, session); err != nil {
			return err
		}

		bbgo.Notify("Fixed %s position", s.Symbol, s.Position)
		bbgo.Notify("Fixed %s profitStats", s.Symbol, s.ProfitStats)
	}

	s.Strategy.Initialize(ctx, s.Environment, session, s.Market, ID, s.InstanceID())

	if s.StopEMA != nil && s.StopEMA.Enabled {
		s.stopEMA = session.Indicators(s.Symbol).EMA(s.StopEMA.IntervalWindow)
	}

	if s.MidPriceEMA != nil && s.MidPriceEMA.Enabled {
		s.midPriceEMA = session.Indicators(s.Symbol).EMA(s.MidPriceEMA.IntervalWindow)
	}

	instanceID := s.InstanceID()
	if err := dynamic.InitializeConfigMetrics(ID, instanceID, s); err != nil {
		return err
	}

	s.metricsLabels["exchange"] = session.ExchangeName.String()

	s.orderGenerator = &LiquidityOrderGenerator{
		Symbol: s.Symbol,
		Market: s.Market,
		logger: s.logger,
	}

	if s.AskLiquidityPriceRange.IsZero() {
		s.AskLiquidityPriceRange = fixedpoint.NewFromFloat(0.02)
	}

	if s.BidLiquidityPriceRange.IsZero() {
		s.BidLiquidityPriceRange = fixedpoint.NewFromFloat(0.02)
	}

	s.liquidityOrderBook = bbgo.NewActiveOrderBook(s.Symbol)
	s.liquidityOrderBook.BindStream(session.UserDataStream)

	s.adjustmentOrderBook = bbgo.NewActiveOrderBook(s.Symbol)
	s.adjustmentOrderBook.BindStream(session.UserDataStream)

	scale, err := s.LiquiditySlideRule.Scale()
	if err != nil {
		return err
	}

	if err := scale.Solve(); err != nil {
		return err
	}

	s.liquidityScale = scale

	if err := tradingutil.UniversalCancelAllOrders(ctx, session.Exchange, s.Symbol, nil); err != nil {
		return err
	}

	s.Position.UpdateMetrics()

	session.MarketDataStream.OnKLineClosed(func(k types.KLine) {
		if k.Interval == s.AdjustmentUpdateInterval {
			s.placeAdjustmentOrders(ctx)
		}
	})

	if intervalProvider, ok := session.Exchange.(types.CustomIntervalProvider); ok && intervalProvider.IsSupportedInterval(s.LiquidityUpdateInterval) {
		session.UserDataStream.OnAuth(func() {
			s.placeLiquidityOrders(ctx)
		})
		session.MarketDataStream.OnKLineClosed(func(k types.KLine) {
			if k.Interval == s.LiquidityUpdateInterval {
				s.placeLiquidityOrders(ctx)
			}
		})
	} else {
		session.UserDataStream.OnStart(func() {
			go s.liquidityWorker(ctx, s.LiquidityUpdateInterval)
		})
	}

	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()

		// sync trades before canceling orders
		// the order cancelation may take some time, we need to wait for the order cancelation to complete
		s.OrderExecutor.TradeCollector().Process()
		bbgo.Sync(ctx, s)

		if err := s.liquidityOrderBook.GracefulCancel(ctx, s.Session.Exchange); err != nil {
			util.LogErr(err, "unable to cancel liquidity orders")
		}

		if err := s.adjustmentOrderBook.GracefulCancel(ctx, s.Session.Exchange); err != nil {
			util.LogErr(err, "unable to cancel adjustment orders")
		}

		if err := tradingutil.UniversalCancelAllOrders(ctx, s.Session.Exchange, s.Symbol, nil); err != nil {
			util.LogErr(err, "unable to cancel all orders")
		}

		s.OrderExecutor.TradeCollector().Process()
		bbgo.Sync(ctx, s)
	})

	return nil
}

func (s *Strategy) placeAdjustmentOrders(ctx context.Context) {

	_ = s.adjustmentOrderBook.GracefulCancel(ctx, s.Session.Exchange)

	if s.Position.IsDust() {
		return
	}

	ticker, err := s.Session.Exchange.QueryTicker(ctx, s.Symbol)
	if util.LogErr(err, "unable to query ticker") {
		return
	}

	if _, err := s.Session.UpdateAccount(ctx); err != nil {
		util.LogErr(err, "unable to update account")
		return
	}

	baseBal, _ := s.Session.Account.Balance(s.Market.BaseCurrency)
	quoteBal, _ := s.Session.Account.Balance(s.Market.QuoteCurrency)

	var adjOrders []types.SubmitOrder

	posSize := s.Position.Base.Abs()

	if !s.AdjustmentOrderMaxQuantity.IsZero() {
		posSize = fixedpoint.Min(posSize, s.AdjustmentOrderMaxQuantity)
	}

	tickSize := s.Market.TickSize

	if s.Position.IsShort() {
		price := s.AdjustmentOrderPriceType.GetPrice(ticker, types.SideTypeBuy)
		price = profitProtectedPrice(types.SideTypeBuy, s.Position.AverageCost, price.Add(tickSize.Neg()), s.Session.MakerFeeRate, s.MinProfit)
		quoteQuantity := fixedpoint.Min(price.Mul(posSize), quoteBal.Available)
		bidQuantity := quoteQuantity.Div(price)

		if s.Market.IsDustQuantity(bidQuantity, price) {
			return
		}

		adjOrders = append(adjOrders, types.SubmitOrder{
			Symbol:      s.Symbol,
			Type:        types.OrderTypeLimitMaker,
			Side:        types.SideTypeBuy,
			Price:       price,
			Quantity:    bidQuantity,
			Market:      s.Market,
			TimeInForce: types.TimeInForceGTC,
		})
	} else if s.Position.IsLong() {
		price := s.AdjustmentOrderPriceType.GetPrice(ticker, types.SideTypeSell)
		price = profitProtectedPrice(types.SideTypeSell, s.Position.AverageCost, price.Add(tickSize), s.Session.MakerFeeRate, s.MinProfit)
		askQuantity := fixedpoint.Min(posSize, baseBal.Available)

		if s.Market.IsDustQuantity(askQuantity, price) {
			return
		}

		adjOrders = append(adjOrders, types.SubmitOrder{
			Symbol:      s.Symbol,
			Type:        types.OrderTypeLimitMaker,
			Side:        types.SideTypeSell,
			Price:       price,
			Quantity:    askQuantity,
			Market:      s.Market,
			TimeInForce: types.TimeInForceGTC,
		})
	}

	createdOrders, err := s.OrderExecutor.SubmitOrders(ctx, adjOrders...)
	if util.LogErr(err, "unable to place liquidity orders") {
		return
	}

	s.adjustmentOrderBook.Add(createdOrders...)
}

func (s *Strategy) placeLiquidityOrders(ctx context.Context) {
	s.OrderExecutor.TradeCollector().Process()
	bbgo.Sync(ctx, s)

	err := s.liquidityOrderBook.GracefulCancel(ctx, s.Session.Exchange)
	if util.LogErr(err, "unable to cancel orders") {
		return
	}

	ticker, err := s.Session.Exchange.QueryTicker(ctx, s.Symbol)
	if util.LogErr(err, "unable to query ticker") {
		return
	}

	if s.IsHalted(ticker.Time) {
		s.logger.Warn("circuitBreakRiskControl: trading halted")
		return
	}

	if _, err := s.Session.UpdateAccount(ctx); err != nil {
		util.LogErr(err, "unable to update account")
		return
	}

	baseBal, _ := s.Session.Account.Balance(s.Market.BaseCurrency)
	quoteBal, _ := s.Session.Account.Balance(s.Market.QuoteCurrency)

	if ticker.Buy.IsZero() && ticker.Sell.IsZero() {
		ticker.Sell = ticker.Last.Add(s.Market.TickSize)
		ticker.Buy = ticker.Last.Sub(s.Market.TickSize)
	} else if ticker.Buy.IsZero() {
		ticker.Buy = ticker.Sell.Sub(s.Market.TickSize)
	} else if ticker.Sell.IsZero() {
		ticker.Sell = ticker.Buy.Add(s.Market.TickSize)
	}

	s.logger.Infof("ticker: %+v", ticker)

	lastTradedPrice := ticker.GetValidPrice()
	midPrice := ticker.Sell.Add(ticker.Buy).Div(fixedpoint.Two)
	currentSpread := ticker.Sell.Sub(ticker.Buy)
	sideSpread := s.Spread.Div(fixedpoint.Two)

	if !lastTradedPrice.IsZero() && (s.UseLastTradePrice || midPrice.IsZero()) {
		midPrice = lastTradedPrice
	}

	midPriceMetrics.With(s.metricsLabels).Set(midPrice.Float64())

	s.logger.Infof("current spread: %f lastTradedPrice: %f midPrice: %f", currentSpread.Float64(), lastTradedPrice.Float64(), midPrice.Float64())

	placeBid := true
	placeAsk := true

	if s.StopBidPrice.Sign() > 0 && midPrice.Compare(s.StopBidPrice) > 0 {
		s.logger.Infof("mid price %f > stop bid price %f, turning off bid orders", midPrice.Float64(), s.StopBidPrice.Float64())
		placeBid = false
	}

	if s.StopAskPrice.Sign() > 0 && midPrice.Compare(s.StopAskPrice) < 0 {
		s.logger.Infof("mid price %f < stop ask price %f, turning off ask orders", midPrice.Float64(), s.StopAskPrice.Float64())
		placeAsk = false
	}

	if s.stopEMA != nil {
		emaPrice := fixedpoint.NewFromFloat(s.stopEMA.Last(0))
		if midPrice.Compare(emaPrice) > 0 {
			s.logger.Infof("mid price %f > stop ema price %f, turning off bid orders", midPrice.Float64(), emaPrice.Float64())
			placeBid = false
		}

		if midPrice.Compare(emaPrice) < 0 {
			s.logger.Infof("mid price %f < stop ema price %f, turning off ask orders", midPrice.Float64(), emaPrice.Float64())
			placeAsk = false
		}
	}

	if s.MidPriceEMA != nil && s.MidPriceEMA.Enabled && s.MidPriceEMA.MaxBiasRatio.Sign() > 0 {
		s.logger.Infof("mid price ema protection is enabled, checking bias ratio...")

		emaMidPrice := s.midPriceEMA.Last(0)
		if emaMidPrice > 0 {
			biasRatio := (midPrice.Float64() - emaMidPrice) / emaMidPrice

			midPriceEMAMetrics.With(s.metricsLabels).Set(emaMidPrice)
			midPriceBiasRatioMetrics.With(s.metricsLabels).Set(biasRatio)

			s.logger.Infof("mid price bias ratio: %f, mid price: %f, ema mid price: %f",
				biasRatio, midPrice.Float64(), emaMidPrice)

			// only handle positive case
			if biasRatio > s.MidPriceEMA.MaxBiasRatio.Float64() {
				// fix midPrice
				s.logger.Infof("fixing mid price %f to ema mid price %f", midPrice.Float64(), emaMidPrice)
				midPrice = fixedpoint.NewFromFloat(emaMidPrice)
			}
		}
	}

	ask1Price := midPrice.Mul(fixedpoint.One.Add(sideSpread))
	bid1Price := midPrice.Mul(fixedpoint.One.Sub(sideSpread))
	askLastPrice := midPrice.Mul(fixedpoint.One.Add(s.AskLiquidityPriceRange))
	bidLastPrice := midPrice.Mul(fixedpoint.One.Sub(s.BidLiquidityPriceRange))
	s.logger.Infof("wanted side spread: %f askRange: %f ~ %f bidRange: %f ~ %f",
		sideSpread.Float64(),
		ask1Price.Float64(), askLastPrice.Float64(),
		bid1Price.Float64(), bidLastPrice.Float64())

	availableBase := baseBal.Available
	availableQuote := quoteBal.Available

	s.logger.Infof("balances before liq orders: %s, %s",
		baseBal.String(),
		quoteBal.String())

	if !s.Position.IsDust() {
		positionBase := s.Position.GetBase()
		if s.Position.IsLong() {
			availableBase = availableBase.Sub(positionBase)
			availableBase = s.Market.RoundDownQuantityByPrecision(availableBase)

			if s.UseProtectedPriceRange {
				ask1Price = profitProtectedPrice(types.SideTypeSell, s.Position.AverageCost, ask1Price, s.Session.MakerFeeRate, s.MinProfit)
			}
		} else if s.Position.IsShort() {
			posSizeInQuote := positionBase.Mul(ticker.Sell)
			availableQuote = availableQuote.Sub(posSizeInQuote)

			if s.UseProtectedPriceRange {
				bid1Price = profitProtectedPrice(types.SideTypeBuy, s.Position.AverageCost, bid1Price, s.Session.MakerFeeRate, s.MinProfit)
			}
		}

		if s.MaxPositionExposure.Sign() > 0 {
			if positionBase.Abs().Compare(s.MaxPositionExposure) > 0 {
				if s.Position.IsLong() {
					s.logger.Infof("long position size %f exceeded max position exposure %f, turnning off bid orders",
						positionBase.Float64(), s.MaxPositionExposure.Float64())

					placeBid = false
				}
				if s.Position.IsShort() {
					s.logger.Infof("short position size %f exceeded max position exposure %f, turnning off ask orders",
						positionBase.Float64(), s.MaxPositionExposure.Float64())

					placeAsk = false
				}
			}
		}
	}

	s.logger.Infof("place bid: %v, place ask: %v", placeBid, placeAsk)

	s.logger.Infof("bid liquidity amount %f, ask liquidity amount %f", s.BidLiquidityAmount.Float64(), s.AskLiquidityAmount.Float64())

	var bidExposureInUsd = fixedpoint.Zero
	var askExposureInUsd = fixedpoint.Zero
	var orderForms []types.SubmitOrder

	orderPlacementStatusMetrics.With(extendLabels(s.metricsLabels, prometheus.Labels{
		"side": "bid",
	})).Set(bool2float(placeBid))

	orderPlacementStatusMetrics.With(extendLabels(s.metricsLabels, prometheus.Labels{
		"side": "ask",
	})).Set(bool2float(placeAsk))

	if placeAsk {
		askOrders := s.orderGenerator.Generate(types.SideTypeSell,
			s.AskLiquidityAmount,
			ask1Price,
			askLastPrice,
			s.NumOfLiquidityLayers,
			s.liquidityScale)

		askOrders = filterAskOrders(askOrders, baseBal.Available)

		if len(askOrders) > 0 {
			askLiquidityPriceLowMetrics.With(s.metricsLabels).Set(askOrders[0].Price.Float64())
			askLiquidityPriceHighMetrics.With(s.metricsLabels).Set(askOrders[len(askOrders)-1].Price.Float64())
		}

		askExposureInUsd = sumOrderQuoteQuantity(askOrders)
		orderForms = append(orderForms, askOrders...)
	}

	bidLiquidityAmountMetrics.With(s.metricsLabels).Set(s.BidLiquidityAmount.Float64())
	askLiquidityAmountMetrics.With(s.metricsLabels).Set(s.AskLiquidityAmount.Float64())
	liquidityPriceRangeMetrics.With(s.metricsLabels).Set(s.LiquidityPriceRange.Float64())

	if placeBid {
		bidOrders := s.orderGenerator.Generate(types.SideTypeBuy,
			fixedpoint.Min(s.BidLiquidityAmount, quoteBal.Available),
			bid1Price,
			bidLastPrice,
			s.NumOfLiquidityLayers,
			s.liquidityScale)

		bidExposureInUsd = sumOrderQuoteQuantity(bidOrders)

		if len(bidOrders) > 0 {
			bidLiquidityPriceHighMetrics.With(s.metricsLabels).Set(bidOrders[0].Price.Float64())
			bidLiquidityPriceLowMetrics.With(s.metricsLabels).Set(bidOrders[len(bidOrders)-1].Price.Float64())
		}

		orderForms = append(orderForms, bidOrders...)
	}

	dbg.DebugSubmitOrders(s.logger, orderForms)

	createdOrders, err := s.OrderExecutor.SubmitOrders(ctx, orderForms...)
	if util.LogErr(err, "unable to place liquidity orders") {
		openOrderBidExposureInUsdMetrics.With(s.metricsLabels).Set(0.0)
		openOrderAskExposureInUsdMetrics.With(s.metricsLabels).Set(0.0)
		return
	}

	s.liquidityOrderBook.Add(createdOrders...)

	openOrderBidExposureInUsdMetrics.With(s.metricsLabels).Set(bidExposureInUsd.Float64())
	openOrderAskExposureInUsdMetrics.With(s.metricsLabels).Set(askExposureInUsd.Float64())

	s.logger.Infof("%d liq orders are placed successfully", len(orderForms))
	for _, o := range createdOrders {
		s.logger.Infof("liq order: %+v", o)
	}
}

func profitProtectedPrice(
	side types.SideType, averageCost, price, feeRate, minProfit fixedpoint.Value,
) fixedpoint.Value {
	switch side {
	case types.SideTypeSell:
		minProfitPrice := averageCost.Add(
			averageCost.Mul(feeRate.Add(minProfit)))
		return fixedpoint.Max(minProfitPrice, price)

	case types.SideTypeBuy:
		minProfitPrice := averageCost.Sub(
			averageCost.Mul(feeRate.Add(minProfit)))
		return fixedpoint.Min(minProfitPrice, price)

	}
	return price
}

func sumOrderQuoteQuantity(orders []types.SubmitOrder) fixedpoint.Value {
	sum := fixedpoint.Zero
	for _, order := range orders {
		sum = sum.Add(order.Price.Mul(order.Quantity))
	}
	return sum
}

func filterAskOrders(askOrders []types.SubmitOrder, available fixedpoint.Value) (out []types.SubmitOrder) {
	usedBase := fixedpoint.Zero
	for _, askOrder := range askOrders {
		if usedBase.Add(askOrder.Quantity).Compare(available) > 0 {
			return out
		}

		usedBase = usedBase.Add(askOrder.Quantity)
		out = append(out, askOrder)
	}

	return out
}

func extendLabels(a, o prometheus.Labels) prometheus.Labels {
	for k, v := range a {
		if _, exists := o[k]; !exists {
			o[k] = v
		}
	}

	return o
}

func bool2float(b bool) float64 {
	if b {
		return 1.0
	} else {
		return -1.0
	}
}
