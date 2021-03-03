package ftx

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func Test_rawResponse_toSubscribedResp(t *testing.T) {
	input := `{"type": "subscribed", "channel": "orderbook", "market": "BTC/USDT"}`
	var m rawResponse
	assert.NoError(t, json.Unmarshal([]byte(input), &m))
	r := m.toSubscribedResp()
	assert.Equal(t, subscribedRespType, r.Type)
	assert.Equal(t, orderbook, r.Channel)
	assert.Equal(t, "BTC/USDT", r.Market)
}

func Test_rawResponse_toSnapshotResp(t *testing.T) {
	f, err := ioutil.ReadFile("./orderbook_snapshot.json")
	assert.NoError(t, err)
	var m rawResponse
	assert.NoError(t, json.Unmarshal(f, &m))
	r, err := m.toDataResponse()
	assert.NoError(t, err)
	assert.Equal(t, partialRespType, r.Type)
	assert.Equal(t, orderbook, r.Channel)
	assert.Equal(t, "BTC/USDT", r.Market)
	assert.Equal(t, int64(1614520368), r.Timestamp.Unix())
	assert.Equal(t, int64(2150525410), r.Checksum)
	assert.Len(t, r.Bids, 100)
	assert.Equal(t, []float64{44555.0, 3.3968}, r.Bids[0])
	assert.Equal(t, []float64{44554.0, 0.0561}, r.Bids[1])
	assert.Len(t, r.Asks, 100)
	assert.Equal(t, []float64{44574.0, 0.4591}, r.Asks[0])
	assert.Equal(t, []float64{44579.0, 0.15}, r.Asks[1])
}

func Test_snapshotResponse_toGlobalOrderBook(t *testing.T) {
	f, err := ioutil.ReadFile("./orderbook_snapshot.json")
	assert.NoError(t, err)
	var m rawResponse
	assert.NoError(t, json.Unmarshal(f, &m))
	r, err := m.toDataResponse()
	assert.NoError(t, err)

	b, err := r.toGlobalOrderBook()
	assert.NoError(t, err)
	assert.Equal(t, "BTC/USDT", b.Symbol)
	isValid, err := b.IsValid()
	assert.True(t, isValid)
	assert.NoError(t, err)

	assert.Len(t, b.Bids, 100)
	assert.Equal(t, types.PriceVolume{
		Price:  fixedpoint.MustNewFromString("44555.0"),
		Volume: fixedpoint.MustNewFromString("3.3968"),
	}, b.Bids[0])
	assert.Equal(t, types.PriceVolume{
		Price:  fixedpoint.MustNewFromString("44222.0"),
		Volume: fixedpoint.MustNewFromString("0.0002"),
	}, b.Bids[99])

	assert.Len(t, b.Asks, 100)
	assert.Equal(t, types.PriceVolume{
		Price:  fixedpoint.MustNewFromString("44574.0"),
		Volume: fixedpoint.MustNewFromString("0.4591"),
	}, b.Asks[0])
	assert.Equal(t, types.PriceVolume{
		Price:  fixedpoint.MustNewFromString("45010.0"),
		Volume: fixedpoint.MustNewFromString("0.0003"),
	}, b.Asks[99])
}
