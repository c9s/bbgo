package ftx

import (
	"encoding/json"
	"io/ioutil"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/exchange/ftx/ftxapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func Test_rawResponse_toSubscribedResp(t *testing.T) {
	input := `{"type": "subscribed", "channel": "orderbook", "market": "BTC/USDT"}`
	var m websocketResponse
	assert.NoError(t, json.Unmarshal([]byte(input), &m))
	r, err := m.toSubscribedResponse()
	assert.NoError(t, err)
	assert.Equal(t, subscribedResponse{
		mandatoryFields: mandatoryFields{
			Channel: orderBookChannel,
			Type:    subscribedRespType,
		},
		Market: "BTC/USDT",
	}, r)
}

func Test_websocketResponse_toPublicOrderBookResponse(t *testing.T) {
	f, err := ioutil.ReadFile("./orderbook_snapshot.json")
	assert.NoError(t, err)
	var m websocketResponse
	assert.NoError(t, json.Unmarshal(f, &m))
	r, err := m.toPublicOrderBookResponse()
	assert.NoError(t, err)
	assert.Equal(t, partialRespType, r.Type)
	assert.Equal(t, orderBookChannel, r.Channel)
	assert.Equal(t, "BTC/USDT", r.Market)
	assert.Equal(t, int64(1614520368), r.Timestamp.Unix())
	assert.Equal(t, uint32(2150525410), r.Checksum)
	assert.Len(t, r.Bids, 100)
	assert.Equal(t, []json.Number{"44555.0", "3.3968"}, r.Bids[0])
	assert.Equal(t, []json.Number{"44554.0", "0.0561"}, r.Bids[1])
	assert.Len(t, r.Asks, 100)
	assert.Equal(t, []json.Number{"44574.0", "0.4591"}, r.Asks[0])
	assert.Equal(t, []json.Number{"44579.0", "0.15"}, r.Asks[1])
}

func Test_orderBookResponse_toGlobalOrderBook(t *testing.T) {
	f, err := ioutil.ReadFile("./orderbook_snapshot.json")
	assert.NoError(t, err)
	var m websocketResponse
	assert.NoError(t, json.Unmarshal(f, &m))
	r, err := m.toPublicOrderBookResponse()
	assert.NoError(t, err)

	b, err := toGlobalOrderBook(r)
	assert.NoError(t, err)
	assert.Equal(t, "BTCUSDT", b.Symbol)
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

func Test_checksumString(t *testing.T) {
	type args struct {
		bids [][]json.Number
		asks [][]json.Number
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "more bids",
			args: args{
				bids: [][]json.Number{{"5000.5", "10"}, {"4995.0", "5"}},
				asks: [][]json.Number{{"5001.0", "6"}},
			},
			want: "5000.5:10:5001.0:6:4995.0:5",
		},
		{
			name: "lengths of bids and asks are the same",
			args: args{
				bids: [][]json.Number{{"5000.5", "10"}, {"4995.0", "5"}},
				asks: [][]json.Number{{"5001.0", "6"}, {"5002.0", "7"}},
			},
			want: "5000.5:10:5001.0:6:4995.0:5:5002.0:7",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checksumString(tt.args.bids, tt.args.asks); got != tt.want {
				t.Errorf("checksumString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_orderBookResponse_verifyChecksum(t *testing.T) {
	for _, file := range []string{"./orderbook_snapshot.json"} {
		f, err := ioutil.ReadFile(file)
		assert.NoError(t, err)
		var m websocketResponse
		assert.NoError(t, json.Unmarshal(f, &m))
		r, err := m.toPublicOrderBookResponse()
		assert.NoError(t, err)
		assert.NoError(t, r.verifyChecksum(), "filename: "+file)
	}
}

func Test_removePrice(t *testing.T) {
	pairs := [][]json.Number{{"123.99", "2.0"}, {"2234.12", "3.1"}}
	assert.Equal(t, pairs, removePrice(pairs, "99333"))

	pairs = removePrice(pairs, "2234.12")
	assert.Equal(t, [][]json.Number{{"123.99", "2.0"}}, pairs)
	assert.Equal(t, [][]json.Number{}, removePrice(pairs, "123.99"))
}

func Test_orderBookResponse_update(t *testing.T) {
	ob := &orderBookResponse{Bids: nil, Asks: nil}

	ob.update(orderBookResponse{
		Bids: [][]json.Number{{"1.0", "0"}, {"10.0", "1"}, {"11.0", "1"}},
		Asks: [][]json.Number{{"1.0", "1"}},
	})
	assert.Equal(t, [][]json.Number{{"11.0", "1"}, {"10.0", "1"}}, ob.Bids)
	assert.Equal(t, [][]json.Number{{"1.0", "1"}}, ob.Asks)
	ob.update(orderBookResponse{
		Bids: [][]json.Number{{"9.0", "1"}, {"12.0", "1"}, {"10.5", "1"}},
		Asks: [][]json.Number{{"1.0", "0"}},
	})
	assert.Equal(t, [][]json.Number{{"12.0", "1"}, {"11.0", "1"}, {"10.5", "1"}, {"10.0", "1"}, {"9.0", "1"}}, ob.Bids)
	assert.Equal(t, [][]json.Number{}, ob.Asks)

	// remove them
	ob.update(orderBookResponse{
		Bids: [][]json.Number{{"9.0", "0"}, {"12.0", "0"}, {"10.5", "0"}},
		Asks: [][]json.Number{{"9.0", "1"}, {"12.0", "1"}, {"10.5", "1"}},
	})
	assert.Equal(t, [][]json.Number{{"11.0", "1"}, {"10.0", "1"}}, ob.Bids)
	assert.Equal(t, [][]json.Number{{"9.0", "1"}, {"10.5", "1"}, {"12.0", "1"}}, ob.Asks)
}

func Test_insertAt(t *testing.T) {
	r := insertAt([][]json.Number{{"1.2", "2"}, {"1.4", "2"}}, 1, []json.Number{"1.3", "2"})
	assert.Equal(t, [][]json.Number{{"1.2", "2"}, {"1.3", "2"}, {"1.4", "2"}}, r)

	r = insertAt([][]json.Number{{"1.2", "2"}, {"1.4", "2"}}, 0, []json.Number{"1.1", "2"})
	assert.Equal(t, [][]json.Number{{"1.1", "2"}, {"1.2", "2"}, {"1.4", "2"}}, r)

	r = insertAt([][]json.Number{{"1.2", "2"}, {"1.4", "2"}}, 2, []json.Number{"1.5", "2"})
	assert.Equal(t, [][]json.Number{{"1.2", "2"}, {"1.4", "2"}, {"1.5", "2"}}, r)
}

func Test_newLoginRequest(t *testing.T) {
	// From API doc: https://docs.ftx.com/?javascript#authentication-2
	r := newLoginRequest("", "Y2QTHI23f23f23jfjas23f23To0RfUwX3H42fvN-", time.Unix(0, 1557246346499*int64(time.Millisecond)), "")
	// pragma: allowlist nextline secret
	expectedSignature := "d10b5a67a1a941ae9463a60b285ae845cdeac1b11edc7da9977bef0228b96de9"
	assert.Equal(t, expectedSignature, r.Login.Signature)
	jsonStr, err := json.Marshal(r)
	assert.NoError(t, err)
	assert.True(t, strings.Contains(string(jsonStr), expectedSignature))
}

func Test_websocketResponse_toOrderUpdateResponse(t *testing.T) {
	input := []byte(`
{
  "channel": "orders",
  "type": "update",
  "data": {
    "id": 12345,
    "clientId": "test-client-id",
    "market": "SOL/USD",
    "type": "limit",
    "side": "buy",
    "price": 0.5,
    "size": 100.0,
    "status": "closed",
    "filledSize": 0.0,
    "remainingSize": 0.0,
    "reduceOnly": false,
    "liquidation": false,
    "avgFillPrice": null,
    "postOnly": false,
    "ioc": false,
    "createdAt": "2021-03-27T11:00:36.418674+00:00"
  }
}
`)

	var raw websocketResponse
	assert.NoError(t, json.Unmarshal(input, &raw))

	r, err := raw.toOrderUpdateResponse()
	assert.NoError(t, err)

	assert.Equal(t, orderUpdateResponse{
		mandatoryFields: mandatoryFields{
			Channel: privateOrdersChannel,
			Type:    updateRespType,
		},
		Data: ftxapi.Order{
			Id:            12345,
			ClientId:      "test-client-id",
			Market:        "SOL/USD",
			Type:          "limit",
			Side:          "buy",
			Price:         fixedpoint.NewFromFloat(0.5),
			Size:          fixedpoint.NewFromInt(100),
			Status:        "closed",
			FilledSize:    fixedpoint.Zero,
			RemainingSize: fixedpoint.Zero,
			ReduceOnly:    false,
			AvgFillPrice:  fixedpoint.Zero,
			PostOnly:      false,
			Ioc:           false,
			CreatedAt:     mustParseDatetime("2021-03-27T11:00:36.418674+00:00"),
			Future:        "",
		},
	}, r)
}
