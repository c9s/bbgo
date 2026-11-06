package binance

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/exchange/binance/binanceapi"
)

func decodeInterest(t *testing.T, data string) binanceapi.MarginInterest {
	t.Helper()
	var record binanceapi.MarginInterest
	require.NoError(t, json.Unmarshal([]byte(data), &record))
	return record
}

func TestToGlobalInterest_Converted(t *testing.T) {
	// the shape of a BNB-deducted row of /sapi/v1/margin/interestHistory
	m := toGlobalInterest(decodeInterest(t, `{"txId":1,"interestAccuredTime":1793934000000,"asset":"BNB","rawAsset":"BTC",`+
		`"principal":"0.5","interest":"0.000003","interestRate":"0.00024","type":"PERIODIC_CONVERTED"}`))
	assert.Equal(t, "BNB", m.Asset)
	assert.Equal(t, "BTC", m.RawAsset)
	assert.Equal(t, "PERIODIC_CONVERTED", m.InterestType)
	assert.True(t, m.IsConverted())
	assert.Equal(t, "BTC", m.BorrowedAsset())
	assert.Equal(t, "0.000003", m.Interest.String())
	assert.Equal(t, int64(1793934000000), m.Time.Time().UnixMilli())

	m = toGlobalInterest(decodeInterest(t, `{"asset":"BNB","rawAsset":"ETH","interest":"0.000001","type":"ON_BORROW_CONVERTED"}`))
	assert.Equal(t, "ETH", m.RawAsset)
}

func TestToGlobalInterest_NotConverted(t *testing.T) {
	// the docs example: rawAsset is returned on a row paid in the borrowed asset
	m := toGlobalInterest(decodeInterest(t, `{"asset":"USDT","rawAsset":"USDT","interest":"0.00024995","type":"ON_BORROW"}`))
	assert.Equal(t, "USDT", m.RawAsset)
	assert.False(t, m.IsConverted())
	assert.Equal(t, "USDT", m.BorrowedAsset())

	// rawAsset is not trusted on a row that is not converted, whatever it holds
	m = toGlobalInterest(decodeInterest(t, `{"asset":"USDT","rawAsset":"BTC","interest":"0.1","type":"PERIODIC"}`))
	assert.Equal(t, "USDT", m.RawAsset)

	// and filled from asset when it is not returned
	m = toGlobalInterest(decodeInterest(t, `{"asset":"USDT","interest":"0.1","type":"PERIODIC"}`))
	assert.Equal(t, "USDT", m.RawAsset)
}

func TestToGlobalInterest_ConvertedWithoutRawAsset(t *testing.T) {
	// e.g. isolated margin, where the docs say rawAsset is not returned
	m := toGlobalInterest(decodeInterest(t, `{"asset":"BNB","interest":"0.000001","type":"PERIODIC_CONVERTED","isolatedSymbol":"ETHUSDT"}`))
	assert.Equal(t, "BNB", m.RawAsset)
	assert.True(t, m.IsConverted())
}
