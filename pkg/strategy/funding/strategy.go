package funding

import (
	"context"
	"errors"
	"fmt"
	"github.com/c9s/bbgo/pkg/exchange/binance"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/sirupsen/logrus"
	"math"
	"strings"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "funding"

var log = logrus.WithField("strategy", ID)

func init() {
	// Register the pointer of the strategy struct,
	// so that bbgo knows what struct to be used to unmarshal the configs (YAML or JSON)
	// Note: built-in strategies need to imported manually in the bbgo cmd package.
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	*bbgo.Notifiability
	// These fields will be filled from the config file (it translates YAML to JSON)
	Symbol              string           `json:"symbol"`
	Market              types.Market     `json:"-"`
	Quantity            fixedpoint.Value `json:"quantity,omitempty"`
	MaxExposurePosition fixedpoint.Value `json:"maxExposurePosition"`
	//Interval            types.Interval   `json:"interval"`

	FundingRate *struct {
		High          fixedpoint.Value `json:"high"`
		Neutral       fixedpoint.Value `json:"neutral"`
		DiffThreshold fixedpoint.Value `json:"diffThreshold"`
	} `json:"fundingRate"`

	SupportDetection []struct {
		Interval types.Interval `json:"interval"`
		// MovingAverageType is the moving average indicator type that we want to use,
		// it could be SMA or EWMA
		MovingAverageType string `json:"movingAverageType"`

		// MovingAverageInterval is the interval of k-lines for the moving average indicator to calculate,
		// it could be "1m", "5m", "1h" and so on.  note that, the moving averages are calculated from
		// the k-line data we subscribed
		//MovingAverageInterval types.Interval `json:"movingAverageInterval"`
		//
		//// MovingAverageWindow is the number of the window size of the moving average indicator.
		//// The number of k-lines in the window. generally used window sizes are 7, 25 and 99 in the TradingView.
		//MovingAverageWindow int `json:"movingAverageWindow"`

		MovingAverageIntervalWindow types.IntervalWindow `json:"movingAverageIntervalWindow"`

		MinVolume fixedpoint.Value `json:"minVolume"`

		MinQuoteVolume fixedpoint.Value `json:"minQuoteVolume"`
	} `json:"supportDetection"`
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	// session.Subscribe(types.BookChannel, s.Symbol, types.SubscribeOptions{})

	//session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{
	//	Interval: string(s.Interval),
	//})

	for _, detection := range s.SupportDetection {
		session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{
			Interval: string(detection.Interval),
		})
		session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{
			Interval: string(detection.MovingAverageIntervalWindow.Interval),
		})
	}
}

func (s *Strategy) Validate() error {
	if len(s.Symbol) == 0 {
		return errors.New("symbol is required")
	}

	return nil
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {

	standardIndicatorSet, ok := session.StandardIndicatorSet(s.Symbol)
	if !ok {
		return fmt.Errorf("standardIndicatorSet is nil, symbol %s", s.Symbol)
	}
	//binanceExchange, ok := session.Exchange.(*binance.Exchange)
	//if !ok {
	//	log.Error("exchange failed")
	//}
	if !session.Futures {
		log.Error("futures not enabled in config for this strategy")
		return nil
	}

	//if s.FundingRate != nil {
	//	go s.listenToFundingRate(ctx, binanceExchange)
	//}
	premiumIndex, err := session.Exchange.(*binance.Exchange).QueryPremiumIndex(ctx, s.Symbol)
	if err != nil {
		log.Error("exchange does not support funding rate api")
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

	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		// skip k-lines from other symbols
		if kline.Symbol != s.Symbol {
			return
		}
		for _, detection := range s.SupportDetection {
			var lastMA = ma.Last()

			closePriceF := kline.GetClose()
			closePrice := fixedpoint.NewFromFloat(closePriceF)
			// skip if the closed price is under the moving average
			if closePrice.Float64() < lastMA {
				log.Infof("skip %s closed price %f < last ma %f", s.Symbol, closePrice.Float64(), lastMA)
				return
			}

			fundingRate := premiumIndex.LastFundingRate

			if fundingRate >= s.FundingRate.High {
				s.Notifiability.Notify("%s funding rate %s is too high! threshold %s",
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

			if detection.MinVolume > 0 && kline.Volume > detection.MinVolume.Float64() {
				s.Notifiability.Notify("Detected %s %s resistance base volume %s > min base volume %s, quote volume %s",
					s.Symbol, detection.Interval.String(),
					prettyBaseVolume.FormatMoney(math.Round(kline.Volume)),
					prettyBaseVolume.FormatMoney(math.Round(detection.MinVolume.Float64())),
					prettyQuoteVolume.FormatMoney(math.Round(kline.QuoteVolume)),
				)
				s.Notifiability.Notify(kline)

				baseBalance, ok := session.Account.Balance(s.Market.BaseCurrency)
				if !ok {
					return
				}

				if baseBalance.Available > 0 && baseBalance.Total() < s.MaxExposurePosition {
					log.Infof("opening a short position")
					_, err := orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
						Symbol:   kline.Symbol,
						Side:     types.SideTypeSell,
						Type:     types.OrderTypeMarket,
						Quantity: s.Quantity.Float64(),
					})
					if err != nil {
						log.WithError(err).Error("submit order error")
					}
				}
			} else if detection.MinQuoteVolume > 0 && kline.QuoteVolume > detection.MinQuoteVolume.Float64() {
				s.Notifiability.Notify("Detected %s %s resistance quote volume %s > min quote volume %s, base volume %s",
					s.Symbol, detection.Interval.String(),
					prettyQuoteVolume.FormatMoney(math.Round(kline.QuoteVolume)),
					prettyQuoteVolume.FormatMoney(math.Round(detection.MinQuoteVolume.Float64())),
					prettyBaseVolume.FormatMoney(math.Round(kline.Volume)),
				)
				s.Notifiability.Notify(kline)
			}
		}
	})
	return nil
}
