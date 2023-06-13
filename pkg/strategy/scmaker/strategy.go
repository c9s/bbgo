package scmaker

import (
	"context"
	"fmt"
	"math"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "scmaker"

var ten = fixedpoint.NewFromInt(10)

type BollingerConfig struct {
	Interval types.Interval `json:"interval"`
	Window   int            `json:"window"`
	K        float64        `json:"k"`
}

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

// scmaker is a stable coin market maker
type Strategy struct {
	Environment *bbgo.Environment
	Market      types.Market

	Symbol string `json:"symbol"`

	NumOfLiquidityLayers int `json:"numOfLiquidityLayers"`

	LiquidityUpdateInterval types.Interval   `json:"liquidityUpdateInterval"`
	PriceRangeBollinger     *BollingerConfig `json:"priceRangeBollinger"`
	StrengthInterval        types.Interval   `json:"strengthInterval"`

	AdjustmentUpdateInterval types.Interval `json:"adjustmentUpdateInterval"`

	MidPriceEMA        *types.IntervalWindow `json:"midPriceEMA"`
	LiquiditySlideRule *bbgo.SlideRule       `json:"liquidityScale"`

	Position    *types.Position    `json:"position,omitempty" persistence:"position"`
	ProfitStats *types.ProfitStats `json:"profitStats,omitempty" persistence:"profit_stats"`

	session                                 *bbgo.ExchangeSession
	orderExecutor                           *bbgo.GeneralOrderExecutor
	liquidityOrderBook, adjustmentOrderBook *bbgo.ActiveOrderBook
	book                                    *types.StreamOrderBook

	liquidityScale bbgo.Scale

	// indicators
	ewma      *indicator.EWMAStream
	boll      *indicator.BOLLStream
	intensity *IntensityStream
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

	if s.MidPriceEMA != nil {
		session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.MidPriceEMA.Interval})
	}
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	instanceID := s.InstanceID()

	s.session = session
	s.book = types.NewStreamBook(s.Symbol)
	s.book.BindStream(session.UserDataStream)

	s.liquidityOrderBook = bbgo.NewActiveOrderBook(s.Symbol)
	s.adjustmentOrderBook = bbgo.NewActiveOrderBook(s.Symbol)

	// If position is nil, we need to allocate a new position for calculation
	if s.Position == nil {
		s.Position = types.NewPositionFromMarket(s.Market)
	}

	// Always update the position fields
	s.Position.Strategy = ID
	s.Position.StrategyInstanceID = instanceID

	if s.session.MakerFeeRate.Sign() > 0 || s.session.TakerFeeRate.Sign() > 0 {
		s.Position.SetExchangeFeeRate(s.session.ExchangeName, types.ExchangeFee{
			MakerFeeRate: s.session.MakerFeeRate,
			TakerFeeRate: s.session.TakerFeeRate,
		})
	}

	if s.ProfitStats == nil {
		s.ProfitStats = types.NewProfitStats(s.Market)
	}

	scale, err := s.LiquiditySlideRule.Scale()
	if err != nil {
		return err
	}

	if err := scale.Solve(); err != nil {
		return err
	}

	s.liquidityScale = scale

	s.orderExecutor = bbgo.NewGeneralOrderExecutor(session, s.Symbol, ID, instanceID, s.Position)
	s.orderExecutor.BindEnvironment(s.Environment)
	s.orderExecutor.BindProfitStats(s.ProfitStats)
	s.orderExecutor.Bind()
	s.orderExecutor.TradeCollector().OnPositionUpdate(func(position *types.Position) {
		bbgo.Sync(ctx, s)
	})

	s.initializeMidPriceEMA(session)
	s.initializePriceRangeBollinger(session)
	s.initializeIntensityIndicator(session)

	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, s.AdjustmentUpdateInterval, func(k types.KLine) {
		s.placeAdjustmentOrders(ctx)
	}))

	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, s.LiquidityUpdateInterval, func(k types.KLine) {
		s.placeLiquidityOrders(ctx)
	}))

	return nil
}

func (s *Strategy) initializeMidPriceEMA(session *bbgo.ExchangeSession) {
	kLines := indicator.KLines(session.MarketDataStream, s.Symbol, s.MidPriceEMA.Interval)
	s.ewma = indicator.EWMA2(indicator.ClosePrices(kLines), s.MidPriceEMA.Window)
}

func (s *Strategy) initializeIntensityIndicator(session *bbgo.ExchangeSession) {
	kLines := indicator.KLines(session.MarketDataStream, s.Symbol, s.StrengthInterval)
	s.intensity = Intensity(kLines, 10)
}

func (s *Strategy) initializePriceRangeBollinger(session *bbgo.ExchangeSession) {
	kLines := indicator.KLines(session.MarketDataStream, s.Symbol, s.PriceRangeBollinger.Interval)
	closePrices := indicator.ClosePrices(kLines)
	s.boll = indicator.BOLL2(closePrices, s.PriceRangeBollinger.Window, s.PriceRangeBollinger.K)
}

func (s *Strategy) placeAdjustmentOrders(ctx context.Context) {
	if s.Position.IsDust() {
		return
	}
}

func (s *Strategy) placeLiquidityOrders(ctx context.Context) {
	_ = s.liquidityOrderBook.GracefulCancel(ctx, s.session.Exchange)

	ticker, err := s.session.Exchange.QueryTicker(ctx, s.Symbol)
	if logErr(err, "unable to query ticker") {
		return
	}

	baseBal, _ := s.session.Account.Balance(s.Market.BaseCurrency)
	quoteBal, _ := s.session.Account.Balance(s.Market.QuoteCurrency)

	spread := ticker.Sell.Sub(ticker.Buy)
	_ = spread

	midPriceEMA := s.ewma.Last(0)
	midPrice := fixedpoint.NewFromFloat(midPriceEMA)

	makerQuota := &bbgo.QuotaTransaction{}
	makerQuota.QuoteAsset.Add(quoteBal.Available)
	makerQuota.BaseAsset.Add(baseBal.Available)

	bandWidth := s.boll.Last(0)
	_ = bandWidth

	log.Infof("mid price ema: %f boll band width: %f", midPriceEMA, bandWidth)

	var liqOrders []types.SubmitOrder
	for i := 0; i <= s.NumOfLiquidityLayers; i++ {
		fi := fixedpoint.NewFromInt(int64(i))
		quantity := fixedpoint.NewFromFloat(s.liquidityScale.Call(float64(i)))
		bidPrice := midPrice.Sub(s.Market.TickSize.Mul(fi))
		askPrice := midPrice.Add(s.Market.TickSize.Mul(fi))
		if i == 0 {
			bidPrice = ticker.Buy
			askPrice = ticker.Sell
		}

		log.Infof("layer #%d %f/%f = %f", i, askPrice.Float64(), bidPrice.Float64(), quantity.Float64())

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

		quoteQuantity := quantity.Mul(bidPrice)

		if !makerQuota.QuoteAsset.Lock(quoteQuantity) {
			placeBuy = false
		}

		if !makerQuota.BaseAsset.Lock(quantity) {
			placeSell = false
		}

		if placeBuy {
			liqOrders = append(liqOrders, types.SubmitOrder{
				Symbol:      s.Symbol,
				Side:        types.SideTypeBuy,
				Type:        types.OrderTypeLimitMaker,
				Quantity:    quantity,
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
				Quantity:    quantity,
				Price:       askPrice,
				Market:      s.Market,
				TimeInForce: types.TimeInForceGTC,
			})
		}
	}

	_, err = s.orderExecutor.SubmitOrders(ctx, liqOrders...)
	logErr(err, "unable to place liquidity orders")
}

func (s *Strategy) generateOrders(symbol string, side types.SideType, price, priceTick, baseQuantity fixedpoint.Value, numOrders int) (orders []types.SubmitOrder) {
	var expBase = fixedpoint.Zero

	switch side {
	case types.SideTypeBuy:
		if priceTick.Sign() > 0 {
			priceTick = priceTick.Neg()
		}

	case types.SideTypeSell:
		if priceTick.Sign() < 0 {
			priceTick = priceTick.Neg()
		}
	}

	decdigits := priceTick.Abs().NumIntDigits()
	step := priceTick.Abs().MulExp(-decdigits + 1)

	for i := 0; i < numOrders; i++ {
		quantityExp := fixedpoint.NewFromFloat(math.Exp(expBase.Float64()))
		volume := baseQuantity.Mul(quantityExp)
		amount := volume.Mul(price)
		// skip order less than 10usd
		if amount.Compare(ten) < 0 {
			log.Warnf("amount too small (< 10usd). price=%s volume=%s amount=%s",
				price.String(), volume.String(), amount.String())
			continue
		}

		orders = append(orders, types.SubmitOrder{
			Symbol:   symbol,
			Side:     side,
			Type:     types.OrderTypeLimit,
			Price:    price,
			Quantity: volume,
		})

		log.Infof("%s order: %s @ %s", side, volume.String(), price.String())

		if len(orders) >= numOrders {
			break
		}

		price = price.Add(priceTick)
		expBase = expBase.Add(step)
	}

	return orders
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
