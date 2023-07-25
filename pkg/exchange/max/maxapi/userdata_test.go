package max

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/valyala/fastjson"
)

func Test_parseTradeSnapshotEvent(t *testing.T) {
	fv, err := fastjson.Parse(`{
		"c": "user",
		"e": "trade_snapshot",
		"t": [{
			"i": 68444,
			"p": "21499.0",
			"v": "0.2658",
			"M": "ethtwd",
			"T": 1521726960357,
			"sd": "bid",
			"f": "3.2",
			"fc": "twd",
            "fd": false,
			"m": true,
			"oi": 7423,
			"ci": "client-oid-1",
			"gi": 123
		}],
		"T": 1591786735192
	}`)
	assert.NoError(t, err)
	assert.NotNil(t, fv)

	evt, err := parseTradeSnapshotEvent(fv)
	assert.NoError(t, err)
	assert.NotNil(t, evt)
	assert.Equal(t, "trade_snapshot", evt.Event)
	assert.Equal(t, int64(1591786735192), evt.Timestamp)
	assert.Equal(t, 1, len(evt.Trades))
	assert.Equal(t, "bid", evt.Trades[0].Side)
	assert.Equal(t, "ethtwd", evt.Trades[0].Market)
	assert.Equal(t, int64(1521726960357), evt.Trades[0].Timestamp.Time().UnixMilli())
	assert.Equal(t, "3.2", evt.Trades[0].Fee.String())
	assert.Equal(t, "twd", evt.Trades[0].FeeCurrency)
}
