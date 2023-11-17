package csvsource

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

func Test_Download_Binance_Default(t *testing.T) {
	if _, ok := os.LookupEnv("TEST_CSV_DOWNLOADER"); !ok {
		t.Skip()
	}
	var (
		symbol = "FXSUSDT"
		path   = "testdata/binance/" + symbol
		since  = time.Now().Round(0)
		until  = since.Add(-24 * time.Hour)
	)

	err := Download(path, symbol, types.ExchangeBybit, since, until)
	assert.NoError(t, err)
	klines, err := ReadTicksFromCSV(path, symbol, types.Interval1h)
	assert.NoError(t, err)
	assert.Equal(t, 24, len(klines))
	err = WriteKLines(fmt.Sprintf("%s/%s_%s.csv", path, symbol, types.Interval1h), klines)
	assert.NoError(t, err)
	err = os.RemoveAll("testdata/binance/")
	assert.NoError(t, err)
}

func Test_Download_Bybit(t *testing.T) {
	if _, ok := os.LookupEnv("TEST_CSV_DOWNLOADER"); !ok {
		t.Skip()
	}
	var (
		symbol = "FXSUSDT"
		path   = "testdata/bybit/" + symbol
		since  = time.Now().Round(0)
		until  = since.Add(-24 * time.Hour)
	)

	err := Download(path, symbol, types.ExchangeBybit, since, until)
	assert.NoError(t, err)
	klines, err := ReadTicksFromCSVWithDecoder(path, symbol, types.Interval1h, MakeCSVTickReader(NewBybitCSVTickReader))
	assert.NoError(t, err)
	assert.Equal(t, 24, len(klines))
	err = WriteKLines(fmt.Sprintf("%s/%s_%s.csv", path, symbol, types.Interval1h), klines)
	assert.NoError(t, err)
	err = os.RemoveAll("testdata/bybit/")
	assert.NoError(t, err)
}

func Test_Download_OkEx(t *testing.T) {
	if _, ok := os.LookupEnv("TEST_CSV_DOWNLOADER"); !ok {
		t.Skip()
	}
	var (
		symbol = "FXSUSDT"
		path   = "testdata/okex/" + symbol
		since  = time.Now().Round(0)
		until  = since.Add(-24 * time.Hour)
	)

	err := Download(path, symbol, types.ExchangeOKEx, since, until)
	assert.NoError(t, err)
	klines, err := ReadTicksFromCSVWithDecoder(path, symbol, types.Interval1h, MakeCSVTickReader(NewOKExCSVTickReader))
	assert.NoError(t, err)
	assert.Equal(t, 24, len(klines))
	err = WriteKLines(fmt.Sprintf("%s/%s_%s.csv", path, symbol, types.Interval1h), klines)
	assert.NoError(t, err)
	err = os.RemoveAll("testdata/okex/")
	assert.NoError(t, err)
}
