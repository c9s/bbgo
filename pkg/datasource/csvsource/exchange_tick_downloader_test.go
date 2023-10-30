package csvsource

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_Download(t *testing.T) {
	var (
		symbol = strings.ToUpper("fxsusdt")
		path   = "testdata/bybit/" + symbol
		start  = time.Now().Round(0).Add(-24 * time.Hour)
	)

	err := Download(path, symbol, Bybit, start)
	assert.NoError(t, err)
	klines, err := ConvertTicksToKLines(path, symbol, M30)
	assert.NoError(t, err)
	err = WriteKLines(fmt.Sprintf("%s/%s_%s.csv", path, symbol, M30), klines)
	assert.NoError(t, err)
	err = os.RemoveAll("testdata/bybit/")
	assert.NoError(t, err)
}
