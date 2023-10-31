package csvsource

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_Download_Binance_Default(t *testing.T) {
	var (
		symbol = strings.ToUpper("fxsusdt")
		path   = "testdata/binance/" + symbol
		start  = time.Now().Round(0).Add(-24 * time.Hour)
	)

	err := Download(path, symbol, Binance, start)
	assert.NoError(t, err)
	klines, err := ReadTicksFromCSV(path, H1)
	assert.NoError(t, err)
	assert.Equal(t, 24, len(klines))
	err = WriteKLines(fmt.Sprintf("%s/%s_%s.csv", path, symbol, H1), klines)
	assert.NoError(t, err)
	err = os.RemoveAll("testdata/binance/")
	assert.NoError(t, err)
}

func Test_Download_Bybit(t *testing.T) {
	var (
		symbol = strings.ToUpper("fxsusdt")
		path   = "testdata/bybit/" + symbol
		start  = time.Now().Round(0).Add(-24 * time.Hour)
	)

	err := Download(path, symbol, Bybit, start)
	assert.NoError(t, err)
	klines, err := ReadTicksFromCSVWithDecoder(path, H1, MakeCSVTickReader(NewBybitCSVTickReader))
	assert.NoError(t, err)
	assert.Equal(t, 24, len(klines))
	err = WriteKLines(fmt.Sprintf("%s/%s_%s.csv", path, symbol, H1), klines)
	assert.NoError(t, err)
	err = os.RemoveAll("testdata/bybit/")
	assert.NoError(t, err)
}
