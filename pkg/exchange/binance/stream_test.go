package binance

import (
	"context"
	batch2 "github.com/c9s/bbgo/pkg/exchange/batch"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
	"time"
)

func Test_Batch(t *testing.T) {
	key := os.Getenv("BINANCE_API_KEY")
	secret := os.Getenv("BINANCE_API_SECRET")
	if len(key) == 0 && len(secret) == 0 {
		t.Skip("api key/secret are not configured")
	}

	e := New(key, secret)
	//stream := NewStream(key, secret, subAccount, e)

	batch := &batch2.KLineBatchQuery{Exchange: e}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// should use channel here

	starttime, _ := time.Parse("2006-1-2 15:04", "2021-08-01 00:00")
	endtime, _ := time.Parse("2006-1-2 15:04", "2021-12-14 00:19")
	klineC, _ := batch.Query(ctx, "XRPUSDT", types.Interval1m, starttime, endtime)

	var lastmintime time.Time
	var lastmaxtime time.Time
	for klines := range klineC {
		assert.NotEmpty(t, klines)

		var nowMinTime = klines[0].StartTime
		var nowMaxTime = klines[0].StartTime
		for _, item := range klines {
			if nowMaxTime.Unix() < item.StartTime.Unix() {
				nowMaxTime = item.StartTime
			}
			if nowMinTime.Unix() > item.StartTime.Unix() {
				nowMinTime = item.StartTime
			}
		}
		assert.True(t, nowMinTime.Unix() <= nowMaxTime.Unix())
		assert.True(t, nowMinTime.Unix() > lastmaxtime.Unix())
		assert.True(t, nowMaxTime.Unix() > lastmaxtime.Unix())

		lastmintime = nowMinTime
		lastmaxtime = nowMaxTime
		assert.True(t, lastmintime.Unix() <= lastmaxtime.Unix())

	}

}
