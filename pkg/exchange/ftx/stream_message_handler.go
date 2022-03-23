package ftx

import (
	"encoding/json"

	"github.com/c9s/bbgo/pkg/types"
)

type messageHandler struct {
	*types.StandardStream
}

func (h *messageHandler) handleMessage(message []byte) {
	var r websocketResponse
	if err := json.Unmarshal(message, &r); err != nil {
		logger.WithError(err).Errorf("failed to unmarshal resp: %s", string(message))
		return
	}

	if r.Type == errRespType {
		logger.Errorf("receives err: %+v", r)
		return
	}

	if r.Type == pongRespType {
		return
	}

	switch r.Channel {
	case orderBookChannel:
		h.handleOrderBook(r)
	case bookTickerChannel:
		h.handleBookTicker(r)
	case marketTradeChannel:
		h.handleMarketTrade(r)
	case privateOrdersChannel:
		h.handlePrivateOrders(r)
	case privateTradesChannel:
		h.handleTrades(r)
	default:
		logger.Warnf("unsupported message type: %+v", r.Type)
	}
}

// {"type": "subscribed", "channel": "orderbook", "market": "BTC/USDT"}
func (h messageHandler) handleSubscribedMessage(response websocketResponse) {
	r, err := response.toSubscribedResponse()
	if err != nil {
		logger.WithError(err).Errorf("failed to convert the subscribed message")
		return
	}
	logger.Info(r)
}

func (h *messageHandler) handleOrderBook(response websocketResponse) {
	if response.Type == subscribedRespType {
		h.handleSubscribedMessage(response)
		return
	}
	r, err := response.toPublicOrderBookResponse()
	if err != nil {
		logger.WithError(err).Errorf("failed to convert the public orderbook")
		return
	}

	globalOrderBook, err := toGlobalOrderBook(r)
	if err != nil {
		logger.WithError(err).Errorf("failed to generate orderbook snapshot")
		return
	}

	switch r.Type {
	case partialRespType:
		if err := r.verifyChecksum(); err != nil {
			logger.WithError(err).Errorf("invalid orderbook snapshot")
			return
		}
		h.EmitBookSnapshot(globalOrderBook)
	case updateRespType:
		// emit updates, not the whole orderbook
		h.EmitBookUpdate(globalOrderBook)
	default:
		logger.Errorf("unsupported order book data type %s", r.Type)
		return
	}
}

func (h *messageHandler) handleMarketTrade(response websocketResponse) {
	if response.Type == subscribedRespType {
		h.handleSubscribedMessage(response)
		return
	}
	trades, err := response.toMarketTradeResponse()
	if err != nil {
		logger.WithError(err).Errorf("failed to generate market trade %v", response)
		return
	}
	for _, trade := range trades {
		h.EmitMarketTrade(trade)
	}
}

func (h *messageHandler) handleBookTicker(response websocketResponse) {
	if response.Type == subscribedRespType {
		h.handleSubscribedMessage(response)
		return
	}

	r, err := response.toBookTickerResponse()
	if err != nil {
		logger.WithError(err).Errorf("failed to convert the book ticker")
		return
	}

	globalBookTicker, err := toGlobalBookTicker(r)
	if err != nil {
		logger.WithError(err).Errorf("failed to generate book ticker")
		return
	}

	switch r.Type {
	case updateRespType:
		// emit updates, not the whole orderbook
		h.EmitBookTickerUpdate(globalBookTicker)
	default:
		logger.Errorf("unsupported book ticker data type %s", r.Type)
		return
	}
}

func (h *messageHandler) handlePrivateOrders(response websocketResponse) {
	if response.Type == subscribedRespType {
		h.handleSubscribedMessage(response)
		return
	}

	r, err := response.toOrderUpdateResponse()
	if err != nil {
		logger.WithError(err).Errorf("failed to convert the order update response")
		return
	}

	globalOrder, err := toGlobalOrderNew(r.Data)
	if err != nil {
		logger.WithError(err).Errorf("failed to convert order update to global order")
		return
	}
	h.EmitOrderUpdate(globalOrder)
}

func (h *messageHandler) handleTrades(response websocketResponse) {
	if response.Type == subscribedRespType {
		h.handleSubscribedMessage(response)
		return
	}

	r, err := response.toTradeUpdateResponse()
	if err != nil {
		logger.WithError(err).Errorf("failed to convert the trade update response")
		return
	}

	t, err := toGlobalTrade(r.Data)
	if err != nil {
		logger.WithError(err).Errorf("failed to convert trade update to global trade ")
		return
	}
	h.EmitTradeUpdate(t)
}
