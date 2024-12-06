package sentinel

import (
	"context"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/narumiruna/go-iforest/pkg/iforest"
	log "github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

const ID = "sentinel"

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	Symbol         string         `json:"symbol"`
	Interval       types.Interval `json:"interval"`
	ScoreThreshold float64        `json:"scoreThreshold"`
	KLineLimit     int            `json:"klineLimit"`
	Window         int            `json:"window"`

	IsolationForest      *iforest.IsolationForest `json:"isolationForest"`
	NotificationInterval time.Duration            `json:"notificationInterval"`

	notificationRateLimiter *rate.Limiter
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Defaults() error {
	if s.ScoreThreshold == 0 {
		s.ScoreThreshold = 0.6
	}

	if s.KLineLimit == 0 {
		s.KLineLimit = 1440
	}

	if s.Window == 0 {
		s.Window = 60
	}

	if s.NotificationInterval == 0 {
		s.NotificationInterval = 10 * time.Minute
	}

	s.notificationRateLimiter = rate.NewLimiter(rate.Every(s.NotificationInterval), 1)
	return nil
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	session.MarketDataStream.OnKLine(types.KLineWith(s.Symbol, s.Interval, func(kline types.KLine) {
		if !s.isMarketAvailable(session, s.Symbol) {
			return
		}

		klines, err := s.queryKLines(ctx, session)
		if err != nil {
			log.Errorf("Unable to query klines: %v", err)
			return
		}

		volumes := s.extractVolumes(klines)
		samples := s.generateSamples(volumes)

		if s.shouldSkipIsolationForest(volumes, samples) {
			s.logSkipIsolationForest(samples, volumes, kline)
			return
		}

		s.fitIsolationForest(samples)
		scores := s.IsolationForest.Score(samples)
		s.handleIsolationForestScore(scores, kline)
	}))
	return nil
}

func (s *Strategy) isMarketAvailable(session *bbgo.ExchangeSession, symbol string) bool {
	_, ok := session.Market(symbol)
	return ok
}

func (s *Strategy) queryKLines(ctx context.Context, session *bbgo.ExchangeSession) ([]types.KLine, error) {
	endTime := time.Now()
	options := types.KLineQueryOptions{
		Limit:   s.KLineLimit,
		EndTime: &endTime,
	}
	return session.Exchange.QueryKLines(ctx, s.Symbol, s.Interval, options)
}

func (s *Strategy) extractVolumes(klines []types.KLine) floats.Slice {
	volumes := floats.Slice{}
	for _, kline := range klines {
		volumes.Push(kline.Volume.Float64())
	}
	return volumes
}

func (s *Strategy) generateSamples(volumes floats.Slice) [][]float64 {
	samples := make([][]float64, 0, len(volumes))
	for i := range volumes {
		if i < s.Window {
			continue
		}

		subset := volumes.Tail(s.Window)

		mean := subset.Mean()
		std := subset.Std()
		samples = append(samples, []float64{mean, std})
	}
	return samples
}

func (s *Strategy) shouldSkipIsolationForest(volumes floats.Slice, samples [][]float64) bool {
	volumeMean := volumes.Mean()
	lastMovingMean := samples[len(samples)-1][0]
	return lastMovingMean < volumeMean
}

func (s *Strategy) logSkipIsolationForest(samples [][]float64, volumes floats.Slice, kline types.KLine) {
	log.Infof("Skipping isolation forest calculation for symbol: %s, last moving mean: %f, average volume: %f, kline: %s", s.Symbol, samples[len(samples)-1][0], volumes.Mean(), kline.String())
}

func (s *Strategy) fitIsolationForest(samples [][]float64) {
	s.IsolationForest = iforest.New()
	s.IsolationForest.Fit(samples)
	log.Infof("Isolation forest fitted with %d samples and %d/%d trees", len(samples), len(s.IsolationForest.Trees), s.IsolationForest.NumTrees)
}

func (s *Strategy) handleIsolationForestScore(scores []float64, kline types.KLine) {
	lastScore := scores[len(scores)-1]
	log.Warnf("Symbol: %s, isolation forest score: %f, threshold: %f, kline: %s", s.Symbol, lastScore, s.ScoreThreshold, kline.String())
	if lastScore > s.ScoreThreshold && s.notificationRateLimiter.Allow() {
		bbgo.Notify("symbol: %s isolation forest score: %f", s.Symbol, lastScore)
	}
}
