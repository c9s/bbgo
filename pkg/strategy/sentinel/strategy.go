package sentinel

import (
	"context"
	"fmt"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/ensemble/iforest"
	"github.com/c9s/bbgo/pkg/exchange/batch"
	"github.com/c9s/bbgo/pkg/types"
	log "github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

const ID = "sentinel"

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	Symbol     string         `json:"symbol"`
	Interval   types.Interval `json:"interval"`
	Threshold  float64        `json:"threshold"`
	Proportion float64        `json:"proportion"`
	NumSamples int            `json:"numSamples"`
	Window     int            `json:"window"`

	IsolationForest      *iforest.IsolationForest `json:"isolationForest"`
	NotificationInterval time.Duration            `json:"notificationInterval"`
	RetrainingInterval   time.Duration            `json:"retrainingInterval"`

	notificationRateLimiter *rate.Limiter
	retrainingRateLimiter   *rate.Limiter
	klines                  []types.KLine
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Defaults() error {
	if s.Threshold == 0 {
		s.Threshold = 0.6
	}

	if s.Proportion == 0 {
		s.Proportion = 0.05
	}

	if s.NumSamples == 0 {
		s.NumSamples = 1440
	}

	if s.Window == 0 {
		s.Window = 60
	}

	if s.NotificationInterval == 0 {
		s.NotificationInterval = 10 * time.Minute
	}

	if s.RetrainingInterval == 0 {
		s.RetrainingInterval = 1 * time.Hour
	}

	s.notificationRateLimiter = rate.NewLimiter(rate.Every(s.NotificationInterval), 1)
	s.retrainingRateLimiter = rate.NewLimiter(rate.Every(s.RetrainingInterval), 1)
	return nil
}

func (s *Strategy) Validate() error {
	if s.NumSamples < 0 {
		return fmt.Errorf("num samples should be greater than 0")
	}

	if s.Window < 0 {
		return fmt.Errorf("window size should be greater than 0")
	}

	if s.Window > s.NumSamples {
		return fmt.Errorf("window size should be less than num samples")
	}
	return nil
}
func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	err := s.queryHistoricalKlines(ctx, session)
	if err != nil {
		return err
	}

	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, s.Interval, func(kline types.KLine) {
		if !s.isMarketAvailable(session, s.Symbol) {
			return
		}

		if s.isNewKline(kline) {
			s.klines = append(s.klines, kline)

			numKlines := len(s.klines)
			klineLimit := s.NumSamples + s.Window - 1
			if numKlines > klineLimit {
				s.klines = s.klines[numKlines-klineLimit:]
			}

		}

		volumes := s.getVolumeFromKlines(s.klines)

		volume := volumes.Last(0)
		mean := volumes.Mean()
		std := volumes.Std()
		// if the volume is not significantly above the mean, we don't need to calculate the isolation forest
		if volume < mean+2*std {
			log.Infof("Volume is not significantly above mean, skipping isolation forest calculation, symbol: %s, volume: %f, mean: %f, std: %f", s.Symbol, volume, mean, std)
			return
		}

		samples := s.generateSamples(volumes)
		s.trainIsolationForest(samples)

		scores := s.IsolationForest.Score(samples)
		score := scores[len(scores)-1]
		quantile := iforest.Quantile(scores, 1-s.Proportion)
		log.Infof("symbol: %s, volume: %f, mean: %f, std: %f, iforest score: %f, quantile: %f", s.Symbol, volume, mean, std, score, quantile)

		s.notifyOnScoreThresholdExceeded(score, quantile)
	}))
	return nil
}

func (s *Strategy) isMarketAvailable(session *bbgo.ExchangeSession, symbol string) bool {
	_, ok := session.Market(symbol)
	return ok
}

func (s *Strategy) queryHistoricalKlines(ctx context.Context, session *bbgo.ExchangeSession) error {
	batchQuery := batch.KLineBatchQuery{Exchange: session.Exchange}
	endTime := time.Now()
	startTime := endTime.Add(-time.Duration(s.NumSamples+s.Window) * s.Interval.Duration())
	klineC, errC := batchQuery.Query(ctx, s.Symbol, s.Interval, startTime, endTime)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errC:
			if err != nil {
				return err
			}
		case kline, ok := <-klineC:
			if !ok {
				return <-errC
			}
			s.klines = append(s.klines, kline)
		}
	}
}

func (s *Strategy) getVolumeFromKlines(klines []types.KLine) floats.Slice {
	volumes := floats.Slice{}
	for _, kline := range klines {
		volumes.Push(kline.Volume.Float64())
	}
	return volumes
}

func (s *Strategy) generateSamples(volumes floats.Slice) [][]float64 {
	samples := make([][]float64, 0, len(volumes))
	for i := range volumes {
		if i+s.Window > len(volumes) {
			break
		}
		subset := volumes[i : i+s.Window]
		last := subset.Last(0)
		mean := subset.Mean()
		std := subset.Std()

		sample := []float64{(last - mean) / std, mean, std}
		samples = append(samples, sample)
	}
	return samples
}

func (s *Strategy) trainIsolationForest(samples [][]float64) {
	if !s.retrainingRateLimiter.Allow() {
		return
	}

	s.IsolationForest = iforest.New()
	s.IsolationForest.Fit(samples)
	log.Infof("Isolation forest fitted with %d samples and %d/%d trees", len(samples), len(s.IsolationForest.Trees), s.IsolationForest.NumTrees)
}

func (s *Strategy) notifyOnScoreThresholdExceeded(score float64, quantile float64) {
	// if the score is below the threshold, we don't need to notify
	if score < s.Threshold {
		return
	}

	// if the score is below the quantile, we don't need to notify
	if score < quantile {
		return
	}

	// if the notification rate limiter is not allowed, we don't need to notify
	if !s.notificationRateLimiter.Allow() {
		return
	}

	bbgo.Notify("symbol: %s, iforest score: %f, threshold: %f, quantile: %f", s.Symbol, score, s.Threshold, quantile)
}

func (s *Strategy) isNewKline(kline types.KLine) bool {
	if len(s.klines) == 0 {
		return true
	}

	lastKline := s.klines[len(s.klines)-1]
	return lastKline.EndTime.Before(kline.EndTime.Time())
}
