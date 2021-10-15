package techsignal

import (
	"context"
	"errors"
	"fmt"
	"github.com/c9s/bbgo/pkg/exchange/binance"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/leekchan/accounting"
	"github.com/sirupsen/logrus"
	"strings"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "techsignal"

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
	Symbol string       `json:"symbol"`
	Market types.Market `json:"-"`

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
		MovingAverageInterval types.Interval `json:"movingAverageInterval"`

		// MovingAverageWindow is the number of the window size of the moving average indicator.
		// The number of k-lines in the window. generally used window sizes are 7, 25 and 99 in the TradingView.
		MovingAverageWindow int `json:"movingAverageWindow"`

		MinVolume fixedpoint.Value `json:"minVolume"`

		MinQuoteVolume fixedpoint.Value `json:"minQuoteVolume"`
	} `json:"supportDetection"`
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	// session.Subscribe(types.BookChannel, s.Symbol, types.SubscribeOptions{})
	for _, detection := range s.SupportDetection {
		session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{
			Interval: string(detection.Interval),
		})
	}
}

func (s *Strategy) Validate() error {
	if len(s.Symbol) == 0 {
		return errors.New("symbol is required")
	}

	return nil
}

func signedPercentage(val fixedpoint.Value) string {
	if val > 0 {
		return "+" + val.Percentage()
	}
	return val.Percentage()
}

func (s *Strategy) listenToFundingRate(ctx context.Context, exchange *binance.Exchange) {
	var previousFundingRate, fundingRate24HoursLow *binance.FundingRate

	fundingRateTicker := time.NewTicker(1 * time.Hour)
	defer fundingRateTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-fundingRateTicker.C:
			fundingRate, err := exchange.QueryLastFundingRate(ctx, s.Symbol)
			if err != nil {
				log.WithError(err).Error("can not query last funding rate")
				continue
			}

			if fundingRate.FundingRate >= s.FundingRate.High {
				s.Notifiability.Notify("%s funding rate is too high! current %s > threshold %s",
					s.Symbol,
					fundingRate.FundingRate.Percentage(),
					s.FundingRate.High.Percentage(),
				)
			}

			if previousFundingRate != nil {
				if s.FundingRate.DiffThreshold == 0 {
					s.FundingRate.DiffThreshold = fixedpoint.NewFromFloat(0.005 * 0.01)
				}

				diff := fundingRate.FundingRate - previousFundingRate.FundingRate
				if diff > s.FundingRate.DiffThreshold {
					s.Notifiability.Notify("%s funding rate changed %s, current funding rate %s",
						s.Symbol,
						signedPercentage(diff),
						fundingRate.FundingRate.Percentage(),
					)
				}
			}

			previousFundingRate = fundingRate
			if fundingRate24HoursLow != nil {
				if fundingRate24HoursLow.Time.Before(time.Now().Add(24 * time.Hour)) {
					fundingRate24HoursLow = fundingRate
				}
				if fundingRate.FundingRate < fundingRate24HoursLow.FundingRate {
					fundingRate24HoursLow = fundingRate
				}
			} else {
				fundingRate24HoursLow = fundingRate
			}
		}
	}
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	standardIndicatorSet, ok := session.StandardIndicatorSet(s.Symbol)
	if !ok {
		return fmt.Errorf("standardIndicatorSet is nil, symbol %s", s.Symbol)
	}

	if s.FundingRate != nil {
		if binanceExchange, ok := session.Exchange.(*binance.Exchange); ok {
			go s.listenToFundingRate(ctx, binanceExchange)
		} else {
			log.Error("exchange does not support funding rate api")
		}
	}

	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		// skip k-lines from other symbols
		if kline.Symbol != s.Symbol {
			return
		}

		for _, detection := range s.SupportDetection {
			if kline.Interval != detection.Interval {
				continue
			}

			closePriceF := kline.GetClose()
			closePrice := fixedpoint.NewFromFloat(closePriceF)

			var ma types.Float64Indicator

			switch strings.ToLower(detection.MovingAverageType) {
			case "sma":
				ma = standardIndicatorSet.SMA(types.IntervalWindow{
					Interval: detection.MovingAverageInterval,
					Window:   detection.MovingAverageWindow,
				})
			case "ema", "ewma":
				ma = standardIndicatorSet.EWMA(types.IntervalWindow{
					Interval: detection.MovingAverageInterval,
					Window:   detection.MovingAverageWindow,
				})
			default:
				ma = standardIndicatorSet.EWMA(types.IntervalWindow{
					Interval: detection.MovingAverageInterval,
					Window:   detection.MovingAverageWindow,
				})
			}

			var lastMA = ma.Last()

			// skip if the closed price is above the moving average
			if closePrice.Float64() > lastMA {
				return
			}

			prettyBaseVolume := accounting.DefaultAccounting(s.Market.BaseCurrency, s.Market.VolumePrecision)
			prettyBaseVolume.Format = "%v %s"

			prettyQuoteVolume := accounting.DefaultAccounting(s.Market.QuoteCurrency, 0)
			prettyQuoteVolume.Format = "%v %s"

			if detection.MinVolume > 0 && kline.Volume > detection.MinVolume.Float64() {
				s.Notifiability.Notify("Detected %s %s support base volume %s > min base volume %s, quote volume %s",
					s.Symbol, detection.Interval,
					prettyBaseVolume.FormatMoney(kline.Volume),
					prettyBaseVolume.FormatMoney(detection.MinVolume.Float64()),
					prettyQuoteVolume.FormatMoney(kline.QuoteVolume),
				)
				s.Notifiability.Notify(kline)
			} else if detection.MinQuoteVolume > 0 && kline.QuoteVolume > detection.MinQuoteVolume.Float64() {
				s.Notifiability.Notify("Detected %s %s support quote volume %s > min quote volume %s, base volume %s",
					s.Symbol, detection.Interval,
					prettyQuoteVolume.FormatMoney(kline.QuoteVolume),
					prettyQuoteVolume.FormatMoney(detection.MinQuoteVolume.Float64()),
					prettyBaseVolume.FormatMoney(kline.Volume),
				)
				s.Notifiability.Notify(kline)
			}
		}
	})
	return nil
}
