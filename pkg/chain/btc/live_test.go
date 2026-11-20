package btc

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLiveNode runs against a real node when BTC_RPC_ENDPOINT is set, e.g.
//
//	BTC_RPC_ENDPOINT=https://bitcoin-rpc.publicnode.com go test ./pkg/chain/btc -run TestLiveNode -v
func TestLiveNode(t *testing.T) {
	endpoint := os.Getenv("BTC_RPC_ENDPOINT")
	if endpoint == "" {
		t.Skip("BTC_RPC_ENDPOINT is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	c, err := NewClient(endpoint)
	require.NoError(t, err)

	info, err := c.GetBlockchainInfo(ctx)
	require.NoError(t, err)
	t.Logf("chain=%s blocks=%d best=%s", info.Chain, info.Blocks, info.BestBlockHash)

	// the tip may move between the calls; read the block by hash from here on
	hash, err := c.GetBlockHash(ctx, info.Blocks)
	require.NoError(t, err)

	block, err := c.GetBlock(ctx, hash)
	require.NoError(t, err)
	require.Equal(t, hash, block.Hash)
	require.NotEmpty(t, block.Tx)
	assert.Equal(t, block.NTx, int64(len(block.Tx)))

	coinbase := &block.Tx[0]
	require.True(t, coinbase.IsCoinbase())

	var fees Amount
	for i := 1; i < len(block.Tx); i++ {
		tx := &block.Tx[i]
		require.False(t, tx.IsCoinbase())

		in, ok := tx.InputValue()
		require.True(t, ok, "verbosity=3 gives every prevout: %s", tx.TxID)
		require.NotNil(t, tx.Fee)
		require.Equal(t, *tx.Fee, in-tx.OutputValue(), tx.TxID.String())

		fees += *tx.Fee
	}

	// the miner may claim at most the subsidy and the fees
	claimed := coinbase.OutputValue()
	assert.LessOrEqual(t, claimed, BlockSubsidy(block.Height)+fees)
	t.Logf("block %d: %d txs, fees %s BTC, coinbase pays %s BTC (subsidy %s)",
		block.Height, len(block.Tx), fees, claimed, BlockSubsidy(block.Height))

	// a coinbase output of the tip is unspent; ask for one with a value
	for _, out := range coinbase.Vout {
		if out.Value == 0 {
			continue
		}

		utxo, err := c.GetTxOut(ctx, Outpoint{TxID: coinbase.TxID, Vout: out.N}, true)
		if err == nil {
			assert.True(t, utxo.Coinbase)
			assert.Equal(t, out.Value, utxo.Value)
		} else {
			// a newer block may have arrived and its miner swept it already: rare
			assert.ErrorIs(t, err, ErrNotFound)
		}

		break
	}

	var count int64
	var best Hash
	batch := []BatchElem{
		{Method: "getblockcount", Result: &count},
		{Method: "getbestblockhash", Result: &best},
	}
	require.NoError(t, c.BatchCall(ctx, batch))
	require.NoError(t, batch[0].Error)
	require.NoError(t, batch[1].Error)
	assert.GreaterOrEqual(t, count, block.Height)

	txids, err := c.GetRawMempool(ctx)
	require.NoError(t, err)
	t.Logf("mempool: %d transactions", len(txids))

	if len(txids) > 0 {
		entry, err := c.GetMempoolEntry(ctx, txids[0])
		if err == nil {
			assert.Positive(t, entry.VSize)
		} else {
			// mined or replaced since getrawmempool
			assert.True(t, IsNotFound(err), err.Error())
		}
	}
}
