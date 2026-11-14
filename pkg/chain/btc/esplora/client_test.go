package esplora

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/chain/btc"
)

func TestAddressUTXOs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/address/bc1qxnycg69jw22uew2l8l2n6w4nxf60nmt7dc4yp8/utxo", r.URL.Path)
		_, _ = io.WriteString(w, `[{"txid":"66d9eec1f2da1bb6d2cae6030c25994632891bae945f2192e16f24005b19831d","vout":0,`+
			`"status":{"confirmed":true,"block_height":975765,"block_hash":"000000000000000000007356df50fa66f5954437ac5b93fa7d08f887ee576e34","block_time":1794641661},`+
			`"value":99983380},`+
			`{"txid":"571568ee5ee5ec7a7646e31d3525d12451488ca5190da09e72fcae441ac20ad5","vout":1,"status":{"confirmed":false},"value":2000}]`)
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL+"/api/", nil)
	require.NoError(t, err)

	utxos, err := c.AddressUTXOs(context.Background(), "bc1qxnycg69jw22uew2l8l2n6w4nxf60nmt7dc4yp8")
	require.NoError(t, err)
	require.Len(t, utxos, 2)

	assert.Equal(t, "66d9eec1f2da1bb6d2cae6030c25994632891bae945f2192e16f24005b19831d:0", utxos[0].Outpoint.String())
	assert.Equal(t, btc.Amount(99983380), utxos[0].Value)
	assert.Equal(t, int64(975765), utxos[0].Height)
	assert.Equal(t, "000000000000000000007356df50fa66f5954437ac5b93fa7d08f887ee576e34", utxos[0].BlockHash.String())
	assert.Equal(t, "bc1qxnycg69jw22uew2l8l2n6w4nxf60nmt7dc4yp8", utxos[0].Address)

	assert.False(t, utxos[1].Confirmed())
	assert.True(t, utxos[1].BlockHash.IsZero())
}

func TestErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/blocks/tip/height" {
			_, _ = io.WriteString(w, "not a number")
			return
		}

		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, "Too many unspent transaction outputs (>500). Contact support to raise limits.")
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, nil)
	require.NoError(t, err)

	_, err = c.AddressUTXOs(context.Background(), "bc1qhp7n555hk5mxlp3pz0347nzah8gazwe7wp5fjj")
	assert.ErrorContains(t, err, "http 400: Too many unspent")

	_, err = c.TipHeight(context.Background())
	assert.ErrorContains(t, err, "invalid tip height")

	_, err = NewClient("ftp://example.com", nil)
	assert.Error(t, err)
}

// TestLiveEsplora runs against a real instance when ESPLORA_URL is set, e.g.
//
//	ESPLORA_URL=https://mempool.space/api go test ./pkg/chain/btc/esplora -run TestLiveEsplora -v
func TestLiveEsplora(t *testing.T) {
	baseURL := os.Getenv("ESPLORA_URL")
	if baseURL == "" {
		t.Skip("ESPLORA_URL is not set")
	}

	c, err := NewClient(baseURL, nil)
	require.NoError(t, err)

	tip, err := c.TipHeight(context.Background())
	require.NoError(t, err)
	assert.Greater(t, tip, int64(975000))

	// the address of the 1 BTC output of tx 66d9eec1... in block 975765
	utxos, err := c.AddressUTXOs(context.Background(), "bc1qxnycg69jw22uew2l8l2n6w4nxf60nmt7dc4yp8")
	require.NoError(t, err)
	t.Logf("tip %d, %d utxos", tip, len(utxos))
}
