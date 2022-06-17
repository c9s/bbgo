package ftx

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/exchange/ftx/ftxapi"
	"github.com/c9s/bbgo/pkg/types"
)

func Test_toGlobalOrderFromOpenOrder(t *testing.T) {
	input := `
{
	"createdAt": "2019-03-05T09:56:55.728933+00:00",
	"filledSize": 10,
	"future": "XRP-PERP",
	"id": 9596912,
	"market": "XRP-PERP",
	"price": 0.306525,
	"avgFillPrice": 0.306526,
	"remainingSize": 31421,
	"side": "sell",
	"size": 31431,
	"status": "open",
	"type": "limit",
	"reduceOnly": false,
	"ioc": false,
	"postOnly": false,
	"clientId": "client-id-123"
}
`

	var r order
	assert.NoError(t, json.Unmarshal([]byte(input), &r))

	o, err := toGlobalOrder(r)
	assert.NoError(t, err)
	assert.Equal(t, "client-id-123", o.ClientOrderID)
	assert.Equal(t, "XRP-PERP", o.Symbol)
	assert.Equal(t, types.SideTypeSell, o.Side)
	assert.Equal(t, types.OrderTypeLimit, o.Type)
	assert.Equal(t, "31431", o.Quantity.String())
	assert.Equal(t, "0.306525", o.Price.String())
	assert.Equal(t, types.TimeInForceGTC, o.TimeInForce)
	assert.Equal(t, types.ExchangeFTX, o.Exchange)
	assert.True(t, o.IsWorking)
	assert.Equal(t, uint64(9596912), o.OrderID)
	assert.Equal(t, types.OrderStatusPartiallyFilled, o.Status)
	assert.Equal(t, "10", o.ExecutedQuantity.String())
}

func TestTrimLowerString(t *testing.T) {
	type args struct {
		original string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "spaces",
			args: args{
				original: "  ",
			},
			want: "",
		},
		{
			name: "uppercase",
			args: args{
				original: " HELLO ",
			},
			want: "hello",
		},
		{
			name: "lowercase",
			args: args{
				original: " hello",
			},
			want: "hello",
		},
		{
			name: "upper/lower cases",
			args: args{
				original: " heLLo  ",
			},
			want: "hello",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TrimLowerString(tt.args.original); got != tt.want {
				t.Errorf("TrimLowerString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_toGlobalSymbol(t *testing.T) {
	assert.Equal(t, "BTCUSDT", toGlobalSymbol("BTC/USDT"))
}

func Test_toLocalOrderTypeWithLimitMaker(t *testing.T) {
	orderType, err := toLocalOrderType(types.OrderTypeLimitMaker)
	assert.NoError(t, err)
	assert.Equal(t, ftxapi.OrderTypeLimit, orderType)
}

func Test_toLocalOrderTypeWithLimit(t *testing.T) {
	orderType, err := toLocalOrderType(types.OrderTypeLimit)
	assert.NoError(t, err)
	assert.Equal(t, ftxapi.OrderTypeLimit, orderType)
}

func Test_toLocalOrderTypeWithMarket(t *testing.T) {
	orderType, err := toLocalOrderType(types.OrderTypeMarket)
	assert.NoError(t, err)
	assert.Equal(t, ftxapi.OrderTypeMarket, orderType)
}
