package ftx

import (
	"encoding/json"

	"github.com/c9s/bbgo/pkg/types"
)

type messageHandler struct {
	types.StandardStream
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
	default:
		logger.Errorf("unsupported message type: %+v", r.Type)
	}
}

// {"type": "subscribed", "channel": "orderbook", "market": "BTC/USDT"}
func (h messageHandler) handleSubscribedMessage(response rawResponse) {
	logger.Infof("%s orderbook is subscribed", response.toSubscribedResp().Market)
}
