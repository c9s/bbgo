package csvsource

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestReadKlinesFromCSV(t *testing.T) {
	prices, err := ReadKlinesFromCSV("./testdata/BTCUSDT-1h-2021-Q1.csv", time.Hour)
	assert.NoError(t, err)
	assert.Len(t, prices, 2158)
}
