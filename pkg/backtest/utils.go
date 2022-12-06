package backtest

import (
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
)

func CollectSubscriptionIntervals(environ *bbgo.Environment) (allKLineIntervals map[types.Interval]struct{}, requiredInterval types.Interval, backTestIntervals []types.Interval) {
	// default extra back-test intervals
	backTestIntervals = []types.Interval{types.Interval1h, types.Interval1d}
	// all subscribed intervals
	allKLineIntervals = make(map[types.Interval]struct{})

	for _, interval := range backTestIntervals {
		allKLineIntervals[interval] = struct{}{}
	}
	// default interval is 1m for all exchanges
	requiredInterval = types.Interval1m
	for _, session := range environ.Sessions() {
		for _, sub := range session.Subscriptions {
			if sub.Channel == types.KLineChannel {
				if sub.Options.Interval.Seconds()%60 > 0 {
					// if any subscription interval is less than 60s, then we will use 1s for back-testing
					requiredInterval = types.Interval1s
					logrus.Warnf("found kline subscription interval less than 60s, modify default backtest interval to 1s")
				}
				allKLineIntervals[sub.Options.Interval] = struct{}{}
			}
		}
	}
	return allKLineIntervals, requiredInterval, backTestIntervals
}

func InitializeExchangeSources(sessions map[string]*bbgo.ExchangeSession, startTime, endTime time.Time, requiredInterval types.Interval, extraIntervals ...types.Interval) (exchangeSources []*ExchangeDataSource, err error) {
	for _, session := range sessions {
		backtestEx := session.Exchange.(*Exchange)

		c, err := backtestEx.SubscribeMarketData(startTime, endTime, requiredInterval, extraIntervals...)
		if err != nil {
			return exchangeSources, err
		}

		sessionCopy := session
		src := &ExchangeDataSource{
			C:        c,
			Exchange: backtestEx,
			Session:  sessionCopy,
		}
		backtestEx.Src = src
		exchangeSources = append(exchangeSources, src)
	}
	return exchangeSources, nil
}
