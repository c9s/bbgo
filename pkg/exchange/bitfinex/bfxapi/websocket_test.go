package bfxapi

import (
	"bufio"
	"context"
	"os"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/testutil"
)

const publicMessageRecordFile = "testdata/bitfinex_ws_raw.jsonl"

const privateMessageRecordFile = "testdata/bitfinex_ws_private_raw.jsonl"

func TestWebSocketRecord(t *testing.T) {
	if os.Getenv("TEST_BFX_WS_RECORD") == "" {
		t.Skip("TEST_BFX_WS_RECORD env not set, skipping real websocket test")
	}

	url := PublicWebSocketURL
	c, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("failed to connect to bitfinex ws: %v", err)
	}
	defer c.Close()

	file, err := os.Create(publicMessageRecordFile)
	if err != nil {
		t.Fatalf("failed to create output file: %v", err)
	}

	defer func() {
		if err := file.Sync(); err != nil {
			logrus.Errorf("failed to sync file: %v", err)
		}
		if err := file.Close(); err != nil {
			logrus.Errorf("failed to close file: %v", err)
		}
	}()

	// use WebSocketRequest struct for subscription
	subscribe := func(channel Channel, symbol string) error {
		req := WebSocketRequest{
			Event:   "subscribe",
			Channel: channel,
			Symbol:  symbol,
		}
		return c.WriteJSON(req)
	}

	if err := subscribe(ChannelTrades, "tBTCUSD"); err != nil {
		t.Fatalf("failed to subscribe trades: %v", err)
	}
	if err := subscribe(ChannelBook, "tBTCUSD"); err != nil {
		t.Fatalf("failed to subscribe book: %v", err)
	}
	if err := subscribe(ChannelBook, "fUST"); err != nil {
		t.Fatalf("failed to subscribe book: %v", err)
	}
	if err := subscribe(ChannelTicker, "tBTCUSD"); err != nil {
		t.Fatalf("failed to subscribe ticker: %v", err)
	}
	if err := subscribe(ChannelTicker, "fUST"); err != nil {
		t.Fatalf("failed to subscribe ticker: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			t.Log("timeout reached, closing connection")
			return
		default:
			if err := c.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
				logrus.Errorf("failed to set read deadline: %v", err)
				return
			}
			_, msg, err := c.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					t.Log("websocket closed normally")
					return
				}
				logrus.Errorf("read error: %v", err)
				return
			}
			if _, err := file.Write(append(msg, '\n')); err != nil {
				logrus.Errorf("failed to write message: %v", err)
			}
			t.Logf("received message: %s", string(msg))
		}
	}
}

func TestParserParseFromFile(t *testing.T) {
	t.Run(privateMessageRecordFile, func(t *testing.T) {
		file, err := os.Open(privateMessageRecordFile)
		if err != nil {
			t.Fatalf("failed to open record file: %v", err)
		}

		defer file.Close()

		parser := NewParser()

		scanner := bufio.NewScanner(file)
		lineNum := 0
		errorCount := 0

		numUserTrade := 0
		numUserOrder := 0
		numUnhandled := 0
		var unhandledMessages []string
		for scanner.Scan() {
			lineNum++
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			result, err := parser.Parse(line)
			if err != nil {
				errorCount++
				assert.NoError(t, err, "parse error at line %d: %v", lineNum, err)
				t.Logf("skipping line %d due to parse error: %s", lineNum, string(line))
				continue
			} else if result == nil {
				numUnhandled++
				unhandledMessages = append(unhandledMessages, string(line))
				continue
			}

			// t.Logf("parsed line %d: %T %+v", lineNum, result, result)

			switch tr := result.(type) { // tr = typed result
			case *WebSocketResponse:
				t.Logf("websocket response: %+v from line %s", tr, string(line))
				switch tr.Event {
				case "info":
				case "error":
					assert.NotZero(t, tr.Code)
					assert.NotEmpty(t, tr.Message)
				case "auth":
					assert.Equal(t, "OK", tr.Status)
				}

			case *WalletSnapshotEvent:
				assert.NotEmpty(t, tr.Wallets)

			case *UserOrderSnapshotEvent:

			case *UserPositionSnapshotEvent:

			case *TradeUpdateEvent:
				numUserTrade++
				assert.NotZero(t, tr.ID)
				assert.NotEmpty(t, tr.Symbol)
				assert.False(t, tr.ExecAmount.IsZero())
				assert.False(t, tr.ExecPrice.IsZero())

			case *UserOrder:
				numUserOrder++
				assert.NotZero(t, tr.OrderID)
				assert.NotEmpty(t, tr.Symbol)
				assert.False(t, tr.AmountOrig.IsZero(), string(line))
				assert.False(t, tr.Price.IsZero(), string(line))

			}
		}

		t.Logf("total parse errors: %d", errorCount)
		t.Logf("total user trades: %d", numUserTrade)
		t.Logf("total unhandled: %d", numUnhandled)
		if numUnhandled > 0 {
			t.Logf("unhandled messages: %d", len(unhandledMessages))
			for i, msg := range unhandledMessages {
				if i < 10 {
					t.Logf("unhandled message %d: %s", i+1, msg)
				} else {
					break
				}
			}
		}
	})

	t.Run(publicMessageRecordFile, func(t *testing.T) {
		file, err := os.Open(publicMessageRecordFile)
		if err != nil {
			t.Fatalf("failed to open record file: %v", err)
		}
		defer file.Close()

		parser := NewParser()

		scanner := bufio.NewScanner(file)
		lineNum := 0
		errorCount := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			result, err := parser.Parse(line)
			if err != nil {
				errorCount++
				assert.NoError(t, err, "parse error at line %d: %v", lineNum, err)
				t.Logf("skipping line %d due to parse error: %s", lineNum, string(line))
				continue
			}

			// t.Logf("parsed line %d: %T %+v", lineNum, result, result)

			switch tr := result.(type) { // tr = typed result
			case *WebSocketResponse:
				assert.NotEmpty(t, tr.Event)
				switch tr.Event {
				case "subscribed":
					assert.NotZero(t, tr.ChanId)
				}
			case *TickerEvent:
				assert.NotZero(t, tr.ChannelID)
				assert.False(t, tr.Ask.IsZero())
				assert.False(t, tr.Bid.IsZero())
			case *CandleSnapshotEvent:

			case *FundingTickerEvent:
				assert.NotZero(t, tr.ChannelID)
				assert.False(t, tr.FRR.IsZero())
			case *MarketTradeEvent:
				assert.NotZero(t, tr.ChannelID)
				assert.NotZero(t, tr.ID)
				assert.True(t, !tr.Price.IsZero())
			case []MarketTradeEvent: // trade snapshot
				for _, trade := range tr {
					assert.NotZero(t, trade.ChannelID)
					assert.NotZero(t, trade.ID)
					assert.False(t, trade.Price.IsZero(), "price should not be zero for market trades")
					assert.False(t, trade.Amount.IsZero(), "amount should not be zero for market trades")
				}

			case *FundingMarketTradeEvent:
				assert.NotZero(t, tr.ChannelID)
				assert.NotZero(t, tr.ID)
				assert.False(t, tr.Amount.IsZero(), "amount should not be zero for funding trades")

			case *FundingBookUpdateEvent:
				assert.NotZero(t, tr.ChannelID)
				assert.NotZero(t, tr.Entry.Period)
				assert.False(t, tr.Entry.Amount.IsZero(), "amount should not be zero for funding book events")
				assert.False(t, tr.Entry.Rate.IsZero(), "rate should not be zero for funding book events")

			case *FundingBookSnapshotEvent:
				assert.NotZero(t, tr.ChannelID)
				for _, fundingBookEvent := range tr.Entries {
					assert.NotZero(t, fundingBookEvent.Period)
					assert.False(t, fundingBookEvent.Amount.IsZero(), "amount should not be zero for funding book events")
					assert.False(t, fundingBookEvent.Rate.IsZero(), "rate should not be zero for funding book events")
				}

			case *BookUpdateEvent:
				assert.NotZero(t, tr.ChannelID)
				assert.False(t, tr.Entry.Price.IsZero(), "price should not be zero for book events")
				assert.False(t, tr.Entry.Amount.IsZero(), "amount should not be zero for book events")
			case *BookSnapshotEvent:
				assert.NotZero(t, tr.ChannelID)
				for _, bookEvent := range tr.Entries {
					assert.False(t, bookEvent.Price.IsZero(), "price should not be zero for book events")
					assert.False(t, bookEvent.Amount.IsZero(), "amount should not be zero for book events")
				}
			case *HeartBeatEvent:
			default:
				t.Logf("unhandled type at line %d: %T %+v", lineNum, result, result)
			}
		}

		if err := scanner.Err(); err != nil {
			t.Fatalf("scanner error: %v", err)
		}

		t.Logf("total parse errors: %d", errorCount)
	})

}

func TestParser_Parse(t *testing.T) {
	t.Run("auth response", func(t *testing.T) {
		body := `{"event":"auth","status":"OK","chanId":0,"userId":99999,"auth_id":"0c9d85ec-1eef-4079-9703-1f2ef19feb90","caps":{"bfxpay":{"read":0,"write":0},"orders":{"read":1,"write":1},"account":{"read":1,"write":0},"funding":{"read":1,"write":1},"history":{"read":1,"write":0},"wallets":{"read":1,"write":0},"settings":{"read":1,"write":0},"withdraw":{"read":0,"write":0},"positions":{"read":1,"write":1},"ui_withdraw":{"read":0,"write":0}}}`
		p := NewParser()
		msg, err := p.Parse([]byte(body))
		assert.NoError(t, err)
		if assert.NotNil(t, msg) {
			authMessage, ok := msg.(*WebSocketResponse)
			assert.True(t, ok, "expected WebSocketResponse type")
			assert.Equal(t, "auth", authMessage.Event, "expected auth event")
			assert.Equal(t, "OK", authMessage.Status, "expected OK status")
			assert.Equal(t, float64(0), authMessage.ChanId.(float64), "expected chanId to be 0")
			assert.Equal(t, int64(99999), authMessage.UserId, "expected userId to be 99999")
		}
	})

	t.Run("wallet snapshot", func(t *testing.T) {
		body := `[0,"ws",[["exchange","UST",239.02464552,0,null,null,null],["exchange","BTC",0.0009982,0,null,"Exchange 0.00156 BTC for UST @ 121790.0",{"reason":"TRADE","order_id":22334,"order_id_oppo":12234,"trade_price":"121790.0","trade_amount":"-0.00156","order_cid":33445,"order_gid":null}],["funding","UST",150.00146503,0,null,null,null]]]`
		p := NewParser()
		msg, err := p.Parse([]byte(body))
		assert.NoError(t, err)
		if assert.NotNil(t, msg) {
			snapshot, ok := msg.(*WalletSnapshotEvent)
			assert.True(t, ok, "expected *WalletSnapshotEvent type")

			wallets := snapshot.Wallets
			assert.Len(t, wallets, 3)

			assert.Equal(t, WalletTypeExchange, wallets[0].Type)
			assert.Equal(t, "UST", wallets[0].Currency)
			assert.InDelta(t, 239.02464552, wallets[0].Balance.Float64(), 1e-8)
			assert.True(t, wallets[0].AvailableBalance.IsZero())

			assert.Equal(t, WalletTypeExchange, wallets[1].Type)
			assert.Equal(t, "BTC", wallets[1].Currency)
			assert.InDelta(t, 0.0009982, wallets[1].Balance.Float64(), 1e-8)
			assert.Equal(t, "Exchange 0.00156 BTC for UST @ 121790.0", wallets[1].LastChange)
			if assert.NotNil(t, wallets[1].LastChangeMetaData) {
				switch data := wallets[1].LastChangeMetaData.Data.(type) {
				case *WalletTradeDetail:
					assert.Equal(t, "TRADE", data.Reason)
					assert.Equal(t, int64(22334), data.OrderId)
					assert.Equal(t, int64(12234), data.OrderIdOppo)
					assert.InDelta(t, 121790.0, data.TradePrice.Float64(), 1e-8)
					assert.InDelta(t, -0.00156, data.TradeAmount.Float64(), 1e-8)
					assert.Equal(t, int64(33445), data.OrderCid)
					assert.Zero(t, data.OrderGid)
				}
			}

			assert.Equal(t, WalletTypeFunding, wallets[2].Type)
			assert.Equal(t, "UST", wallets[2].Currency)
			assert.InDelta(t, 150.00146503, wallets[2].Balance.Float64(), 1e-8)
		}
	})

	t.Run("wallet update - UST exchange", func(t *testing.T) {
		body := `[0,"wu",["exchange","UST",239.02464552,0,239.02464552,null,null]]`
		p := NewParser()
		msg, err := p.Parse([]byte(body))
		assert.NoError(t, err)
		if assert.NotNil(t, msg) {
			wallet, ok := msg.(*Wallet)
			assert.True(t, ok, "expected Wallet type")
			assert.Equal(t, WalletTypeExchange, wallet.Type)
			assert.Equal(t, "UST", wallet.Currency)
			assert.InDelta(t, 239.02464552, wallet.Balance.Float64(), 1e-8)
			assert.InDelta(t, 239.02464552, wallet.AvailableBalance.Float64(), 1e-8)
			assert.Nil(t, wallet.LastChangeMetaData)
		}
	})

	t.Run("wallet update - BTC exchange", func(t *testing.T) {
		body := `[0,"wu",["exchange","BTC",0.0009982,0,0.0009982,"Exchange 0.00156 BTC for UST @ 121790.0",{"reason":"TRADE","order_id":21439,"order_id_oppo":21440,"trade_price":"121790.0","trade_amount":"-0.00156","order_cid":17540000,"order_gid":null}]]`
		p := NewParser()
		msg, err := p.Parse([]byte(body))
		assert.NoError(t, err)
		if assert.NotNil(t, msg) {
			wallet, ok := msg.(*Wallet)
			assert.True(t, ok, "expected Wallet type")
			assert.Equal(t, WalletTypeExchange, wallet.Type)
			assert.Equal(t, "BTC", wallet.Currency)
			assert.InDelta(t, 0.0009982, wallet.Balance.Float64(), 1e-8)
			assert.InDelta(t, 0.0009982, wallet.AvailableBalance.Float64(), 1e-8)
			assert.Equal(t, "Exchange 0.00156 BTC for UST @ 121790.0", wallet.LastChange)
			if assert.NotNil(t, wallet.LastChangeMetaData) {
				switch data := wallet.LastChangeMetaData.Data.(type) {
				case *WalletTradeDetail:
					assert.Equal(t, "TRADE", data.Reason)
					assert.Equal(t, int64(21439), data.OrderId)
					assert.Equal(t, int64(21440), data.OrderIdOppo)
					assert.InDelta(t, 121790.0, data.TradePrice.Float64(), 1e-8)
					assert.InDelta(t, -0.00156, data.TradeAmount.Float64(), 1e-8)
					assert.Equal(t, int64(17540000), data.OrderCid)
					assert.Zero(t, data.OrderGid)
				}
			}
		}
	})

	t.Run("wallet update - UST funding", func(t *testing.T) {
		body := `[0,"wu",["funding","UST",150.00146503,0,0.0014660299999889048,null,null]]`
		p := NewParser()
		msg, err := p.Parse([]byte(body))
		assert.NoError(t, err)
		if assert.NotNil(t, msg) {
			wallet, ok := msg.(*Wallet)
			assert.True(t, ok, "expected Wallet type")
			assert.Equal(t, WalletTypeFunding, wallet.Type)
			assert.Equal(t, "UST", wallet.Currency)
			assert.InDelta(t, 150.00146503, wallet.Balance.Float64(), 1e-8)
			assert.InDelta(t, 0.0014660299999889048, wallet.AvailableBalance.Float64(), 1e-8)
			assert.Nil(t, wallet.LastChangeMetaData)
		}
	})

	t.Run("order new", func(t *testing.T) {
		body := `[0,"on",[215271,null,1755668711760,"tBTCUST",1755668711760,1755668711762,0.001,0.001,"EXCHANGE LIMIT",null,null,null,0,"ACTIVE",null,null,10000,0,0,0,null,null,null,0,0,null,null,null,"API>BFX",null,null,{"source":"api"}]]`
		p := NewParser()
		msg, err := p.Parse([]byte(body))
		assert.NoError(t, err)
		if assert.NotNil(t, msg) {
			order, ok := msg.(*UserOrder)
			t.Logf("parsed order: %+v", order)
			if assert.True(t, ok, "expected UserOrder type") {
				assert.Equal(t, int64(215271), order.OrderID)
				assert.Equal(t, "tBTCUST", order.Symbol)
				assert.InDelta(t, 0.001, order.Amount.Float64(), 1e-8)
				assert.InDelta(t, 10000, order.Price.Float64(), 1e-8)
				assert.Equal(t, OrderTypeExchangeLimit, order.OrderType)
				assert.Equal(t, "ACTIVE", order.Status)
				assert.Equal(t, "API>BFX", *order.Routing)
				if assert.NotNil(t, order.Meta) {
					t.Logf("order meta: %+v", order.Meta)
					assert.Equal(t, "api", order.Meta["source"])
				}
			}
		}
	})

	t.Run("trade update", func(t *testing.T) {
		body := `[0,"tu",[17972,"tBTCUSD",1755671464700,15268096239,-0.00008867,113910,"EXCHANGE LIMIT",113910,-1,-0.00606023982,"USD",1755671464699]]`
		p := NewParser()
		msg, err := p.Parse([]byte(body))
		assert.NoError(t, err)
		if assert.NotNil(t, msg) {
			trade, ok := msg.(*TradeUpdateEvent)
			assert.True(t, ok, "expected TradeUpdate type")
			assert.Equal(t, int64(17972), trade.ID)
			assert.Equal(t, "tBTCUSD", trade.Symbol)
			assert.Equal(t, int64(1755671464700), trade.Time.Time().UnixMilli())
			assert.Equal(t, int64(15268096239), trade.OrderID)
			assert.InDelta(t, -0.00008867, trade.ExecAmount.Float64(), 1e-8)
			assert.InDelta(t, 113910, trade.ExecPrice.Float64(), 1e-8)
			assert.Equal(t, "EXCHANGE LIMIT", trade.OrderType)
			assert.InDelta(t, -0.00606023982, trade.Fee.Float64(), 1e-8)

			if assert.NotNil(t, trade.FeeCurrency) {
				assert.Equal(t, "USD", *trade.FeeCurrency)
				assert.Equal(t, int64(1755671464699), trade.ClientOrderID)
			}
		}
	})

	t.Run("trade execute", func(t *testing.T) {
		body := `[0,"te",[17972,"tBTCUSD",1755671464700,15268096239,-0.00008867,113910,"EXCHANGE LIMIT",113910,-1,null,null,1755671464699]]`
		p := NewParser()
		msg, err := p.Parse([]byte(body))
		assert.NoError(t, err)
		if assert.NotNil(t, msg) {
			trade, ok := msg.(*TradeUpdateEvent)
			assert.True(t, ok, "expected TradeExecute type")
			assert.Equal(t, int64(17972), trade.ID)
			assert.Equal(t, "tBTCUSD", trade.Symbol)
			assert.Equal(t, int64(1755671464700), trade.Time.Time().UnixMilli())
			assert.Equal(t, int64(15268096239), trade.OrderID)
			assert.InDelta(t, -0.00008867, trade.ExecAmount.Float64(), 1e-8)
			assert.InDelta(t, 113910, trade.ExecPrice.Float64(), 1e-8)
			assert.Equal(t, "EXCHANGE LIMIT", trade.OrderType)
			assert.Equal(t, int64(1755671464699), trade.ClientOrderID)
		}
	})

	t.Run("funding offer snapshot", func(t *testing.T) {
		body := `[0,"fos",[[41237920,"fETH",1573912039000,1573912039000,0.5,0.5,"LIMIT",null,null,0,"ACTIVE",null,null,null,0.0024,2,0,0,null,0,null]]]`
		p := NewParser()
		msg, err := p.Parse([]byte(body))
		assert.NoError(t, err)
		if assert.NotNil(t, msg) {
			events, ok := msg.([]FundingOfferUpdateEvent)
			assert.True(t, ok, "expected []FundingOfferUpdateEvent type")
			assert.Len(t, events, 1)
			fo := events[0]
			assert.Equal(t, int64(41237920), fo.OfferID)
			assert.Equal(t, "fETH", fo.Symbol)
			assert.Equal(t, int64(1573912039000), fo.MtsCreated.Time().UnixMilli())
			assert.Equal(t, int64(1573912039000), fo.MtsUpdated.Time().UnixMilli())
			assert.InDelta(t, 0.5, fo.Amount.Float64(), 1e-8)
			assert.InDelta(t, 0.5, fo.AmountOrig.Float64(), 1e-8)
			assert.Equal(t, "LIMIT", fo.OfferType)
			assert.Equal(t, int64(0), fo.Flags)
			assert.Equal(t, "ACTIVE", fo.Status)
			assert.InDelta(t, 0.0024, fo.Rate.Float64(), 1e-8)
			assert.Equal(t, int64(2), fo.Period)
			assert.Equal(t, int64(0), fo.Notify)
			assert.Equal(t, int64(0), fo.Hidden)
			assert.Equal(t, int64(0), fo.Renew)
		}
	})

	t.Run("funding offer update", func(t *testing.T) {
		body := `[0,"fon",[41238747,"fUST",1575026670000,1575026670000,5000,5000,"LIMIT",null,null,0,"ACTIVE",null,null,null,0.006000000000000001,30,0,0,null,0,null]]`
		p := NewParser()
		msg, err := p.Parse([]byte(body))
		assert.NoError(t, err)
		if assert.NotNil(t, msg) {
			event, ok := msg.(*FundingOfferUpdateEvent)
			assert.True(t, ok, "expected FundingOfferUpdateEvent type")
			assert.Equal(t, int64(41238747), event.OfferID)
			assert.Equal(t, "fUST", event.Symbol)
			assert.Equal(t, int64(1575026670000), event.MtsCreated.Time().UnixMilli())
			assert.Equal(t, int64(1575026670000), event.MtsUpdated.Time().UnixMilli())
			assert.InDelta(t, 5000, event.Amount.Float64(), 1e-8)
			assert.InDelta(t, 5000, event.AmountOrig.Float64(), 1e-8)
			assert.Equal(t, "LIMIT", event.OfferType)
			assert.Equal(t, int64(0), event.Flags)
			assert.Equal(t, "ACTIVE", event.Status)
			assert.InDelta(t, 0.006000000000000001, event.Rate.Float64(), 1e-8)
			assert.Equal(t, int64(30), event.Period)
			assert.Equal(t, int64(0), event.Notify)
			assert.Equal(t, int64(0), event.Hidden)
			assert.Equal(t, int64(0), event.Renew)
		}
	})

	// funding info update
	t.Run("funding info update", func(t *testing.T) {
		body := `[0,"fiu",["sym","fUSD",[0.0008595462068208099,0,1.8444560185185186,0]]]`
		p := NewParser()
		msg, err := p.Parse([]byte(body))
		assert.NoError(t, err)
		if assert.NotNil(t, msg) {
			event, ok := msg.(*FundingInfoEvent)
			assert.True(t, ok, "expected FundingInfoEvent type")

			info := event.Info
			assert.Equal(t, "sym", event.UpdateType)
			assert.Equal(t, "fUSD", event.Symbol)
			assert.InDelta(t, 0.0008595462068208099, info.YieldLoan.Float64(), 1e-8)
			assert.InDelta(t, 0, info.YieldLend.Float64(), 1e-8)
			assert.InDelta(t, 1.8444560185185186, info.DurationLoan.Float64(), 1e-8)
			assert.InDelta(t, 0, info.DurationLend.Float64(), 1e-8)
		}
	})
}

func TestWebSocketRecordPrivate(t *testing.T) {
	if os.Getenv("TEST_BFX_WS_PRIVATE_RECORD") == "" {
		t.Skip("TEST_BFX_WS_PRIVATE_RECORD env not set, skipping real websocket test")
	}

	key, secret, ok := testutil.IntegrationTestConfigured(t, "BITFINEX")
	if !ok {
		t.Skip("BITFINEX integration test not configured, skipping private websocket test")
	}

	var orderIds []int64
	url := PrivateWebSocketURL
	c, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("failed to connect to bitfinex private ws: %v", err)
	}
	defer c.Close()

	file, err := os.Create(privateMessageRecordFile)
	if err != nil {
		t.Fatalf("failed to create output file: %v", err)
	}
	defer func() {
		if err := file.Sync(); err != nil {
			logrus.Errorf("failed to sync file: %v", err)
		}
		if err := file.Close(); err != nil {
			logrus.Errorf("failed to close file: %v", err)
		}
	}()

	// send auth message
	authMsg := GenerateAuthRequest(key, secret)
	if err := c.WriteJSON(authMsg); err != nil {
		t.Fatalf("failed to send auth message: %v", err)
	}

	// subscribe to wallet channel (see Bitfinex docs)
	walletSub := WebSocketRequest{
		Event:   "subscribe",
		Channel: "wallet",
	}
	if err := c.WriteJSON(walletSub); err != nil {
		t.Fatalf("failed to subscribe wallet: %v", err)
	}

	client := NewClient()
	client.Auth(key, secret)

	client.NewSubmitOrderRequest()

	ctx := context.Background()

	submitOrder := func(symbol string, price fixedpoint.Value) {
		req := newLimitOrderRequest(client, symbol, price)
		resp, err := req.Do(ctx)
		if assert.NoError(t, err) {
			t.Logf("submit order response: %+v", resp)
			if assert.NotNil(t, resp.Data, "expected order data in response") {
				order := resp.Data[0]
				if order.Status == "ACTIVE" {
					orderIds = append(orderIds, order.OrderID)
				}
			}
		} else {
			return
		}
	}

	ticker, err := client.NewGetTickerRequest().Symbol("tBTCUSD").Do(ctx)
	assert.NoError(t, err)

	submitOrder("tBTCUSD", ticker.Bid.Mul(n(0.99)).Neg()) // a small sell taker order
	submitOrder("tBTCUSD", n(10000))                      // small amount for test
	submitOrder("tBTCUSD", ticker.Ask.Mul(n(1.01)))       // a small taker order

	t.Cleanup(func() {
		t.Logf("test case %s cleaning up", t.Name())

		for _, id := range orderIds {
			// cancel the submitted order to clean up
			cancelReq := client.NewCancelOrderRequest()
			cancelReq.OrderID(id)

			if cancelResp, err := cancelReq.Do(context.Background()); err == nil {
				t.Logf("cancel order response: %+v", cancelResp)
			}
		}
	})

	recordCtx, cancelRecord := context.WithTimeout(ctx, 1*time.Minute)
	defer cancelRecord()

	for {
		select {
		case <-recordCtx.Done():
			t.Log("timeout reached, closing connection")
			return
		default:
			if err := c.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
				log.WithError(err).Errorf("failed to set websocket read deadline")
				return
			}

			_, msg, err := c.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					t.Log("websocket closed normally")
					return
				}

				log.WithError(err).Errorf("websocket read error")
				return
			}

			if _, err := file.Write(append(msg, '\n')); err != nil {
				log.WithError(err).Errorf("failed to write websocket message")
			}

			t.Logf("received message: %s", string(msg))
		}
	}
}

func newLimitOrderRequest(client *Client, symbol string, price fixedpoint.Value) *SubmitOrderRequest {
	req := client.NewSubmitOrderRequest()
	req.Symbol(symbol)

	amount := fixedpoint.NewFromFloat(10.0).Div(price)
	req.Amount(amount.String())     // small amount for test
	req.Price(price.Abs().String()) // far from market price to avoid execution
	req.OrderType(OrderTypeExchangeLimit)
	return req
}

func n(value float64) fixedpoint.Value {
	return fixedpoint.NewFromFloat(value)
}
