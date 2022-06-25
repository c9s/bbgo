package backtest

import (
	"context"
	"errors"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type KLineFixtureGenerator struct {
	Symbol             string
	Interval           types.Interval
	StartTime, EndTime time.Time
	StartPrice         fixedpoint.Value
}

func (g *KLineFixtureGenerator) Generate(ctx context.Context, c chan types.KLine) error {
	defer close(c)

	startTime := g.StartTime
	price := g.StartPrice

	if price.IsZero() {
		return errors.New("startPrice can not be zero")
	}

	for startTime.Before(g.EndTime) {
		open := price
		high := price.Mul(fixedpoint.NewFromFloat(1.01))
		low := price.Mul(fixedpoint.NewFromFloat(0.99))
		amp := high.Sub(low)
		cls := low.Add(amp.Mul(fixedpoint.NewFromFloat(rand.Float64())))

		vol := fixedpoint.NewFromFloat(rand.Float64() * 1000.0)
		quoteVol := fixedpoint.NewFromFloat(rand.Float64() * 1000.0).Mul(price)

		nextStartTime := startTime.Add(g.Interval.Duration())
		k := types.KLine{
			Exchange:    types.ExchangeBinance,
			Symbol:      g.Symbol,
			StartTime:   types.Time(startTime),
			EndTime:     types.Time(nextStartTime.Add(-time.Millisecond)),
			Interval:    g.Interval,
			Open:        open,
			Close:       cls,
			High:        high,
			Low:         low,
			Volume:      vol,
			QuoteVolume: quoteVol,
			Closed:      true,
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case c <- k:
		}
		price = cls
		startTime = nextStartTime
	}

	return nil
}

func TestKLineFixtureGenerator(t *testing.T) {
	startTime := time.Date(2022, time.January, 1, 0, 0, 0, 0, time.Local)
	endTime := time.Date(2022, time.January, 31, 0, 0, 0, 0, time.Local)
	ctx := context.Background()
	g := &KLineFixtureGenerator{
		Symbol:     "BTCUSDT",
		Interval:   types.Interval1m,
		StartTime:  startTime,
		EndTime:    endTime,
		StartPrice: fixedpoint.NewFromFloat(18000.0),
	}

	c := make(chan types.KLine, 20)
	go func() {
		err := g.Generate(ctx, c)
		assert.NoError(t, err)
	}()
	for k := range c {
		// high must higher than low
		assert.True(t, k.High.Compare(k.Low) > 0)
		assert.True(t, k.StartTime.After(startTime) || k.StartTime.Equal(startTime))
		assert.True(t, k.StartTime.Before(endTime))
	}
}
