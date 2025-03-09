package coinbase

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_ParseHeartbeat(t *testing.T) {
	data := []byte(`
{
  "type": "heartbeat",
  "sequence": 90,
  "last_trade_id": 20,
  "product_id": "BTC-USD",
  "time": "2014-11-07T08:19:28.464459Z"
}	
`)
	msg, err := parseMessage(data)
	assert.NoError(t, err)
	assert.IsType(t, &HeartbeatMessage{}, msg)
}

func Test_ParseStatus(t *testing.T) {
	data := []byte(`
{
  "type": "status",
  "products": [
    {
      "id": "BTC-USD",
      "base_currency": "BTC",
      "quote_currency": "USD",
      "base_increment": "0.00000001",
      "quote_increment": "0.01",
      "display_name": "BTC-USD",
      "status": "online",
      "status_message": null,
      "min_market_funds": "10",
      "post_only": false,
      "limit_only": false,
      "cancel_only": false,
      "fx_stablecoin": false
    }
  ],
  "currencies": [
    {
      "id": "USD",
      "name": "United States Dollar",
      "display_name": "USD",
      "min_size": "0.01000000",
      "status": "online",
      "status_message": null,
      "max_precision": "0.01",
      "convertible_to": ["USDC"],
      "details": {},
      "default_network": "",
      "supported_networks": []
    },
    {
      "id": "BTC",
      "name": "Bitcoin",
      "display_name": "BTC",
      "min_size": "0.00000001",
      "status": "online",
      "status_message": null,
      "max_precision": "0.00000001",
      "convertible_to": [],
      "details": {},
      "default_network": "bitcoin",
      "supported_networks": [
        {
          "id": "bitcoin",
          "name": "Bitcoin",
          "status": "online",
          "contract_address": "",
          "crypto_address_link": "https://live.blockcypher.com/btc/address/{{address}}",
          "crypto_transaction_link": "https://live.blockcypher.com/btc/tx/{{txId}}",
          "min_withdrawal_amount": 0.0001,
          "max_withdrawal_amount": 2400,
          "network_confirmations": 2,
          "processing_time_seconds": 0,
          "destination_tag_regex": ""
        }
      ]
    }
  ]
}
`)
	msg, err := parseMessage(data)
	assert.NoError(t, err)
	assert.IsType(t, &StatusMessage{}, msg)
}

func Test_ParseAuction(t *testing.T) {
	data := []byte(`
{
    "type": "auction",
    "product_id": "LTC-USD",
    "sequence": 3262786978,
    "auction_state": "collection",
    "best_bid_price": "333.98",
    "best_bid_size": "4.39088265",
    "best_ask_price": "333.99",
    "best_ask_size": "25.23542881",
    "open_price": "333.99",
    "open_size": "0.193",
    "can_open": "yes",
    "timestamp": "2015-11-14T20:46:03.511254Z"
}
`)
	msg, err := parseMessage(data)
	assert.NoError(t, err)
	assert.IsType(t, &AuctionMessage{}, msg)
}

func Test_ParseRfq(t *testing.T) {
	data := []byte(`
{
  "type": "rfq_match",
  "maker_order_id": "ac928c66-ca53-498f-9c13-a110027a60e8",
  "taker_order_id": "132fb6ae-456b-4654-b4e0-d681ac05cea1",
  "time": "2014-11-07T08:19:27.028459Z",
  "trade_id": 30,
  "product_id": "BTC-USD",
  "size": "5.23512",
  "price": "400.23",
  "side": "sell"
}
`)
	msg, err := parseMessage(data)
	assert.NoError(t, err)
	assert.IsType(t, &RfqMessage{}, msg)
}

func Test_ParseTicker(t *testing.T) {
	data := []byte(`
{
  "type": "ticker",
  "sequence": 37475248783,
  "product_id": "ETH-USD",
  "price": "1285.22",
  "open_24h": "1310.79",
  "volume_24h": "245532.79269678",
  "low_24h": "1280.52",
  "high_24h": "1313.8",
  "volume_30d": "9788783.60117027",
  "best_bid": "1285.04",
  "best_bid_size": "0.46688654",
  "best_ask": "1285.27",
  "best_ask_size": "1.56637040",
  "side": "buy",
  "time": "2022-10-19T23:28:22.061769Z",
  "trade_id": 370843401,
  "last_size": "11.4396987"
}
`)
	msg, err := parseMessage(data)
	assert.NoError(t, err)
	assert.IsType(t, &TickerMessage{}, msg)
}

func Test_ReceivedMessage(t *testing.T) {
	// limit order
	data := []byte(`
{
  "type": "received",
  "time": "2014-11-07T08:19:27.028459Z",
  "product_id": "BTC-USD",
  "sequence": 10,
  "order_id": "d50ec984-77a8-460a-b958-66f114b0de9b",
  "size": "1.34",
  "price": "502.1",
  "side": "buy",
  "order_type": "limit",
  "client-oid": "d50ec974-76a2-454b-66f135b1ea8c"
}
`)
	msg, err := parseMessage(data)
	assert.NoError(t, err)
	assert.IsType(t, &ReceivedMessage{}, msg)

	// market order
	data = []byte(`
{
  "type": "received",
  "time": "2014-11-09T08:19:27.028459Z",
  "product_id": "BTC-USD",
  "sequence": 12,
  "order_id": "dddec984-77a8-460a-b958-66f114b0de9b",
  "funds": "3000.234",
  "side": "buy",
  "order_type": "market",
  "client-oid": "d50ec974-76a2-454b-66f135b1ea8c"
}
`)
	msg, err = parseMessage(data)
	assert.NoError(t, err)
	assert.IsType(t, &ReceivedMessage{}, msg)
}

func Test_ParseOpen(t *testing.T) {
	data := []byte(`
{
  "type": "open",
  "time": "2014-11-07T08:19:27.028459Z",
  "product_id": "BTC-USD",
  "sequence": 10,
  "order_id": "d50ec984-77a8-460a-b958-66f114b0de9b",
  "price": "200.2",
  "remaining_size": "1.00",
  "side": "sell"
}
`)
	msg, err := parseMessage(data)
	assert.NoError(t, err)
	assert.IsType(t, &OpenMessage{}, msg)
}

func Test_ParseDone(t *testing.T) {
	// filled
	data := []byte(`
{
  "type": "done",
  "time": "2014-11-07T08:19:27.028459Z",
  "product_id": "BTC-USD",
  "sequence": 10,
  "price": "200.2",
  "order_id": "d50ec984-77a8-460a-b958-66f114b0de9b",
  "reason": "filled",
  "side": "sell",
  "remaining_size": "0"
}
`)
	msg, err := parseMessage(data)
	assert.NoError(t, err)
	assert.IsType(t, &DoneMessage{}, msg)

	// canceled
	data = []byte(`
{
  "type": "done",
  "time": "2014-11-07T08:19:27.028459Z",
  "product_id": "BTC-USD",
  "sequence": 10,
  "price": "200.2",
  "order_id": "d50ec984-77a8-460a-b958-66f114b0de9b",
  "reason": "canceled",
  "side": "sell",
  "remaining_size": "8",
  "cancel_reason": "102:Self Trade Prevention"
}
`)
	msg, err = parseMessage(data)
	assert.NoError(t, err)
	assert.IsType(t, &DoneMessage{}, msg)
}

func Test_ParseMatch(t *testing.T) {
	data := []byte(`
{
  "type": "match",
  "trade_id": 10,
  "sequence": 50,
  "maker_order_id": "ac928c66-ca53-498f-9c13-a110027a60e8",
  "taker_order_id": "132fb6ae-456b-4654-b4e0-d681ac05cea1",
  "time": "2014-11-07T08:19:27.028459Z",
  "product_id": "BTC-USD",
  "size": "5.23512",
  "price": "400.23",
  "side": "sell"
}
`)
	msg, err := parseMessage(data)
	matchMsg, ok := msg.(*MatchMessage)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.False(t, matchMsg.IsAuthMaker())

	data = []byte(`
{
  "type": "match",
  "trade_id": 10,
  "sequence": 50,
  "maker_order_id": "ac928c66-ca53-498f-9c13-a110027a60e8",
  "taker_order_id": "132fb6ae-456b-4654-b4e0-d681ac05cea1",
  "time": "2014-11-07T08:19:27.028459Z",
  "product_id": "BTC-USD",
  "size": "5.23512",
  "price": "400.23",
  "side": "sell",
  "taker_user_id": "5844eceecf7e803e259d0365",
  "user_id": "5844eceecf7e803e259d0365",
  "taker_profile_id": "765d1549-9660-4be2-97d4-fa2d65fa3352",
  "profile_id": "765d1549-9660-4be2-97d4-fa2d65fa3352",
  "taker_fee_rate": "0.005"
}
`)
	msg, err = parseMessage(data)
	matchMsg, ok = msg.(*MatchMessage)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.False(t, matchMsg.IsAuthMaker())

	data = []byte(`
{
  "type": "match",
  "trade_id": 10,
  "sequence": 50,
  "maker_order_id": "ac928c66-ca53-498f-9c13-a110027a60e8",
  "taker_order_id": "132fb6ae-456b-4654-b4e0-d681ac05cea1",
  "time": "2014-11-07T08:19:27.028459Z",
  "product_id": "BTC-USD",
  "size": "5.23512",
  "price": "400.23",
  "side": "sell",
  "maker_user_id": "5f8a07f17b7a102330be40a3",
  "user_id": "5f8a07f17b7a102330be40a3",
  "maker_profile_id": "7aa6b75c-0ff1-11eb-adc1-0242ac120002",
  "profile_id": "7aa6b75c-0ff1-11eb-adc1-0242ac120002",
  "maker_fee_rate": "0.001"
}
`)
	msg, err = parseMessage(data)
	matchMsg, ok = msg.(*MatchMessage)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.True(t, matchMsg.IsAuthMaker())
}

func Test_ParseChange(t *testing.T) {
	data := []byte(`
{
  "type": "change",
  "reason":"STP",
  "time": "2014-11-07T08:19:27.028459Z",
  "sequence": 80,
  "order_id": "ac928c66-ca53-498f-9c13-a110027a60e8",
  "side": "sell",
  "product_id": "BTC-USD",
  "old_size": "12.234412",
  "new_size": "5.23512",
  "price": "400.23"
}
`)
	msg, err := parseMessage(data)
	changeMsg, ok := msg.(*ChangeMessage)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.True(t, changeMsg.IsStp())

	data = []byte(`
{
  "type": "change",
  "reason":"modify_order",
  "time": "2022-06-06T22:55:43.433114Z",
  "sequence": 24753,
  "order_id": "c3f16063-77b1-408f-a743-88b7bc20cdcd",
  "side": "buy",
  "product_id": "ETH-USD",
  "old_size": "80",
  "new_size": "80",
  "old_price": "7",
  "new_price": "6"
}
`)
	msg, err = parseMessage(data)
	changeMsg, ok = msg.(*ChangeMessage)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.True(t, changeMsg.IsModifyOrder())
}

func Test_ParseActive(t *testing.T) {
	data := []byte(`
{
  "type": "active",
  "time": "2014-11-07T08:19:27.028459Z",
  "product_id": "BTC-USD",
  "sequence": 10,
  "order_id": "d50ec984-77a8-460a-b958-66f114b0de9b",
  "side": "sell",
  "remaining_size": "1.00"
}
`)
	msg, err := parseMessage(data)
	assert.NoError(t, err)
	assert.IsType(t, &ActivateMessage{}, msg)
}

func Test_ParseBalance(t *testing.T) {
	data := []byte(`
{
  "type": "balance",
  "account_id": "d50ec984-77a8-460a-b958-66f114b0de9b",
  "currency": "USD",
  "holds": "1000.23",
  "available": "102030.99",
  "updated": "2023-10-10T20:42:27.265Z",
  "timestamp": "2023-10-10T20:42:29.265Z"
}
`)
	msg, err := parseMessage(data)
	assert.NoError(t, err)
	assert.IsType(t, &BalanceMessage{}, msg)
}

func Test_ParseOrderBookSnapshot(t *testing.T) {
	data := []byte(`
{
  "type": "snapshot",
  "product_id": "BTC-USD",
  "bids": [["10101.10", "0.45054140"]],
  "asks": [["10102.55", "0.57753524"]]
}
`)
	msg, err := parseMessage(data)
	assert.NoError(t, err)
	assert.IsType(t, &OrderBookSnapshotMessage{}, msg)
}

func Test_ParseOrderBookUpdate(t *testing.T) {
	data := []byte(`
{
  "type": "l2update",
  "product_id": "BTC-USD",
  "changes": [
    [
      "buy",
      "22356.270000",
      "0.00000000"
    ],
    [
      "buy",
      "22356.300000",
      "1.00000000"
    ]
  ],
  "time": "2022-08-04T15:25:05.010758Z"
}
`)
	msg, err := parseMessage(data)
	assert.NoError(t, err)
	assert.IsType(t, &OrderBookUpdateMessage{}, msg)
}
