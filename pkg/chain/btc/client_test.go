package btc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	fixtureBlockHash = "000000000000000000007356df50fa66f5954437ac5b93fa7d08f887ee576e34"
	fixtureTxID      = "c9be41ea2f86946bababb451dc63b1e3c355f3214ee00fb2c3a3e7d9c6748dce"
)

// fakeNode answers each RPC method with a canned result and records the
// params of the last call.
type fakeNode struct {
	t       *testing.T
	results map[string]string
	params  map[string][]any
}

func newFakeNode(t *testing.T, results map[string]string) (*Client, *fakeNode) {
	node := &fakeNode{t: t, results: results, params: map[string][]any{}}
	srv := httptest.NewServer(node)
	t.Cleanup(srv.Close)

	c, err := NewClient(srv.URL)
	require.NoError(t, err)

	return c, node
}

func (n *fakeNode) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID     uint64 `json:"id"`
		Method string `json:"method"`
		Params []any  `json:"params"`
	}
	require.NoError(n.t, json.NewDecoder(r.Body).Decode(&req))
	n.params[req.Method] = req.Params

	result, ok := n.results[req.Method]
	if !ok {
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","error":{"code":-32601,"message":"Method not found"},"id":1}`)
		return
	}

	_, _ = io.WriteString(w, `{"jsonrpc":"2.0","result":`+result+`,"id":1}`)
}

func blockFixture(t *testing.T) string {
	raw, err := os.ReadFile("testdata/block_975765.json")
	require.NoError(t, err)

	var resp struct {
		Result json.RawMessage `json:"result"`
	}
	require.NoError(t, json.Unmarshal(raw, &resp))

	return string(resp.Result)
}

func TestGetBlock(t *testing.T) {
	c, node := newFakeNode(t, map[string]string{"getblock": blockFixture(t)})

	block, err := c.GetBlock(context.Background(), MustHash(fixtureBlockHash))
	require.NoError(t, err)

	assert.Equal(t, []any{fixtureBlockHash, float64(3)}, node.params["getblock"])

	assert.Equal(t, fixtureBlockHash, block.Hash.String())
	assert.Equal(t, int64(975765), block.Height)
	assert.Equal(t, int64(7730), block.NTx)
	require.NotNil(t, block.PreviousBlockHash)
	assert.Equal(t, "00000000000000000000ba61763f56b53e2412be7e437cfd8fba00a465a47e12", block.PreviousBlockHash.String())
	assert.Nil(t, block.NextBlockHash, "the tip has no next block")

	require.NotEmpty(t, block.Tx)

	// the coinbase creates coins: no prevout, no fee
	coinbase := block.Tx[0]
	assert.True(t, coinbase.IsCoinbase())
	assert.True(t, coinbase.Vin[0].TxID.IsZero())
	assert.Nil(t, coinbase.Vin[0].Prevout)
	assert.Nil(t, coinbase.Fee)
	assert.Equal(t, Amount(546), coinbase.Vout[0].Value)
	assert.Equal(t, Amount(312696278), coinbase.Vout[1].Value)
	assert.Equal(t, ScriptTypeNullData, coinbase.Vout[2].ScriptPubKey.Type)
	assert.Empty(t, coinbase.Vout[2].ScriptPubKey.Address)

	_, ok := coinbase.InputValue()
	assert.False(t, ok)

	seen := map[ScriptType]bool{}
	for i := range block.Tx[1:] {
		tx := &block.Tx[1+i]
		assert.False(t, tx.IsCoinbase())

		// the fee is not in the transaction; it is what the inputs leave over
		in, ok := tx.InputValue()
		require.True(t, ok, tx.TxID.String())
		require.NotNil(t, tx.Fee, tx.TxID.String())
		assert.Equal(t, *tx.Fee, in-tx.OutputValue(), tx.TxID.String())

		for _, vin := range tx.Vin {
			seen[vin.Prevout.ScriptPubKey.Type] = true
		}
	}

	for _, typ := range []ScriptType{
		ScriptTypePubKeyHash, ScriptTypeScriptHash, ScriptTypeWitnessV0KeyHash,
		ScriptTypeWitnessV0ScriptHash, ScriptTypeWitnessV1Taproot,
	} {
		assert.True(t, seen[typ], "the fixture spends a %s output", typ)
	}

	first := block.Tx[1]
	assert.Equal(t, fixtureTxID, first.TxID.String())
	assert.Equal(t, "a3335070923e31eec296ed7ab12b09ed89c14c59ad0e902f111908fc8db755c7:2", first.Vin[0].Outpoint().String())
	assert.Equal(t, "bc1qyz5ts3dl4wxntavwfkugsj25zaaj8hcwmjkl6h", first.Vin[0].Prevout.ScriptPubKey.Address)
	assert.Equal(t, int64(975764), first.Vin[0].Prevout.Height)
	assert.Equal(t, Amount(17777402), first.Vin[0].Prevout.Value)
}

func TestGetBlockTxIDs(t *testing.T) {
	c, node := newFakeNode(t, map[string]string{
		"getblock": `{"hash":"` + fixtureBlockHash + `","confirmations":1,"height":975765,"nTx":2,` +
			`"tx":["` + fixtureTxID + `","` + genesisHash + `"]}`,
	})

	header, txids, err := c.GetBlockTxIDs(context.Background(), MustHash(fixtureBlockHash))
	require.NoError(t, err)

	assert.Equal(t, float64(1), node.params["getblock"][1])
	assert.Equal(t, int64(975765), header.Height)
	assert.Equal(t, []Hash{MustHash(fixtureTxID), MustHash(genesisHash)}, txids)
}

func TestGetBlockHeader(t *testing.T) {
	c, node := newFakeNode(t, map[string]string{
		"getblockheader": `{"hash":"` + genesisHash + `","confirmations":975766,"height":0,"version":1,` +
			`"versionHex":"00000001","merkleroot":"4a5e1e4baab89f3a32518a88c31bc87f618f76673e2cc77ab2127b7afdeda33b",` +
			`"time":1231006505,"mediantime":1231006505,"nonce":2083236893,"bits":"1d00ffff","difficulty":1,` +
			`"chainwork":"0000000000000000000000000000000000000000000000000000000100010001","nTx":1,` +
			`"nextblockhash":"00000000839a8e6886ab5951d76f411475428afc90947ee320161bbf18eb6048"}`,
	})

	header, err := c.GetBlockHeader(context.Background(), MustHash(genesisHash))
	require.NoError(t, err)

	assert.Equal(t, []any{genesisHash, true}, node.params["getblockheader"])
	assert.Nil(t, header.PreviousBlockHash, "genesis has no parent")
	require.NotNil(t, header.NextBlockHash)
	assert.Equal(t, uint32(2083236893), header.Nonce)
}

func TestGetRawTransaction(t *testing.T) {
	block := blockFixture(t)

	var b struct {
		Tx []json.RawMessage `json:"tx"`
	}
	require.NoError(t, json.Unmarshal([]byte(block), &b))

	c, node := newFakeNode(t, map[string]string{"getrawtransaction": string(b.Tx[1])})

	blockHash := MustHash(fixtureBlockHash)
	tx, err := c.GetRawTransaction(context.Background(), MustHash(fixtureTxID), &blockHash)
	require.NoError(t, err)
	assert.Equal(t, []any{fixtureTxID, float64(2), fixtureBlockHash}, node.params["getrawtransaction"])
	assert.Equal(t, fixtureTxID, tx.TxID.String())

	_, err = c.GetRawTransaction(context.Background(), MustHash(fixtureTxID), nil)
	require.NoError(t, err)
	assert.Equal(t, []any{fixtureTxID, float64(2)}, node.params["getrawtransaction"])
}

func TestGetTxOut(t *testing.T) {
	results := map[string]string{
		"gettxout": `{"bestblock":"` + fixtureBlockHash + `","confirmations":1,"value":3.12696278,` +
			`"scriptPubKey":{"asm":"OP_DUP OP_HASH160 69f2a01f4ff9e6ac24df9062e9828753474b3480 OP_EQUALVERIFY OP_CHECKSIG",` +
			`"desc":"addr(1AfCc4F9c4VTYSE31PUe2kUEKs6ZxiDjxm)#lzyd9nfu","hex":"76a91469f2a01f4ff9e6ac24df9062e9828753474b348088ac",` +
			`"address":"1AfCc4F9c4VTYSE31PUe2kUEKs6ZxiDjxm","type":"pubkeyhash"},"coinbase":true}`,
	}
	c, node := newFakeNode(t, results)

	outpoint := Outpoint{TxID: MustHash("96e573944a25f8a74fc8e123d20a17277199b43034e01c622c58c8e4176f6b1d"), Vout: 1}
	out, err := c.GetTxOut(context.Background(), outpoint, true)
	require.NoError(t, err)

	assert.Equal(t, []any{outpoint.TxID.String(), float64(1), true}, node.params["gettxout"])
	assert.Equal(t, Amount(312696278), out.Value)
	assert.True(t, out.Coinbase)
	assert.Equal(t, "1AfCc4F9c4VTYSE31PUe2kUEKs6ZxiDjxm", out.ScriptPubKey.Address)

	// a spent output is a null result
	results["gettxout"] = "null"
	_, err = c.GetTxOut(context.Background(), outpoint, false)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestScanTxOutSet(t *testing.T) {
	c, node := newFakeNode(t, map[string]string{
		"scantxoutset": `{"success":true,"txouts":187123456,"height":975765,"bestblock":"` + fixtureBlockHash + `",` +
			`"unspents":[{"txid":"` + fixtureTxID + `","vout":0,"scriptPubKey":"0014baba02f9dd4c3c135653eca61fe78f9cba0d8399",` +
			`"desc":"addr(bc1qh2aq97wafs7px4jnajnpleu0njaqmque8dku3p)#pqslph3d","amount":0.17760000,"coinbase":false,` +
			`"height":975765,"blockhash":"` + fixtureBlockHash + `","confirmations":1}],"total_amount":0.17760000}`,
	})

	result, err := c.ScanTxOutSet(context.Background(), []string{"addr(bc1qh2aq97wafs7px4jnajnpleu0njaqmque8dku3p)"})
	require.NoError(t, err)

	assert.Equal(t, []any{"start", []any{"addr(bc1qh2aq97wafs7px4jnajnpleu0njaqmque8dku3p)"}}, node.params["scantxoutset"])
	assert.True(t, result.Success)
	require.Len(t, result.Unspents, 1)
	assert.Equal(t, fixtureTxID+":0", result.Unspents[0].Outpoint().String())
	assert.Equal(t, Amount(17760000), result.TotalAmount)
	require.NotNil(t, result.Unspents[0].BlockHash)
}

func TestMempool(t *testing.T) {
	c, _ := newFakeNode(t, map[string]string{
		"getrawmempool": `["6e970d9a276b4976c67f955fe8fc28eff9dc2212fb9d1b77a8d2f1d3bd797b6b"]`,
		"getmempoolentry": `{"vsize_adjusted":141,"vsize":141,"vsize_bip141":141,"weight":561,"time":1794641886,` +
			`"height":975765,"descendantcount":1,"descendantsize":141,"ancestorcount":1,"ancestorsize":141,` +
			`"wtxid":"c8966fbaab427ebcbd9880df7e795d903d3b88131dd0076ce2469a02ab0f89d6","chunkweight":561,` +
			`"fees":{"base":0.00021150,"modified":0.00021150,"ancestor":0.00021150,"descendant":0.00021150,"chunk":0.00021150},` +
			`"depends":[],"spentby":[],"unbroadcast":false,"bip125-replaceable":true}`,
	})

	txids, err := c.GetRawMempool(context.Background())
	require.NoError(t, err)
	require.Len(t, txids, 1)

	entry, err := c.GetMempoolEntry(context.Background(), txids[0])
	require.NoError(t, err)
	assert.Equal(t, int64(141), entry.VSize)
	assert.Equal(t, Amount(21150), entry.Fees.Base)
	assert.True(t, entry.BIP125Replaceable)
	assert.Empty(t, entry.Depends)
}
