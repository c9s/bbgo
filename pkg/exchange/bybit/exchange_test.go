package bybit

import (
	"context"
	"github.com/c9s/bbgo/pkg/exchange/bybit/bybitapi"
	"github.com/c9s/bbgo/pkg/exchange/bybit/bybitapi/mocks"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestExchange_QueryTicker(t *testing.T) {
	client := mocks.NewClient(t)

	client.EXPECT().GetInstruInfoDo(context.Background()).Return(&bybitapi.InstrumentsInfo{List: []bybitapi.Instrument{{
		Symbol:        "",
		BaseCoin:      "",
		QuoteCoin:     "",
		Innovation:    "",
		Status:        "",
		MarginTrading: "",
		LotSizeFilter: struct {
			BasePrecision  fixedpoint.Value `json:"basePrecision"`
			QuotePrecision fixedpoint.Value `json:"quotePrecision"`
			MinOrderQty    fixedpoint.Value `json:"minOrderQty"`
			MaxOrderQty    fixedpoint.Value `json:"maxOrderQty"`
			MinOrderAmt    fixedpoint.Value `json:"minOrderAmt"`
			MaxOrderAmt    fixedpoint.Value `json:"maxOrderAmt"`
		}{},
		PriceFilter: struct {
			TickSize fixedpoint.Value `json:"tickSize"`
		}{},
	}}}, nil).Once()

	e := &Exchange{client: client}
	_, err := e.QueryMarkets(context.Background())
	assert.NoError(t, err)
	// output:
	//=== RUN   TestExchange_QueryTicker
	//    Client.go:266: PASS:	GetInstruInfoDo(*context.emptyCtx)
	//--- PASS: TestExchange_QueryTicker (0.00s)
	//PASS
}

func TestExchange_QueryTickerOutOfCall(t *testing.T) {
	client := mocks.NewClient(t)

	client.EXPECT().GetInstruInfoSymbol("BTCUSDT").Return(client) // unexpected call
	client.EXPECT().GetInstruInfoDo(context.Background()).Return(&bybitapi.InstrumentsInfo{List: []bybitapi.Instrument{{
		Symbol:        "",
		BaseCoin:      "",
		QuoteCoin:     "",
		Innovation:    "",
		Status:        "",
		MarginTrading: "",
		LotSizeFilter: struct {
			BasePrecision  fixedpoint.Value `json:"basePrecision"`
			QuotePrecision fixedpoint.Value `json:"quotePrecision"`
			MinOrderQty    fixedpoint.Value `json:"minOrderQty"`
			MaxOrderQty    fixedpoint.Value `json:"maxOrderQty"`
			MinOrderAmt    fixedpoint.Value `json:"minOrderAmt"`
			MaxOrderAmt    fixedpoint.Value `json:"maxOrderAmt"`
		}{},
		PriceFilter: struct {
			TickSize fixedpoint.Value `json:"tickSize"`
		}{},
	}}}, nil).Once()

	e := &Exchange{client: client}
	_, err := e.QueryMarkets(context.Background())
	assert.NoError(t, err)
	// output:
	//=== RUN   TestExchange_QueryTickerOutOfCall
	//    Client.go:266: FAIL:	GetInstruInfoSymbol(string)
	//        		at: [/Users/Edwin/Workspace/go/src/github.com/bailantaotao/bbgo/pkg/exchange/bybit/bybitapi/mocks/Client.go:236 /Users/Edwin/Workspace/go/src/github.com/bailantaotao/bbgo/pkg/exchange/bybit/exchange_test.go:49]
	//    Client.go:266: PASS:	GetInstruInfoDo(*context.emptyCtx)
	//    Client.go:266: FAIL: 1 out of 2 expectation(s) were met.
	//        	The code you are testing needs to make 1 more call(s).
	//        	at: [/Users/Edwin/Workspace/go/src/github.com/bailantaotao/bbgo/pkg/exchange/bybit/bybitapi/mocks/Client.go:266 /usr/local/go/src/testing/testing.go:1150 /usr/local/go/src/testing/testing.go:1328 /usr/local/go/src/testing/testing.go:1570]
	//--- FAIL: TestExchange_QueryTickerOutOfCall (0.00s)
	//
	//FAIL
}
