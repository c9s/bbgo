package binance

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_newClientOrderID(t *testing.T) {
	cID := newSpotClientOrderID("")
	assert.Len(t, cID, 32)
	strings.HasPrefix(cID, "x-"+spotBrokerID)

	cID = newSpotClientOrderID("myid1")
	assert.Equal(t, cID, "x-"+spotBrokerID+"myid1")
}

func Test_new(t *testing.T) {
	ex := New("", "")
	assert.NotEmpty(t, ex)
	ctx := context.Background()
	ticker, err := ex.QueryTicker(ctx, "btcusdt")
	assert.NotEmpty(t, ticker)
	assert.NoError(t, err)
}
