package liquiditymaker

import (
	"context"
	"fmt"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	. "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/strategy/common"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
	"github.com/c9s/bbgo/pkg/util/tradingutil"
)

const ID = "liquiditymaker"

type advancedOrderCancelApi interface {
	CancelAllOrders(ctx context.Context) ([]types.Order, error)
	CancelOrdersBySymbol(ctx context.Context, symbol string) ([]types.Order, error)
}

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
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
	MaxAdjustmentOrderQuantity fixedpoint.Value `json:"maxAdjustmentOrderQuantity"`

	NumOfLiquidityLayers int              `json:"numOfLiquidityLayers"`
	LiquiditySlideRule   *bbgo.SlideRule  `json:"liquidityScale"`
	LiquidityPriceRange  fixedpoint.Value `json:"liquidityPriceRange"`
	AskLiquidityAmount   fixedpoint.Value `json:"askLiquidityAmount"`
	BidLiquidityAmount   fixedpoint.Value `json:"bidLiquidityAmount"`

	UseProtectedPriceRange bool `json:"useProtectedPriceRange"`

	UseLastTradePrice bool             `json:"useLastTradePrice"`
	Spread            fixedpoint.Value `json:"spread"`
	MaxPrice          fixedpoint.Value `json:"maxPrice"`
	MinPrice          fixedpoint.Value `json:"minPrice"`

	MaxExposure fixedpoint.Value `json:"maxExposure"`

	MinProfit fixedpoint.Value `json:"minProfit"`

	common.ProfitFixerBundle

	liquidityOrderBook, adjustmentOrderBook *bbgo.ActiveOrderBook

	liquidityScale bbgo.Scale

	orderGenerator *LiquidityOrderGenerator
}

func (s *Strategy) Initialize() error {
	if s.Strategy == nil {
		s.Strategy = &common.Strategy{}
	}
	return nil
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

	s.orderGenerator = &LiquidityOrderGenerator{
		Symbol: s.Symbol,
		Market: s.Market,
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

	if cancelApi, ok := session.Exchange.(advancedOrderCancelApi); ok {
		_, _ = cancelApi.CancelOrdersBySymbol(ctx, s.Symbol)
	}

	s.liquidityScale = scale

	session.UserDataStream.OnStart(func() {
		s.placeLiquidityOrders(ctx)
	})

	session.MarketDataStream.OnKLineClosed(func(k types.KLine) {
		if k.Interval == s.AdjustmentUpdateInterval {
			s.placeAdjustmentOrders(ctx)
		}

		if k.Interval == s.LiquidityUpdateInterval {
			s.placeLiquidityOrders(ctx)
		}
	})

	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()

		if err := s.liquidityOrderBook.GracefulCancel(ctx, s.Session.Exchange); err != nil {
			util.LogErr(err, "unable to cancel liquidity orders")
		}

		if err := s.adjustmentOrderBook.GracefulCancel(ctx, s.Session.Exchange); err != nil {
			util.LogErr(err, "unable to cancel adjustment orders")
		}

		if err := tradingutil.UniversalCancelAllOrders(ctx, s.Session.Exchange, nil); err != nil {
			util.LogErr(err, "unable to cancel all orders")
		}

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

	if !s.MaxAdjustmentOrderQuantity.IsZero() {
		posSize = fixedpoint.Min(posSize, s.MaxAdjustmentOrderQuantity)
	}

	tickSize := s.Market.TickSize

	if s.Position.IsShort() {
		price := profitProtectedPrice(types.SideTypeBuy, s.Position.AverageCost, ticker.Sell.Add(tickSize.Neg()), s.Session.MakerFeeRate, s.MinProfit)
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
		price := profitProtectedPrice(types.SideTypeSell, s.Position.AverageCost, ticker.Buy.Add(tickSize), s.Session.MakerFeeRate, s.MinProfit)
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
	err := s.liquidityOrderBook.GracefulCancel(ctx, s.Session.Exchange)
	if util.LogErr(err, "unable to cancel orders") {
		return
	}

	ticker, err := s.Session.Exchange.QueryTicker(ctx, s.Symbol)
	if util.LogErr(err, "unable to query ticker") {
		return
	}

	if s.IsHalted(ticker.Time) {
		log.Warn("circuitBreakRiskControl: trading halted")
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

	log.Infof("ticker: %+v", ticker)

	lastTradedPrice := ticker.Last
	midPrice := ticker.Sell.Add(ticker.Buy).Div(fixedpoint.Two)
	currentSpread := ticker.Sell.Sub(ticker.Buy)
	sideSpread := s.Spread.Div(fixedpoint.Two)

	if s.UseLastTradePrice && !lastTradedPrice.IsZero() {
		midPrice = lastTradedPrice
	}

	log.Infof("current spread: %f lastTradedPrice: %f midPrice: %f", currentSpread.Float64(), lastTradedPrice.Float64(), midPrice.Float64())

	ask1Price := midPrice.Mul(fixedpoint.One.Add(sideSpread))
	bid1Price := midPrice.Mul(fixedpoint.One.Sub(sideSpread))

	askLastPrice := midPrice.Mul(fixedpoint.One.Add(s.LiquidityPriceRange))
	bidLastPrice := midPrice.Mul(fixedpoint.One.Sub(s.LiquidityPriceRange))
	log.Infof("wanted side spread: %f askRange: %f ~ %f bidRange: %f ~ %f",
		sideSpread.Float64(),
		ask1Price.Float64(), askLastPrice.Float64(),
		bid1Price.Float64(), bidLastPrice.Float64())

	availableBase := baseBal.Available
	availableQuote := quoteBal.Available

	log.Infof("balances before liq orders: %s, %s",
		baseBal.String(),
		quoteBal.String())

	if !s.Position.IsDust() {
		if s.Position.IsLong() {
			availableBase = availableBase.Sub(s.Position.Base)
			availableBase = s.Market.RoundDownQuantityByPrecision(availableBase)

			if s.UseProtectedPriceRange {
				ask1Price = profitProtectedPrice(types.SideTypeSell, s.Position.AverageCost, ask1Price, s.Session.MakerFeeRate, s.MinProfit)
			}
		} else if s.Position.IsShort() {
			posSizeInQuote := s.Position.Base.Mul(ticker.Sell)
			availableQuote = availableQuote.Sub(posSizeInQuote)

			if s.UseProtectedPriceRange {
				bid1Price = profitProtectedPrice(types.SideTypeBuy, s.Position.AverageCost, bid1Price, s.Session.MakerFeeRate, s.MinProfit)
			}
		}
	}

	bidOrders := s.orderGenerator.Generate(types.SideTypeBuy,
		fixedpoint.Min(s.BidLiquidityAmount, quoteBal.Available),
		bid1Price,
		bidLastPrice,
		s.NumOfLiquidityLayers,
		s.liquidityScale)

	askOrders := s.orderGenerator.Generate(types.SideTypeSell,
		s.AskLiquidityAmount,
		ask1Price,
		askLastPrice,
		s.NumOfLiquidityLayers,
		s.liquidityScale)

	askOrders = filterAskOrders(askOrders, baseBal.Available)

	orderForms := append(bidOrders, askOrders...)

	createdOrders, err := s.OrderExecutor.SubmitOrders(ctx, orderForms...)
	if util.LogErr(err, "unable to place liquidity orders") {
		return
	}

	s.liquidityOrderBook.Add(createdOrders...)
	log.Infof("%d liq orders are placed successfully", len(orderForms))
	for _, o := range createdOrders {
		log.Infof("liq order: %+v", o)
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

func preloadKLines(
	inc *KLineStream, session *bbgo.ExchangeSession, symbol string, interval types.Interval,
) {
	if store, ok := session.MarketDataStore(symbol); ok {
		if kLinesData, ok := store.KLinesOfInterval(interval); ok {
			for _, k := range *kLinesData {
				inc.EmitUpdate(k)
			}
		}
	}
}
