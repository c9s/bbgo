package premiumcorr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/strategy/common"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/sirupsen/logrus"
)

const ID = "premiumcorr"

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	*common.Strategy

	LeadingSymbol string         `json:"leadingSymbol"`
	LaggingSymbol string         `json:"laggingSymbol"`
	Interval      types.Interval `json:"interval"`
	Margin        float64        `json:"margin"`
	Delay         int            `json:"delay"`
	OutDir        string         `json:"outDir"`

	logger             *logrus.Entry
	activeRecord       *record
	records            []*record
	startTime, endTime *types.Time
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return strings.Join([]string{ID, s.LeadingSymbol, s.LaggingSymbol}, "-")
}

func (s *Strategy) Defaults() {
	if s.Interval == "" {
		s.Interval = types.Interval1m
	}
	if s.Margin == 0.0 {
		s.Margin = 0.0001
	}
}

func (s *Strategy) Initialize() error {
	s.logger = logrus.WithField("strategy", s.InstanceID())
	return nil
}

func (s *Strategy) Validate() error {
	if s.LeadingSymbol == "" {
		return errors.New("leading symbol is required")
	}
	if s.LaggingSymbol == "" {
		return errors.New("lagging symbol is required")
	}
	return nil
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.LeadingSymbol, types.SubscribeOptions{Interval: s.Interval})
	session.Subscribe(types.KLineChannel, s.LaggingSymbol, types.SubscribeOptions{Interval: s.Interval})
}

func (s *Strategy) Run(ctx context.Context, _ bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.LaggingSymbol, s.Interval, func(kline types.KLine) {
		if s.startTime == nil {
			s.startTime = &kline.EndTime
		} else {
			s.endTime = &kline.EndTime
		}
		if s.activeRecord == nil {
			s.activeRecord = &record{
				LaggingPrice: kline.Close.Float64(),
			}
		}
	}))
	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.LeadingSymbol, s.Interval, func(kline types.KLine) {
		if s.activeRecord != nil {
			s.activeRecord.LeadingPrices = append(s.activeRecord.LeadingPrices, kline.Close.Float64())
		}
	}))

	premiumStream := indicatorv2.Premium(
		session.Indicators(s.LeadingSymbol).CLOSE(s.Interval),
		session.Indicators(s.LaggingSymbol).CLOSE(s.Interval),
		s.Margin,
	)
	premiumStream.OnUpdate(func(value float64) {
		if s.activeRecord != nil {
			if len(s.activeRecord.LeadingPrices) == 1 {
				s.activeRecord.PricePremium = value
			}
			if len(s.activeRecord.LeadingPrices) >= s.Delay {
				s.records = append(s.records, s.activeRecord)
				s.activeRecord = nil
			}
		}
	})
	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()
		if len(s.records) > 0 {
			priceDiffs := types.NewFloat64Series()
			pricePremiums := types.NewFloat64Series()
			for _, record := range s.records {
				if len(record.LeadingPrices) == 0 {
					continue
				}
				maxAbsPriceDiff := 0.0
				priceDiff := 0.0
				if record.PricePremium == 1 {
					continue
				}
				for _, p := range record.LeadingPrices[1:] {
					absDiff := math.Abs(p - record.LaggingPrice)
					if (p-record.LaggingPrice)*(record.PricePremium-1) > 0 && absDiff > maxAbsPriceDiff {
						priceDiff = p - record.LaggingPrice
						maxAbsPriceDiff = absDiff
					}
				}
				s.logger.Infof("price diff: %.4f, price premium: %.4f", priceDiff, record.PricePremium)
				priceDiffs.Push(priceDiff)
				pricePremiums.Push(record.PricePremium)
			}
			corr := priceDiffs.Correlation(pricePremiums, priceDiffs.Length())
			s.logger.Infof("price premium and lagging price diff correlation: %.4f", corr)

			fileName := fmt.Sprintf(
				"%s-%s-%s-%s-i%s-d%d-records.json",
				s.LeadingSymbol,
				s.LaggingSymbol,
				s.startTime.Time().Format(time.DateOnly),
				s.endTime.Time().Format(time.DateOnly),
				s.Interval,
				s.Delay,
			)
			if s.OutDir != "" {
				if err := os.MkdirAll(s.OutDir, 0755); err != nil {
					s.logger.WithError(err).Errorf("unable to create output directory: %s", s.OutDir)
					return
				}
				fileName = fmt.Sprintf("%s/%s", s.OutDir, fileName)
			}
			file, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
			if err != nil {
				s.logger.WithError(err).Errorf("unable to open output file: %s", fileName)
				return
			}
			defer file.Close()

			jsonData, err := json.MarshalIndent(s.records, "", "  ")
			if err != nil {
				s.logger.WithError(err).Errorf("unable to marshal records to JSON")
				return
			}
			_, err = file.Write(jsonData)
			if err != nil {
				s.logger.WithError(err).Errorf("unable to write records to file %s", fileName)
				return
			}
		}
	})
	return nil
}

type record struct {
	PricePremium  float64   `json:"pricePremium"`
	LaggingPrice  float64   `json:"laggingPrice"`
	LeadingPrices []float64 `json:"leadingPrices"`
}
