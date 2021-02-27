package ftx

type operation string

const subscribe operation = "subscribe"
const unsubscribe operation = "unsubscribe"

type channel string

const orderbook channel = "orderbook"
const trades channel = "trades"
const ticker channel = "ticker"

// {'op': 'subscribe', 'channel': 'trades', 'market': 'BTC-PERP'}
type SubscribeRequest struct {
	Operation operation `json:"op"`
	Channel   channel   `json:"channel"`
	Market    string    `json:"market"`
}
