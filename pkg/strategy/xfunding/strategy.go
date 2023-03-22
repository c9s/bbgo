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
	Symbol              string           `json:"symbol"`
	Market              types.Market     `json:"-"`
	Quantity            fixedpoint.Value `json:"quantity,omitempty"`
	MaxExposurePosition fixedpoint.Value `json:"maxExposurePosition"`
	// Interval            types.Interval   `json:"interval"`

	FundingRate *struct {
		High    fixedpoint.Value `json:"high"`
		Neutral fixedpoint.Value `json:"neutral"`
	} `json:"fundingRate"`

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

	spotOrderExecutor, futuresOrderExecutor bbgo.OrderExecutor
	spotMarket, futuresMarket               types.Market

	SpotSession    string `json:"spotSession"`
	FuturesSession string `json:"futuresSession"`

	// positionAction is default to NoOp
	positionAction PositionAction
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
	// session.Subscribe(types.BookChannel, s.Symbol, types.SubscribeOptions{})

	// session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{
	//	Interval: string(s.Interval),
	// })

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

	return nil
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s-%s", ID, s.Symbol)
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	standardIndicatorSet := session.StandardIndicatorSet(s.Symbol)

	if !session.Futures {
		log.Error("futures not enabled in config for this strategy")
		return nil
	}

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

	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, types.Interval1m, func(kline types.KLine) {
		premiumIndex, err := session.Exchange.(*binance.Exchange).QueryPremiumIndex(ctx, s.Symbol)
		if err != nil {
			log.Error("exchange does not support funding rate api")
		}

		// skip k-lines from other symbols
		for _, detection := range s.SupportDetection {
			var lastMA = ma.Last()

			closePrice := kline.GetClose()
			closePriceF := closePrice.Float64()
			// skip if the closed price is under the moving average
			if closePriceF < lastMA {
				log.Infof("skip %s closed price %v < last ma %f", s.Symbol, closePrice, lastMA)
				return
			}

			fundingRate := premiumIndex.LastFundingRate

			if fundingRate.Compare(s.FundingRate.High) >= 0 {
				bbgo.Notify("%s funding rate %s is too high! threshold %s",
					s.Symbol,
					fundingRate.Percentage(),
					s.FundingRate.High.Percentage(),
				)
			} else {
				log.Infof("skip funding rate is too low")
				return
			}

			prettyBaseVolume := s.Market.BaseCurrencyFormatter()
			prettyQuoteVolume := s.Market.QuoteCurrencyFormatter()

			if detection.MinVolume.Sign() > 0 && kline.Volume.Compare(detection.MinVolume) > 0 {
				bbgo.Notify("Detected %s %s resistance base volume %s > min base volume %s, quote volume %s",
					s.Symbol, detection.Interval.String(),
					prettyBaseVolume.FormatMoney(kline.Volume.Trunc()),
					prettyBaseVolume.FormatMoney(detection.MinVolume.Trunc()),
					prettyQuoteVolume.FormatMoney(kline.QuoteVolume.Trunc()),
				)
				bbgo.Notify(kline)

				baseBalance, ok := session.GetAccount().Balance(s.Market.BaseCurrency)
				if !ok {
					return
				}

				if baseBalance.Available.Sign() > 0 && baseBalance.Total().Compare(s.MaxExposurePosition) < 0 {
					log.Infof("opening a short position")
					_, err := orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
						Symbol:   kline.Symbol,
						Side:     types.SideTypeSell,
						Type:     types.OrderTypeMarket,
						Quantity: s.Quantity,
					})
					if err != nil {
						log.WithError(err).Error("submit order error")
					}
				}
			} else if detection.MinQuoteVolume.Sign() > 0 && kline.QuoteVolume.Compare(detection.MinQuoteVolume) > 0 {
				bbgo.Notify("Detected %s %s resistance quote volume %s > min quote volume %s, base volume %s",
					s.Symbol, detection.Interval.String(),
					prettyQuoteVolume.FormatMoney(kline.QuoteVolume.Trunc()),
					prettyQuoteVolume.FormatMoney(detection.MinQuoteVolume.Trunc()),
					prettyBaseVolume.FormatMoney(kline.Volume.Trunc()),
				)
				bbgo.Notify(kline)
			}
		}
	}))
	return nil
}

func (s *Strategy) CrossRun(ctx context.Context, orderExecutionRouter bbgo.OrderExecutionRouter, sessions map[string]*bbgo.ExchangeSession) error {
	instanceID := s.InstanceID()

	// TODO: add safety check
	s.spotSession = sessions[s.SpotSession]
	s.futuresSession = sessions[s.FuturesSession]

	s.spotMarket, _ = s.spotSession.Market(s.Symbol)
	s.futuresMarket, _ = s.futuresSession.Market(s.Symbol)

	if s.ProfitStats == nil {
		s.ProfitStats = types.NewProfitStats(s.Market)
	}

	if s.FuturesPosition == nil {
		s.FuturesPosition = types.NewPositionFromMarket(s.futuresMarket)
	}

	if s.SpotPosition == nil {
		s.SpotPosition = types.NewPositionFromMarket(s.spotMarket)
	}

	s.spotOrderExecutor = s.allocateOrderExecutor(ctx, s.spotSession, instanceID, s.SpotPosition)
	s.futuresOrderExecutor = s.allocateOrderExecutor(ctx, s.futuresSession, instanceID, s.FuturesPosition)
	return nil
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
