package binance

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/exchange/binance/binanceapi"
)

func TestToGlobalInterest(t *testing.T) {
	// the shape of a BNB-deducted row of /sapi/v1/margin/interestHistory
	var record binanceapi.MarginInterest
	require.NoError(t, json.Unmarshal([]byte(`{"txId":1793934000000000123,"interestAccuredTime":1793934000000,`+
		`"asset":"BNB","rawAsset":"BTC","principal":"0.5","interest":"0.000003","interestRate":"0.00024","type":"PERIODIC_CONVERTED"}`), &record))

	m := toGlobalInterest(record)
	require.NotNil(t, m.TxID)
	assert.Equal(t, uint64(1793934000000000123), *m.TxID, "decoded without float rounding")
	assert.Equal(t, "BNB", m.Asset)
	assert.Equal(t, "BTC", m.RawAsset)
	assert.Equal(t, "PERIODIC_CONVERTED", m.InterestType)
	assert.Equal(t, "0.5", m.Principal.String())
	assert.Equal(t, "0.000003", m.Interest.String())
	assert.Equal(t, "0.00024", m.InterestRate.String())
	assert.Equal(t, int64(1793934000000), m.Time.Time().UnixMilli())

	// stored as returned: no rawAsset, e.g. isolated margin, stays empty
	var isolated binanceapi.MarginInterest
	require.NoError(t, json.Unmarshal([]byte(`{"txId":2,"asset":"USDT","isolatedSymbol":"BTCUSDT","type":"PERIODIC"}`), &isolated))
	m = toGlobalInterest(isolated)
	assert.Equal(t, "", m.RawAsset)
	assert.Equal(t, "BTCUSDT", m.IsolatedSymbol)
}
