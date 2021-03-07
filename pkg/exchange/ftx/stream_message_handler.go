package ftx

import (
	"encoding/json"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/types"
)

type messageHandler struct {
	*types.StandardStream
}

func (h *messageHandler) handleMessage(message []byte) {
	var r rawResponse
	if err := json.Unmarshal(message, &r); err != nil {
		logger.WithError(err).Errorf("failed to unmarshal resp: %s", string(message))
		return
	}

	switch r.Type {
	case subscribedRespType:
		h.handleSubscribedMessage(r)
	case partialRespType, updateRespType:
		h.handleMarketData(r)
	default:
		logger.Errorf("unsupported message type: %+v", r.Type)
	}
}

// {"type": "subscribed", "channel": "orderbook", "market": "BTC/USDT"}
func (h messageHandler) handleSubscribedMessage(response rawResponse) {
	r := response.toSubscribedResp()
	logger.Infof("%s %s is subscribed", r.Market, r.Channel)
}

func (h *messageHandler) handleMarketData(response rawResponse) {
	r, err := response.toOrderBookResponse()
	if err != nil {
		log.WithError(err).Errorf("failed to convert the partial response to data response")
		return
	}

	switch r.Channel {
	case orderbook:
		h.handleOrderBook(r)
	default:
		log.Errorf("unsupported market data channel %s", r.Channel)
		return
	}
}

func (h *messageHandler) handleOrderBook(r orderBookResponse) {
	globalOrderBook, err := toGlobalOrderBook(r)
	if err != nil {
		log.WithError(err).Errorf("failed to generate orderbook snapshot")
		return
	}

	switch r.Type {
	case partialRespType:
		if err := r.verifyChecksum(); err != nil {
			log.WithError(err).Errorf("invalid orderbook snapshot")
			return
		}
		h.EmitBookSnapshot(globalOrderBook)
	case updateRespType:
		// emit updates, not the whole orderbook
		h.EmitBookUpdate(globalOrderBook)
	default:
		log.Errorf("unsupported order book data type %s", r.Type)
		return
	}
}
