package sentinel

import (
	"context"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/narumiruna/go-isolation-forest/pkg/iforest"
	log "github.com/sirupsen/logrus"
)

const ID = "sentinel"

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	// These fields will be filled from the config file (it translates YAML to JSON)
	Symbol         string         `json:"symbol"`
	Interval       types.Interval `json:"interval"`
	ScoreThreshold float64        `json:"scoreThreshold"`

	IsolationForest *iforest.IsolationForest `json:"isolationForest"`
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	session.MarketDataStream.OnKLine(func(kline types.KLine) {
		market, ok := session.Market(kline.Symbol)
		if !ok {
			return
		}

		endTime := time.Now()
		options := types.KLineQueryOptions{
			Limit:   1441,
			EndTime: &endTime,
		}

		klines, err := session.Exchange.QueryKLines(ctx, s.Symbol, s.Interval, options)
		if err != nil {
			log.Errorf("unable to query klines: %v", err)
			return
		}

		samples := make(iforest.Matrix, 0, len(klines))
		for i, kline := range klines {
			if i == 0 {
				continue
			}

			sample := []float64{
				// price change
				kline.Close.Float64() - klines[i-1].Close.Float64(),

				// volume change
				kline.Volume.Float64() - klines[i-1].Volume.Float64(),

				// (high - low) / open
				(kline.High.Float64() - kline.Low.Float64()) / kline.Open.Float64(),

				// (close - open) / open
				(kline.Close.Float64() - kline.Open.Float64()) / kline.Open.Float64(),
			}

			samples = append(samples, sample)
		}
		s.IsolationForest = iforest.New()
		s.IsolationForest.Fit(samples)
		log.Infof("number of samples: %d", len(samples))
		log.Infof("number of trees: %d", len(s.IsolationForest.Trees))

		scores := s.IsolationForest.Score(samples)
		lastScore := scores[len(scores)-1]

		log.Infof("symbol: %s, isolation forest score: %f, threshold: %f, kline: %s", s.Symbol, lastScore, s.ScoreThreshold, kline.String())
		if lastScore > s.ScoreThreshold {
			if channel, ok := bbgo.Notification.RouteSymbol(s.Symbol); ok {
				bbgo.NotifyTo(channel, "market: %s, isolation forest score: %f", market, lastScore)
			} else {
				bbgo.Notify("market: %s isolation forest score: %f", market, lastScore)
			}
		}
	})
	return nil
}
