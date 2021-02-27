package ftx

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_rawResponse_toSubscribedResp(t *testing.T) {
	input := `{"type": "subscribed", "channel": "orderbook", "market": "BTC/USDT"}`
	var m rawResponse
	assert.NoError(t, json.Unmarshal([]byte(input), &m))
	r := m.toSubscribedResp()
	assert.Equal(t, subscribedRespType, r.Type)
	assert.Equal(t, orderbook, r.Channel)
	assert.Equal(t, "BTC/USDT", r.Market)
}
