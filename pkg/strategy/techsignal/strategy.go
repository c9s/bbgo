package techsignal

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/exchange/binance"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"

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
			Interval: detection.Interval,
		})

		session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{
			Interval: detection.MovingAverageInterval,
		})
	}
}

func (s *Strategy) Validate() error {
	if len(s.Symbol) == 0 {
		return errors.New("symbol is required")
	}

	return nil
}

func (s *Strategy) listenToFundingRate(ctx context.Context, exchange *binance.Exchange) {
	var previousIndex, fundingRate24HoursLowIndex *types.PremiumIndex

	fundingRateTicker := time.NewTicker(1 * time.Hour)
	defer fundingRateTicker.Stop()
	for {
		select {

		case <-ctx.Done():
			return

		case <-fundingRateTicker.C:
			index, err := exchange.QueryPremiumIndex(ctx, s.Symbol)
			if err != nil {
				log.WithError(err).Error("can not query last funding rate")
				continue
			}

			fundingRate := index.LastFundingRate

			if fundingRate.Compare(s.FundingRate.High) >= 0 {
				bbgo.Notify("%s funding rate %s is too high! threshold %s",
					s.Symbol,
					fundingRate.Percentage(),
					s.FundingRate.High.Percentage(),
				)
			} else {
				if previousIndex != nil {
					if s.FundingRate.DiffThreshold.IsZero() {
						// 0.6%
						s.FundingRate.DiffThreshold = fixedpoint.NewFromFloat(0.006 * 0.01)
					}

					diff := fundingRate.Sub(previousIndex.LastFundingRate)
					if diff.Abs().Compare(s.FundingRate.DiffThreshold) > 0 {
						bbgo.Notify("%s funding rate changed %s, current funding rate %s",
							s.Symbol,
							diff.SignedPercentage(),
							fundingRate.Percentage(),
						)
					}
				}
			}

			previousIndex = index
			if fundingRate24HoursLowIndex != nil {
				if fundingRate24HoursLowIndex.Time.Before(time.Now().Add(24 * time.Hour)) {
					fundingRate24HoursLowIndex = index
				}
				if fundingRate.Compare(fundingRate24HoursLowIndex.LastFundingRate) < 0 {
					fundingRate24HoursLowIndex = index
				}
			} else {
				fundingRate24HoursLowIndex = index
			}
		}
	}
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	// create the moving average indicators before registering the kline handler below,
	// so that they are always updated before the handler reads them
	indicators := session.Indicators(s.Symbol)

	type movingAverage struct {
		ma     types.Float64Indicator
		kLines *indicatorv2.KLineStream

		// minKLines is the number of k-lines required before the moving average is used
		minKLines int
	}

	movingAverages := make([]movingAverage, len(s.SupportDetection))
	for i, detection := range s.SupportDetection {
		iw := types.IntervalWindow{
			Interval: detection.MovingAverageInterval,
			Window:   detection.MovingAverageWindow,
		}

		m := movingAverage{kLines: indicators.KLines(iw.Interval)}
		switch strings.ToLower(detection.MovingAverageType) {
		case "sma":
			m.ma = indicators.SMA(iw)
			// the legacy SMA did not emit any value before the window was full, so the last moving average was zero
			// and the support was skipped. keep waiting for a full window.
			m.minKLines = iw.Window
		case "ema", "ewma":
			m.ma = indicators.EWMA(iw)
		default:
			m.ma = indicators.EWMA(iw)
		}

		movingAverages[i] = m
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

		for i, detection := range s.SupportDetection {
			if kline.Interval != detection.Interval {
				continue
			}

			closePrice := kline.GetClose()

			m := movingAverages[i]

			var lastMA = 0.0
			if m.kLines.Length() >= m.minKLines {
				lastMA = m.ma.Last(0)
			}

			// skip if the closed price is above the moving average
			if closePrice.Float64() > lastMA {
				log.Infof("skip %s support closed price %f > last ma %f", s.Symbol, closePrice.Float64(), lastMA)
				return
			}

			prettyBaseVolume := s.Market.BaseCurrencyFormatter()
			prettyQuoteVolume := s.Market.QuoteCurrencyFormatter()

			if detection.MinVolume.Sign() > 0 && kline.Volume.Compare(detection.MinVolume) > 0 {
				bbgo.Notify("Detected %s %s support base volume %s > min base volume %s, quote volume %s",
					s.Symbol, detection.Interval.String(),
					prettyBaseVolume.FormatMoney(kline.Volume.Trunc()),
					prettyBaseVolume.FormatMoney(detection.MinVolume.Trunc()),
					prettyQuoteVolume.FormatMoney(kline.QuoteVolume.Trunc()),
				)
				bbgo.Notify(kline)
			} else if detection.MinQuoteVolume.Sign() > 0 && kline.QuoteVolume.Compare(detection.MinQuoteVolume) > 0 {
				bbgo.Notify("Detected %s %s support quote volume %s > min quote volume %s, base volume %s",
					s.Symbol, detection.Interval.String(),
					prettyQuoteVolume.FormatMoney(kline.QuoteVolume.Trunc()),
					prettyQuoteVolume.FormatMoney(detection.MinQuoteVolume.Trunc()),
					prettyBaseVolume.FormatMoney(kline.Volume.Trunc()),
				)
				bbgo.Notify(kline)
			}
		}
	})
	return nil
}
