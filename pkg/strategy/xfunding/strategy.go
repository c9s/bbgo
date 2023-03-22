package xfunding

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/exchange/binance"
	"github.com/c9s/bbgo/pkg/fixedpoint"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "xfunding"

type PositionAction int

const (
	PositionNoOp PositionAction = iota
	PositionOpening
	PositionClosing
)

var log = logrus.WithField("strategy", ID)

func init() {
	// Register the pointer of the strategy struct,
	// so that bbgo knows what struct to be used to unmarshal the configs (YAML or JSON)
	// Note: built-in strategies need to imported manually in the bbgo cmd package.
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	Environment *bbgo.Environment

	// These fields will be filled from the config file (it translates YAML to JSON)
	Symbol   string           `json:"symbol"`
	Market   types.Market     `json:"-"`
	Quantity fixedpoint.Value `json:"quantity,omitempty"`

	// IncrementalQuoteQuantity is used for opening position incrementally with a small fixed quote quantity
	// for example, 100usdt per order
	IncrementalQuoteQuantity fixedpoint.Value `json:"incrementalQuoteQuantity"`

	QuoteInvestment fixedpoint.Value `json:"quoteInvestment"`

	// ShortFundingRate is the funding rate range for short positions
	// TODO: right now we don't support negative funding rate (long position) since it's rarer
	ShortFundingRate *struct {
		High fixedpoint.Value `json:"high"`
		Low  fixedpoint.Value `json:"low"`
	} `json:"shortFundingRate"`

	SupportDetection []struct {
		Interval types.Interval `json:"interval"`
		// MovingAverageType is the moving average indicator type that we want to use,
		// it could be SMA or EWMA
		MovingAverageType string `json:"movingAverageType"`

		// MovingAverageInterval is the interval of k-lines for the moving average indicator to calculate,
		// it could be "1m", "5m", "1h" and so on.  note that, the moving averages are calculated from
		// the k-line data we subscribed
		// MovingAverageInterval types.Interval `json:"movingAverageInterval"`
		//
		// // MovingAverageWindow is the number of the window size of the moving average indicator.
		// // The number of k-lines in the window. generally used window sizes are 7, 25 and 99 in the TradingView.
		// MovingAverageWindow int `json:"movingAverageWindow"`

		MovingAverageIntervalWindow types.IntervalWindow `json:"movingAverageIntervalWindow"`

		MinVolume fixedpoint.Value `json:"minVolume"`

		MinQuoteVolume fixedpoint.Value `json:"minQuoteVolume"`
	} `json:"supportDetection"`

	ProfitStats *types.ProfitStats `persistence:"profit_stats"`

	SpotPosition    *types.Position `persistence:"spot_position"`
	FuturesPosition *types.Position `persistence:"futures_position"`

	spotSession, futuresSession *bbgo.ExchangeSession

	spotOrderExecutor, futuresOrderExecutor *bbgo.GeneralOrderExecutor
	spotMarket, futuresMarket               types.Market

	SpotSession    string `json:"spotSession"`
	FuturesSession string `json:"futuresSession"`

	// positionAction is default to NoOp
	positionAction PositionAction

	// positionType is the futures position type
	// currently we only support short position for the positive funding rate
	positionType types.PositionType

	usedQuoteInvestment fixedpoint.Value
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) CrossSubscribe(sessions map[string]*bbgo.ExchangeSession) {
	// TODO: add safety check
	spotSession := sessions[s.SpotSession]
	futuresSession := sessions[s.FuturesSession]

	spotSession.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{
		Interval: types.Interval1m,
	})

	futuresSession.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{
		Interval: types.Interval1m,
	})
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	for _, detection := range s.SupportDetection {
		session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{
			Interval: detection.Interval,
		})
		session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{
			Interval: detection.MovingAverageIntervalWindow.Interval,
		})
	}
}

func (s *Strategy) Validate() error {
	if len(s.Symbol) == 0 {
		return errors.New("symbol is required")
	}

	if len(s.SpotSession) == 0 {
		return errors.New("spotSession name is required")
	}

	if len(s.FuturesSession) == 0 {
		return errors.New("futuresSession name is required")
	}

	if s.QuoteInvestment.IsZero() {
		return errors.New("quoteInvestment can not be zero")
	}

	return nil
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s-%s", ID, s.Symbol)
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	standardIndicatorSet := session.StandardIndicatorSet(s.Symbol)

	var ma types.Float64Indicator
	for _, detection := range s.SupportDetection {

		switch strings.ToLower(detection.MovingAverageType) {
		case "sma":
			ma = standardIndicatorSet.SMA(types.IntervalWindow{
				Interval: detection.MovingAverageIntervalWindow.Interval,
				Window:   detection.MovingAverageIntervalWindow.Window,
			})
		case "ema", "ewma":
			ma = standardIndicatorSet.EWMA(types.IntervalWindow{
				Interval: detection.MovingAverageIntervalWindow.Interval,
				Window:   detection.MovingAverageIntervalWindow.Window,
			})
		default:
			ma = standardIndicatorSet.EWMA(types.IntervalWindow{
				Interval: detection.MovingAverageIntervalWindow.Interval,
				Window:   detection.MovingAverageIntervalWindow.Window,
			})
		}
	}
	_ = ma

	return nil
}

func (s *Strategy) CrossRun(ctx context.Context, orderExecutionRouter bbgo.OrderExecutionRouter, sessions map[string]*bbgo.ExchangeSession) error {
	instanceID := s.InstanceID()

	s.usedQuoteInvestment = fixedpoint.Zero

	s.spotSession = sessions[s.SpotSession]
	s.futuresSession = sessions[s.FuturesSession]

	s.spotMarket, _ = s.spotSession.Market(s.Symbol)
	s.futuresMarket, _ = s.futuresSession.Market(s.Symbol)

	// adjust QuoteInvestment
	if b, ok := s.spotSession.Account.Balance(s.spotMarket.QuoteCurrency); ok {
		originalQuoteInvestment := s.QuoteInvestment
		s.QuoteInvestment = fixedpoint.Min(b.Available, s.QuoteInvestment)

		if originalQuoteInvestment.Compare(s.QuoteInvestment) != 0 {
			log.Infof("adjusted quoteInvestment from %s to %s according to the balance",
				originalQuoteInvestment.String(),
				s.QuoteInvestment.String(),
			)
		}
	}

	if s.ProfitStats == nil {
		s.ProfitStats = types.NewProfitStats(s.Market)
	}

	if s.FuturesPosition == nil {
		s.FuturesPosition = types.NewPositionFromMarket(s.futuresMarket)
	}

	if s.SpotPosition == nil {
		s.SpotPosition = types.NewPositionFromMarket(s.spotMarket)
	}

	binanceFutures := s.futuresSession.Exchange.(*binance.Exchange)
	binanceSpot := s.spotSession.Exchange.(*binance.Exchange)
	_ = binanceSpot

	s.spotOrderExecutor = s.allocateOrderExecutor(ctx, s.spotSession, instanceID, s.SpotPosition)
	s.spotOrderExecutor.TradeCollector().OnTrade(func(trade types.Trade, profit fixedpoint.Value, netProfit fixedpoint.Value) {
		// we act differently on the spot account
		// when opening a position, we place orders on the spot account first, then the futures account,
		// and we need to accumulate the used quote amount
		//
		// when closing a position, we place orders on the futures account first, then the spot account
		// we need to close the position according to its base quantity instead of quote quantity
		if s.positionType == types.PositionShort {
			switch s.positionAction {
			case PositionOpening:
				if trade.Side != types.SideTypeSell {
					log.Errorf("unexpected trade side: %+v, expecting SELL trade", trade)
					return
				}

				// TODO: add mutex lock for this modification
				s.usedQuoteInvestment = s.usedQuoteInvestment.Add(trade.QuoteQuantity)
				if s.usedQuoteInvestment.Compare(s.QuoteInvestment) >= 0 {
					s.positionAction = PositionNoOp
				}

				// 1) if we have trade, try to query the balance and transfer the balance to the futures wallet account
				// balances, err := binanceSpot.QueryAccountBalances(ctx)

				// 2) transferred successfully, sync futures position

				// 3) compare spot position and futures position, increase the position size until they are the same size

			case PositionClosing:
				if trade.Side != types.SideTypeBuy {
					log.Errorf("unexpected trade side: %+v, expecting BUY trade", trade)
					return
				}

			}
		}
	})

	s.futuresOrderExecutor = s.allocateOrderExecutor(ctx, s.futuresSession, instanceID, s.FuturesPosition)

	s.futuresSession.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, types.Interval1m, func(kline types.KLine) {
		premiumIndex, err := binanceFutures.QueryPremiumIndex(ctx, s.Symbol)
		if err != nil {
			log.WithError(err).Error("premium index query error")
			return
		}

		s.detectPremiumIndex(premiumIndex)
	}))

	s.spotSession.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, types.Interval1m, func(k types.KLine) {
		// TODO: use go routine and time.Ticker
		s.triggerPositionAction(ctx)
	}))

	return nil
}

// TODO: replace type binance.Exchange with an interface
func (s *Strategy) transferIn(ctx context.Context, ex *binance.Exchange, trade types.Trade) error {
	balances, err := ex.QueryAccountBalances(ctx)
	if err != nil {
		return err
	}

	b, ok := balances[s.spotMarket.BaseCurrency]
	if !ok {
		return nil
	}

	// TODO: according to the fee, we might not be able to get enough balance greater than the trade quantity
	if b.Available.Compare(trade.Quantity) >= 0 {

	}
	return nil
}

func (s *Strategy) syncSpotPosition(ctx context.Context) {
	ticker, err := s.spotSession.Exchange.QueryTicker(ctx, s.Symbol)
	if err != nil {
		log.WithError(err).Errorf("can not query ticker")
		return
	}

	if s.positionType != types.PositionShort {
		log.Errorf("funding long position type is not supported")
		return
	}

	switch s.positionAction {

	case PositionClosing:

	case PositionOpening:
		if s.usedQuoteInvestment.IsZero() || s.usedQuoteInvestment.Compare(s.QuoteInvestment) >= 0 {
			// stop
			return
		}

		leftQuote := s.QuoteInvestment.Sub(s.usedQuoteInvestment)
		orderPrice := ticker.Sell
		orderQuantity := fixedpoint.Min(s.IncrementalQuoteQuantity, leftQuote).Div(orderPrice)
		orderQuantity = fixedpoint.Max(orderQuantity, s.spotMarket.MinQuantity)
		createdOrders, err := s.spotOrderExecutor.SubmitOrders(ctx, types.SubmitOrder{
			Symbol:      s.Symbol,
			Side:        types.SideTypeSell,
			Type:        types.OrderTypeLimitMaker,
			Quantity:    orderQuantity,
			Price:       orderPrice,
			Market:      s.spotMarket,
			TimeInForce: types.TimeInForceGTC,
		})
		if err != nil {
			log.WithError(err).Errorf("can not submit order")
			return
		}

		log.Infof("created orders: %+v", createdOrders)
	}
}

func (s *Strategy) triggerPositionAction(ctx context.Context) {
	switch s.positionAction {
	case PositionOpening:
		s.syncSpotPosition(ctx)

	case PositionClosing:

	}
}

func (s *Strategy) detectPremiumIndex(premiumIndex *types.PremiumIndex) {
	fundingRate := premiumIndex.LastFundingRate
	if s.ShortFundingRate != nil {
		if fundingRate.Compare(s.ShortFundingRate.High) >= 0 {
			s.positionAction = PositionOpening
			s.positionType = types.PositionShort
		} else if fundingRate.Compare(s.ShortFundingRate.Low) <= 0 {
			s.positionAction = PositionClosing
		}
	}
}

func (s *Strategy) allocateOrderExecutor(ctx context.Context, session *bbgo.ExchangeSession, instanceID string, position *types.Position) *bbgo.GeneralOrderExecutor {
	orderExecutor := bbgo.NewGeneralOrderExecutor(session, s.Symbol, ID, instanceID, position)
	orderExecutor.BindEnvironment(s.Environment)
	orderExecutor.Bind()
	orderExecutor.TradeCollector().OnTrade(func(trade types.Trade, _, _ fixedpoint.Value) {
		s.ProfitStats.AddTrade(trade)
	})
	orderExecutor.TradeCollector().OnPositionUpdate(func(position *types.Position) {
		bbgo.Sync(ctx, s)
	})
	return orderExecutor
}
