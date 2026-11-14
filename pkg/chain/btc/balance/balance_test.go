package balance

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/chain/btc"
	"github.com/c9s/bbgo/pkg/chain/btc/esplora"
)

const (
	alice = "bc1qyz5ts3dl4wxntavwfkugsj25zaaj8hcwmjkl6h"
	bob   = "1AfCc4F9c4VTYSE31PUe2kUEKs6ZxiDjxm"
)

func outpoint(n int) btc.Outpoint {
	var h btc.Hash
	binary.BigEndian.PutUint64(h[:8], uint64(n))
	return btc.Outpoint{TxID: h}
}

func TestCompute(t *testing.T) {
	s := &Snapshot{
		Height: 1000,
		UTXOs: []btc.UTXO{
			{Outpoint: outpoint(1), Address: alice, Value: 100, Height: 990},                // 11 confirmations
			{Outpoint: outpoint(2), Address: alice, Value: 20, Height: 1000},                // 1 confirmation
			{Outpoint: outpoint(3), Address: alice, Value: 3},                               // mempool
			{Outpoint: outpoint(4), Address: bob, Value: 5000, Height: 950, Coinbase: true}, // immature
			{Outpoint: outpoint(5), Address: bob, Value: 7000, Height: 900, Coinbase: true}, // mature: 101 confirmations
			{Outpoint: outpoint(6), Script: "5121aa21bb52ae", Value: 9, Height: 999},        // no address
		},
	}

	balances := Compute(s, 6)
	require.Len(t, balances, 3)

	a := Of(balances, alice)
	assert.Equal(t, btc.Amount(100), a.Confirmed)
	assert.Equal(t, btc.Amount(23), a.Pending, "1 confirmation and mempool are below 6")
	assert.Equal(t, btc.Amount(0), a.Immature)
	assert.Equal(t, 3, a.UTXOs)
	assert.Equal(t, btc.Amount(123), a.Total())

	b := Of(balances, bob)
	assert.Equal(t, btc.Amount(7000), b.Confirmed)
	assert.Equal(t, btc.Amount(5000), b.Immature)

	script := Of(balances, "5121aa21bb52ae")
	assert.Equal(t, btc.Amount(9), script.Pending)

	total := Sum(balances)
	assert.Equal(t, btc.Amount(7100), total.Confirmed)
	assert.Equal(t, btc.Amount(32), total.Pending)
	assert.Equal(t, btc.Amount(5000), total.Immature)
	assert.Equal(t, 6, total.UTXOs)

	// with 1 confirmation the block output is confirmed; with 0 the mempool too
	assert.Equal(t, btc.Amount(120), Of(Compute(s, 1), alice).Confirmed)
	assert.Equal(t, btc.Amount(123), Of(Compute(s, 0), alice).Confirmed)
	assert.Equal(t, btc.Amount(5000), Of(Compute(s, 0), bob).Immature, "maturity does not depend on minConfirmations")
}

func TestOf(t *testing.T) {
	balances := []Balance{{Address: alice, Confirmed: 1}}

	assert.Equal(t, btc.Amount(1), Of(balances, "BC1QYZ5TS3DL4WXNTAVWFKUGSJ25ZAAJ8HCWMJKL6H").Confirmed, "bech32 is case-insensitive")

	none := Of(balances, bob)
	assert.Equal(t, bob, none.Address)
	assert.Equal(t, btc.Amount(0), none.Total())
}

func TestCoreSource(t *testing.T) {
	var params []any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params []any `json:"params"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		params = req.Params

		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{"success":true,"txouts":1,"height":975765,`+
			`"bestblock":"000000000000000000007356df50fa66f5954437ac5b93fa7d08f887ee576e34","unspents":[`+
			`{"txid":"96e573944a25f8a74fc8e123d20a17277199b43034e01c622c58c8e4176f6b1d","vout":1,`+
			`"scriptPubKey":"76a91469f2a01f4ff9e6ac24df9062e9828753474b348088ac","desc":"addr(`+bob+`)#lzyd9nfu",`+
			`"amount":3.12696278,"coinbase":true,"height":975765}],"total_amount":3.12696278}}`)
	}))
	defer srv.Close()

	client, err := btc.NewClient(srv.URL)
	require.NoError(t, err)

	source := &CoreSource{Client: client}
	s, err := source.Snapshot(context.Background(), []string{bob, "wpkh(xpub/0/*)"})
	require.NoError(t, err)

	assert.Equal(t, []any{"start", []any{"addr(" + bob + ")", "wpkh(xpub/0/*)"}}, params)
	assert.Equal(t, int64(975765), s.Height)
	require.Len(t, s.UTXOs, 1)
	assert.Equal(t, bob, s.UTXOs[0].Address)
	assert.True(t, s.UTXOs[0].Coinbase)

	b := Of(Compute(s, 1), bob)
	assert.Equal(t, btc.Amount(312696278), b.Immature, "a coinbase output of the tip block")
}

func TestEsploraSource(t *testing.T) {
	tip := "975780"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/blocks/tip/height":
			_, _ = io.WriteString(w, tip)
		case "/api/address/" + alice + "/utxo":
			_, _ = io.WriteString(w, `[`+
				`{"txid":"342c2d78348d21da57c4f749bf3780a475bbdedd47856577130a822e412cee79","vout":0,`+
				`"status":{"confirmed":true,"block_height":975725,"block_hash":"00000000000000000001d903c0bb3b2b835d5c52f3bf303f22a1f8d08fc72b19","block_time":1794614176},"value":103708151},`+
				`{"txid":"571568ee5ee5ec7a7646e31d3525d12451488ca5190da09e72fcae441ac20ad5","vout":1,`+
				`"status":{"confirmed":false},"value":2000}]`)
		case "/api/address/" + bob + "/utxo":
			// a block was found while listing: the utxo is above the tip read before
			_, _ = io.WriteString(w, `[{"txid":"4dcf175d2d7115f5fd6a025e038450d812859fcef5f77790d31ab6abc4e2ad11","vout":3,`+
				`"status":{"confirmed":true,"block_height":975781,"block_hash":"0000000000000000000179711b8c79109436c0c9c9019d86c75f27cbd90e2958"},"value":5}]`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client, err := esplora.NewClient(srv.URL+"/api", nil)
	require.NoError(t, err)

	s, err := (&EsploraSource{Client: client}).Snapshot(context.Background(), []string{alice, bob})
	require.NoError(t, err)

	assert.Equal(t, int64(975781), s.Height, "raised to the highest utxo block")
	require.Len(t, s.UTXOs, 3)

	balances := Compute(s, 1)
	a := Of(balances, alice)
	assert.Equal(t, btc.Amount(103708151), a.Confirmed)
	assert.Equal(t, btc.Amount(2000), a.Pending)
	assert.Equal(t, btc.Amount(5), Of(balances, bob).Confirmed)
}
