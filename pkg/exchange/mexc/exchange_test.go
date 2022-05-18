package mexc

import (
	"time"
	"context"
	"testing"
	"os"
	"github.com/stretchr/testify/assert"
)

func getExchange(t *testing.T) (*Exchange) {
	key := os.Getenv("MEXC_API_KEY")
	secret := os.Getenv("MEXC_API_SECRET")
	if len(key) == 0 || len(secret) == 0 {
		t.Skip("api key/secret not configured")
	}
	return &Exchange{key, secret, nil}
}

func Test_Ping(t *testing.T) {
	ex := getExchange(t)
	assert.True(t, ex.ping(context.Background()))
}

func Test_Time(t *testing.T) {
	ex := getExchange(t)
	timestamp, err := ex.time(context.Background())
	assert.Equal(t, err, nil)
	assert.InDelta(t, timestamp, time.Now().UnixMilli(), 60000)
}

func Test_Ticker(t *testing.T) {
	ex := getExchange(t)
	ticker, err := ex.QueryTicker(context.Background(), "APEUSDT")
	assert.Equal(t, err, nil)
	assert.True(t, ticker.High.Compare(ticker.Low) > 0)
	assert.InDelta(t, ticker.Time.UnixMilli(), time.Now().UnixMilli(), 86400_000)
}
