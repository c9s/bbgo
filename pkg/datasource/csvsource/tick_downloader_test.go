package csvsource

import (
	"fmt"
	"os"
	"strings"
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
		symbol = strings.ToUpper("fxsusdt")
		path   = "testdata/binance/" + symbol
		start  = time.Now().Round(0).Add(-24 * time.Hour)
	)

	err := Download(path, symbol, types.ExchangeBinance, start)
	assert.NoError(t, err)
	klines, err := ReadTicksFromCSV(path, types.Interval1h)
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
		symbol = strings.ToUpper("fxsusdt")
		path   = "testdata/bybit/" + symbol
		start  = time.Now().Round(0).Add(-24 * time.Hour)
	)

	err := Download(path, symbol, types.ExchangeBybit, start)
	assert.NoError(t, err)
	klines, err := ReadTicksFromCSVWithDecoder(path, types.Interval1h, MakeCSVTickReader(NewBybitCSVTickReader))
	assert.NoError(t, err)
	assert.Equal(t, 24, len(klines))
	err = WriteKLines(fmt.Sprintf("%s/%s_%s.csv", path, symbol, types.Interval1h), klines)
	assert.NoError(t, err)
	err = os.RemoveAll("testdata/bybit/")
	assert.NoError(t, err)
}
