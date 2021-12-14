package ftx

import (
	"context"
	batch2 "github.com/c9s/bbgo/pkg/exchange/batch"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
	"time"
)

func TestLastKline(t *testing.T) {
	key := os.Getenv("FTX_API_KEY")
	secret := os.Getenv("FTX_API_SECRET")
	subAccount := os.Getenv("FTX_SUBACCOUNT")
	if len(key) == 0 && len(secret) == 0 {
		t.Skip("api key/secret are not configured")
	}

	e := NewExchange(key, secret, subAccount)
	//stream := NewStream(key, secret, subAccount, e)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	klines := getLastClosedKLine(e, ctx, "XRPUSD", types.Interval1m)
	assert.Equal(t, 1, len(klines))

}

func Test_Batch(t *testing.T) {
	key := os.Getenv("FTX_API_KEY")
	secret := os.Getenv("FTX_API_SECRET")
	subAccount := os.Getenv("FTX_SUBACCOUNT")
	if len(key) == 0 && len(secret) == 0 {
		t.Skip("api key/secret are not configured")
	}

	e := NewExchange(key, secret, subAccount)
	//stream := NewStream(key, secret, subAccount, e)

	batch := &batch2.KLineBatchQuery{Exchange: e}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// should use channel here

	starttime, err := time.Parse("2006-1-2 15:04", "2021-08-01 00:00")
	assert.NoError(t, err)
	endtime, err := time.Parse("2006-1-2 15:04", "2021-08-04 00:19")
	assert.NoError(t, err)

	klineC, errC := batch.Query(ctx, "XRPUSDT", types.Interval1d, starttime, endtime)

	if err := <-errC; err != nil {
		assert.NoError(t, err)
	}

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

		if !lastmintime.IsZero() {
			assert.True(t, nowMinTime.Unix() <= nowMaxTime.Unix())
			assert.True(t, nowMinTime.Unix() > lastmaxtime.Unix())
			assert.True(t, nowMaxTime.Unix() > lastmaxtime.Unix())
		}
		lastmintime = nowMinTime
		lastmaxtime = nowMaxTime
		assert.True(t, lastmintime.Unix() <= lastmaxtime.Unix())

	}

}
