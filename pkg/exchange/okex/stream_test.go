package okex

import (
	"os"
	"strconv"
	"testing"

	"github.com/c9s/bbgo/pkg/exchange/okex/okexapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/testutil"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func getTestClientOrSkip(t *testing.T) *Stream {
	if b, _ := strconv.ParseBool(os.Getenv("CI")); b {
		t.Skip("skip test for CI")
	}

	key, secret, passphrase, ok := testutil.IntegrationTestWithPassphraseConfigured(t, "OKEX")
	if !ok {
		t.Skip("Please configure all credentials about OKEX")
		return nil
	}

	exchange := New(key, secret, passphrase)
	return NewStream(exchange.client)
}

func TestStream_parseWebSocketEvent(t *testing.T) {
	s := Stream{}

	t.Run("op", func(t *testing.T) {
		input := `{
			"event": "subscribe",
			"arg": {
			  "channel": "tickers",
			  "instId": "LTC-USDT"
			}
		  }`
		res, err := s.parseWebSocketEvent([]byte(input))
		assert.NoError(t, err)
		opEvent, ok := res.(*WebSocketOpEvent)
		assert.True(t, ok)
		assert.Equal(t, WebSocketOpEvent{
			Event: WsOpTypeSubscribe,
			Arg: WebsocketSubscription{
				Channel:      WebSocketChannelType("tickers"),
				InstrumentID: "LTC-USDT",
			},
		}, *opEvent)
	})

	t.Run("account event", func(t *testing.T) {
		input := `{
			"arg": {
			  "channel": "account",
			  "ccy": "BTC",
			  "uid": "77982378738415879"
			},
			"data": [
			  {
				"uTime": "1597026383085",
				"totalEq": "41624.32",
				"isoEq": "3624.32",
				"adjEq": "41624.32",
				"ordFroz": "0",
				"imr": "4162.33",
				"mmr": "4",
				"borrowFroz": "",
				"notionalUsd": "",
				"mgnRatio": "41624.32",
				"details": [
				  {
					"availBal": "",
					"availEq": "1",
					"ccy": "BTC",
					"cashBal": "1",
					"uTime": "1617279471503",
					"disEq": "50559.01",
					"eq": "1",
					"eqUsd": "45078.3790756226851775",
					"frozenBal": "0",
					"interest": "0",
					"isoEq": "0",
					"liab": "0",
					"maxLoan": "",
					"mgnRatio": "",
					"notionalLever": "0.0022195262185864",
					"ordFrozen": "0",
					"upl": "0",
					"uplLiab": "0",
					"crossLiab": "0",
					"isoLiab": "0",
					"coinUsdPrice": "60000",
					"stgyEq":"0",
					"spotInUseAmt":"",
					"isoUpl":"",
					"borrowFroz": ""
				  }
				]
			  }
			]
		  }`
		res, err := s.parseWebSocketEvent([]byte(input))
		assert.NoError(t, err)
		opEvent, ok := res.(*okexapi.Account)
		assert.True(t, ok)
		assert.Equal(t, okexapi.Account{
			TotalEquityInUSD: fixedpoint.MustNewFromString("41624.32"),
			UpdateTime:       types.MustParseMillisecondTimestamp("1597026383085"),
			Details: []okexapi.BalanceDetail{
				{Currency: "BTC",
					Available:               fixedpoint.MustNewFromString("1"),
					CashBalance:             fixedpoint.MustNewFromString("1"),
					OrderFrozen:             fixedpoint.MustNewFromString("0"),
					Frozen:                  fixedpoint.MustNewFromString("0"),
					Equity:                  fixedpoint.MustNewFromString("1"),
					EquityInUSD:             fixedpoint.MustNewFromString("45078.3790756226851775"),
					UpdateTime:              types.MustParseMillisecondTimestamp("1617279471503"),
					UnrealizedProfitAndLoss: fixedpoint.MustNewFromString("0"),
				},
			},
		}, *opEvent)
	})
}
