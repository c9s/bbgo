package liquiditymaker

import (
	"context"
	"fmt"
	"math"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	. "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/strategy/common"
	"github.com/c9s/bbgo/pkg/types"
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

	AdjustmentUpdateInterval types.Interval `json:"adjustmentUpdateInterval"`

	NumOfLiquidityLayers   int              `json:"numOfLiquidityLayers"`
	LiquiditySlideRule     *bbgo.SlideRule  `json:"liquidityScale"`
	LiquidityLayerTickSize fixedpoint.Value `json:"liquidityLayerTickSize"`
	LiquiditySkew          fixedpoint.Value `json:"liquiditySkew"`
	LiquidityPriceRange    fixedpoint.Value `json:"liquidityPriceRange"`

	Spread   fixedpoint.Value `json:"spread"`
	MaxPrice fixedpoint.Value `json:"maxPrice"`
	MinPrice fixedpoint.Value `json:"minPrice"`

	MaxExposure fixedpoint.Value `json:"maxExposure"`

	MinProfit fixedpoint.Value `json:"minProfit"`

	liquidityOrderBook, adjustmentOrderBook *bbgo.ActiveOrderBook
	book                                    *types.StreamOrderBook

	liquidityScale bbgo.Scale
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s:%s", ID, s.Symbol)
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.BookChannel, s.Symbol, types.SubscribeOptions{})
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.AdjustmentUpdateInterval})
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.LiquidityUpdateInterval})
}

func (s *Strategy) Run(ctx context.Context, _ bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	s.Strategy = &common.Strategy{}
	s.Strategy.Initialize(ctx, s.Environment, session, s.Market, ID, s.InstanceID())

	s.book = types.NewStreamBook(s.Symbol)
	s.book.BindStream(session.MarketDataStream)

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
			logErr(err, "unable to cancel liquidity orders")
		}

		if err := s.adjustmentOrderBook.GracefulCancel(ctx, s.Session.Exchange); err != nil {
			logErr(err, "unable to cancel adjustment orders")
		}
	})

	return nil
}

func (s *Strategy) placeAdjustmentOrders(ctx context.Context) {
	_ = s.adjustmentOrderBook.GracefulCancel(ctx, s.Session.Exchange)

	if s.Position.IsDust() {
		return
	}

	ticker, err := s.Session.Exchange.QueryTicker(ctx, s.Symbol)
	if logErr(err, "unable to query ticker") {
		return
	}

	if _, err := s.Session.UpdateAccount(ctx); err != nil {
		logErr(err, "unable to update account")
		return
	}

	baseBal, _ := s.Session.Account.Balance(s.Market.BaseCurrency)
	quoteBal, _ := s.Session.Account.Balance(s.Market.QuoteCurrency)

	var adjOrders []types.SubmitOrder

	posSize := s.Position.Base.Abs()
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
	if logErr(err, "unable to place liquidity orders") {
		return
	}

	s.adjustmentOrderBook.Add(createdOrders...)
}

func (s *Strategy) placeLiquidityOrders(ctx context.Context) {
	ticker, err := s.Session.Exchange.QueryTicker(ctx, s.Symbol)
	if logErr(err, "unable to query ticker") {
		return
	}

	if s.IsHalted(ticker.Time) {
		log.Warn("circuitBreakRiskControl: trading halted")
		return
	}

	err = s.liquidityOrderBook.GracefulCancel(ctx, s.Session.Exchange)
	if logErr(err, "unable to cancel orders") {
		return
	}

	if ticker.Buy.IsZero() && ticker.Sell.IsZero() {
		ticker.Sell = ticker.Last.Add(s.Market.TickSize)
		ticker.Buy = ticker.Last.Sub(s.Market.TickSize)
	} else if ticker.Buy.IsZero() {
		ticker.Buy = ticker.Sell.Sub(s.Market.TickSize)
	} else if ticker.Sell.IsZero() {
		ticker.Sell = ticker.Buy.Add(s.Market.TickSize)
	}

	if _, err := s.Session.UpdateAccount(ctx); err != nil {
		logErr(err, "unable to update account")
		return
	}

	baseBal, _ := s.Session.Account.Balance(s.Market.BaseCurrency)
	quoteBal, _ := s.Session.Account.Balance(s.Market.QuoteCurrency)

	lastTradedPrice := ticker.Last
	midPrice := ticker.Sell.Add(ticker.Buy).Div(fixedpoint.Two)
	currentSpread := ticker.Sell.Sub(ticker.Buy)
	tickSize := fixedpoint.Max(s.LiquidityLayerTickSize, s.Market.TickSize)
	sideSpread := s.Spread.Div(fixedpoint.Two)

	log.Infof("current: spread: %f lastTradedPrice: %f midPrice: %f", currentSpread.Float64(), lastTradedPrice.Float64(), midPrice.Float64())

	ask1Price := midPrice.Mul(fixedpoint.One.Add(sideSpread))
	bid1Price := midPrice.Mul(fixedpoint.One.Sub(sideSpread))

	askLastPrice := midPrice.Mul(fixedpoint.One.Add(s.LiquidityPriceRange))
	bidLastPrice := midPrice.Mul(fixedpoint.One.Sub(s.LiquidityPriceRange))
	log.Infof("wanted side spread: %f askRange: %f ~ %f bidRange: %f ~ %f", sideSpread.Float64(),
		ask1Price.Float64(), askLastPrice.Float64(),
		bid1Price.Float64(), bidLastPrice.Float64())

	askLayerSpread := askLastPrice.Sub(ask1Price).Div(fixedpoint.NewFromInt(int64(s.NumOfLiquidityLayers)))
	bidLayerSpread := bid1Price.Sub(bidLastPrice).Div(fixedpoint.NewFromInt(int64(s.NumOfLiquidityLayers)))

	if askLayerSpread.Compare(tickSize) < 0 {
		askLayerSpread = tickSize
	}

	if bidLayerSpread.Compare(tickSize) < 0 {
		bidLayerSpread = tickSize
	}

	sum := s.liquidityScale.Sum(1.0)
	askSum := sum
	bidSum := sum
	log.Infof("liquidity sum: %f / %f", askSum, bidSum)

	skew := s.LiquiditySkew.Float64()
	useSkew := !s.LiquiditySkew.IsZero()
	if useSkew {
		askSum = sum / skew
		bidSum = sum * skew
		log.Infof("adjusted liqudity skew: %f / %f", askSum, bidSum)
	}

	var bidPrices []fixedpoint.Value
	var askPrices []fixedpoint.Value

	// calculate and collect prices
	for i := 0; i <= s.NumOfLiquidityLayers; i++ {
		fi := fixedpoint.NewFromInt(int64(i))
		bidPrice := bid1Price.Sub(bidLayerSpread.Mul(fi))
		askPrice := ask1Price.Add(askLayerSpread.Mul(fi))

		bidPrice = s.Market.TruncatePrice(bidPrice)
		askPrice = s.Market.TruncatePrice(askPrice)

		bidPrices = append(bidPrices, bidPrice)
		askPrices = append(askPrices, askPrice)
	}

	availableBase := baseBal.Available
	availableQuote := quoteBal.Available

	makerQuota := &bbgo.QuotaTransaction{}
	makerQuota.QuoteAsset.Add(availableQuote)
	makerQuota.BaseAsset.Add(availableBase)

	log.Infof("balances before liq orders: %s, %s",
		baseBal.String(),
		quoteBal.String())

	if !s.Position.IsDust() {
		if s.Position.IsLong() {
			availableBase = availableBase.Sub(s.Position.Base)
			availableBase = s.Market.RoundDownQuantityByPrecision(availableBase)
		} else if s.Position.IsShort() {
			posSizeInQuote := s.Position.Base.Mul(ticker.Sell)
			availableQuote = availableQuote.Sub(posSizeInQuote)
		}
	}

	askX := availableBase.Float64() / askSum
	bidX := availableQuote.Float64() / (bidSum * (fixedpoint.Sum(bidPrices).Float64()))

	askX = math.Trunc(askX*1e8) / 1e8
	bidX = math.Trunc(bidX*1e8) / 1e8

	var liqOrders []types.SubmitOrder
	for i := 0; i <= s.NumOfLiquidityLayers; i++ {
		bidQuantity := fixedpoint.NewFromFloat(s.liquidityScale.Call(float64(i)) * bidX)
		askQuantity := fixedpoint.NewFromFloat(s.liquidityScale.Call(float64(i)) * askX)
		bidPrice := bidPrices[i]
		askPrice := askPrices[i]

		log.Infof("liqudity layer #%d %f/%f = %f/%f", i, askPrice.Float64(), bidPrice.Float64(), askQuantity.Float64(), bidQuantity.Float64())

		placeBuy := true
		placeSell := true
		averageCost := s.Position.AverageCost
		// when long position, do not place sell orders below the average cost
		if !s.Position.IsDust() {
			if s.Position.IsLong() && askPrice.Compare(averageCost) < 0 {
				placeSell = false
			}

			if s.Position.IsShort() && bidPrice.Compare(averageCost) > 0 {
				placeBuy = false
			}
		}

		quoteQuantity := bidQuantity.Mul(bidPrice)

		if s.Market.IsDustQuantity(bidQuantity, bidPrice) || !makerQuota.QuoteAsset.Lock(quoteQuantity) {
			placeBuy = false
		}

		if s.Market.IsDustQuantity(askQuantity, askPrice) || !makerQuota.BaseAsset.Lock(askQuantity) {
			placeSell = false
		}

		if placeBuy {
			liqOrders = append(liqOrders, types.SubmitOrder{
				Symbol:      s.Symbol,
				Side:        types.SideTypeBuy,
				Type:        types.OrderTypeLimitMaker,
				Quantity:    bidQuantity,
				Price:       bidPrice,
				Market:      s.Market,
				TimeInForce: types.TimeInForceGTC,
			})
		}

		if placeSell {
			liqOrders = append(liqOrders, types.SubmitOrder{
				Symbol:      s.Symbol,
				Side:        types.SideTypeSell,
				Type:        types.OrderTypeLimitMaker,
				Quantity:    askQuantity,
				Price:       askPrice,
				Market:      s.Market,
				TimeInForce: types.TimeInForceGTC,
			})
		}
	}

	makerQuota.Commit()

	createdOrders, err := s.OrderExecutor.SubmitOrders(ctx, liqOrders...)
	if logErr(err, "unable to place liquidity orders") {
		return
	}

	s.liquidityOrderBook.Add(createdOrders...)
	log.Infof("%d liq orders are placed successfully", len(liqOrders))
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

func logErr(err error, msgAndArgs ...interface{}) bool {
	if err == nil {
		return false
	}

	if len(msgAndArgs) == 0 {
		log.WithError(err).Error(err.Error())
	} else if len(msgAndArgs) == 1 {
		msg := msgAndArgs[0].(string)
		log.WithError(err).Error(msg)
	} else if len(msgAndArgs) > 1 {
		msg := msgAndArgs[0].(string)
		log.WithError(err).Errorf(msg, msgAndArgs[1:]...)
	}

	return true
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
