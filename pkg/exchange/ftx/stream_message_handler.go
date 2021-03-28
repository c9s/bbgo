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

	switch r.Channel {
	case orderBookChannel:
		h.handleOrderBook(r)
	case privateOrdersChannel:
		h.handlePrivateOrders(r)
	default:
		if r.Type != errRespType {
			logger.Errorf("unsupported message type: %+v", r.Type)
			return
		}
		logger.Errorf("received err: %s", r.toErrResponse())
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

	globalOrder, err := toGlobalOrder(r.Data)
	if err != nil {
		logger.WithError(err).Errorf("failed to convert order update to global order")
		return
	}
	h.EmitOrderUpdate(globalOrder)
}
