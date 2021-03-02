package ftx

import (
	"encoding/json"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/types"
)

type messageHandler struct {
	*types.StandardStream
}

func (h messageHandler) handleMessage(message []byte) {
	var r rawResponse
	if err := json.Unmarshal(message, &r); err != nil {
		logger.WithError(err).Errorf("failed to unmarshal resp: %s", string(message))
		return
	}

	switch r.Type {
	case subscribedRespType:
		h.handleSubscribedMessage(r)
	case partialRespType:
		// snapshot of current market data
		h.handleSnapshot(r)
	case updateRespType:
		//log.Infof("update=> %s", string(message))
	default:
		logger.Errorf("unsupported message type: %+v", r.Type)
	}
}

// {"type": "subscribed", "channel": "orderbook", "market": "BTC/USDT"}
func (h messageHandler) handleSubscribedMessage(response rawResponse) {
	r := response.toSubscribedResp()
	logger.Infof("%s %s is subscribed", r.Market, r.Channel)
}

func (h messageHandler) handleSnapshot(response rawResponse) {
	r, err := response.toSnapshotResp()
	if err != nil {
		log.WithError(err).Errorf("failed to convert the partial response to snapshot")
		return
	}
	h.EmitBookSnapshot(r.toGlobalOrderBook())
}
