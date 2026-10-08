package binance

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

const futuresAccountStatusResponse = `{
  "id": "account-status-1",
  "status": 200,
  "result": {
    "totalInitialMargin": "21.59210000",
    "totalMaintMargin": "0.86368400",
    "totalWalletBalance": "103.12345678",
    "totalUnrealizedProfit": "-1.23000000",
    "totalMarginBalance": "101.89345678",
    "totalPositionInitialMargin": "21.59210000",
    "totalOpenOrderInitialMargin": "0.00000000",
    "totalCrossWalletBalance": "103.12345678",
    "totalCrossUnPnl": "-1.23000000",
    "availableBalance": "80.30135678",
    "maxWithdrawAmount": "80.30135678",
    "assets": [
      {
        "asset": "USDT",
        "walletBalance": "103.12345678",
        "unrealizedProfit": "-1.23000000",
        "marginBalance": "101.89345678",
        "maintMargin": "0.86368400",
        "initialMargin": "21.59210000",
        "positionInitialMargin": "21.59210000",
        "openOrderInitialMargin": "0.00000000",
        "crossWalletBalance": "103.12345678",
        "crossUnPnl": "-1.23000000",
        "availableBalance": "80.30135678",
        "maxWithdrawAmount": "80.30135678",
        "updateTime": 1625474304765
      }
    ],
    "positions": [
      {
        "symbol": "BTCUSDT",
        "positionSide": "BOTH",
        "positionAmt": "0.002",
        "unrealizedProfit": "-1.23000000",
        "isolatedMargin": "0",
        "notional": "215.92100000",
        "isolatedWallet": "0",
        "initialMargin": "21.59210000",
        "maintMargin": "0.86368400",
        "updateTime": 1625474304765
      }
    ]
  },
  "rateLimits": [
    {"rateLimitType": "REQUEST_WEIGHT", "interval": "MINUTE", "intervalNum": 1, "limit": 2400, "count": 10}
  ]
}`

func TestParseFuturesWsApiResponse(t *testing.T) {
	t.Run("account status", func(t *testing.T) {
		e, err := parseFuturesWsApiResponse([]byte(futuresAccountStatusResponse))
		require.NoError(t, err)

		event, ok := e.(*FuturesAccountStatusEvent)
		require.True(t, ok, "unexpected event type %T", e)
		assert.Equal(t, "account-status-1", event.ID)
		assert.Equal(t, 200, event.Status)
		assert.False(t, event.ReceivedAt.IsZero())
		assert.Equal(t, fixedpoint.MustNewFromString("0.86368400"), event.Account.TotalMaintMargin)
		assert.Equal(t, fixedpoint.MustNewFromString("101.89345678"), event.Account.TotalMarginBalance)
		require.Len(t, event.RateLimits, 1)
		assert.Equal(t, 10, event.RateLimits[0].Count)

		account := event.FuturesAccount()
		assert.Equal(t, fixedpoint.MustNewFromString("0.86368400"), account.TotalMaintMargin)
		assert.Equal(t, fixedpoint.MustNewFromString("101.89345678"), account.TotalMarginBalance)
		assert.Equal(t, fixedpoint.MustNewFromString("0.86368400"), account.Assets["USDT"].MaintMargin)
		assert.Len(t, account.Positions, 1)
	})

	t.Run("error", func(t *testing.T) {
		e, err := parseFuturesWsApiResponse([]byte(`{"id":"account-status-2","status":400,"error":{"code":-1021,"msg":"Timestamp for this request is outside of the recvWindow."}}`))
		require.NoError(t, err)

		event, ok := e.(*futuresWsApiErrorEvent)
		require.True(t, ok, "unexpected event type %T", e)
		assert.Equal(t, 400, event.Status)
		assert.Equal(t, -1021, event.Error.Code)
	})
}

func TestEnableFuturesAccountStatusUpdate(t *testing.T) {
	ex := New("key", "secret")
	ex.UseFutures()
	stream := ex.NewStream().(*Stream)
	assert.False(t, stream.shouldUseFuturesAccountStatusStream())

	stream.EnableFuturesAccountStatusUpdate(0)
	assert.Equal(t, DefaultFuturesAccountStatusInterval, stream.futuresAccountStatusInterval)
	assert.True(t, stream.shouldUseFuturesAccountStatusStream())

	stream.EnableFuturesAccountStatusUpdate(time.Millisecond)
	assert.Equal(t, minFuturesAccountStatusInterval, stream.futuresAccountStatusInterval)

	stream.SetPublicOnly()
	assert.False(t, stream.shouldUseFuturesAccountStatusStream())

	spot := New("key", "secret").NewStream().(*Stream)
	spot.EnableFuturesAccountStatusUpdate(time.Second)
	assert.False(t, spot.shouldUseFuturesAccountStatusStream())
}

func TestFuturesAccountStatusStream(t *testing.T) {
	const key, secret = "test-key", "test-secret"

	type request struct {
		ID     string `json:"id"`
		Method string `json:"method"`
		Params struct {
			APIKey    string `json:"apiKey"`
			Timestamp int64  `json:"timestamp"`
			Signature string `json:"signature"`
		} `json:"params"`
	}

	requests := make(chan request, 16)
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		for {
			var req request
			if err := conn.ReadJSON(&req); err != nil {
				return
			}
			requests <- req

			resp := strings.Replace(futuresAccountStatusResponse, `"account-status-1"`, strconv.Quote(req.ID), 1)
			if err := conn.WriteMessage(websocket.TextMessage, []byte(resp)); err != nil {
				return
			}
		}
	}))
	defer server.Close()

	ex := New(key, secret)
	ex.UseFutures()
	stream := ex.NewStream().(*Stream)
	stream.futuresAccountStatusInterval = 200 * time.Millisecond

	events := make(chan *FuturesAccountStatusEvent, 16)
	stream.OnFuturesAccountStatusEvent(func(e *FuturesAccountStatusEvent) {
		events <- e
	})

	aux := stream.initFuturesAccountStatusStream()
	aux.SetEndpointCreator(func(ctx context.Context) (string, error) {
		return "ws" + strings.TrimPrefix(server.URL, "http"), nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, aux.Connect(ctx))
	defer aux.Close()

	// the first request is sent on connect, the following ones on every interval
	for i := 1; i <= 2; i++ {
		select {
		case req := <-requests:
			assert.Equal(t, "account-status-"+strconv.Itoa(i), req.ID)
			assert.Equal(t, futuresAccountStatusMethod, req.Method)
			assert.Equal(t, key, req.Params.APIKey)

			mac := hmac.New(sha256.New, []byte(secret))
			mac.Write([]byte("apiKey=" + key + "&timestamp=" + strconv.FormatInt(req.Params.Timestamp, 10)))
			assert.Equal(t, hex.EncodeToString(mac.Sum(nil)), req.Params.Signature)

		case <-time.After(3 * time.Second):
			t.Fatalf("timed out waiting for request %d", i)
		}

		select {
		case e := <-events:
			assert.Equal(t, "account-status-"+strconv.Itoa(i), e.ID)
			assert.Equal(t, fixedpoint.MustNewFromString("0.86368400"), e.Account.TotalMaintMargin)

		case <-time.After(3 * time.Second):
			t.Fatalf("timed out waiting for event %d", i)
		}
	}
}
