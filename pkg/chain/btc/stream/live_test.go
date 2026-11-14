package stream

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/chain/btc"
)

// TestLiveStream backfills the last blocks from a real node when
// BTC_RPC_ENDPOINT is set, e.g.
//
//	BTC_RPC_ENDPOINT=https://bitcoin-rpc.publicnode.com go test ./pkg/chain/btc/stream -run TestLiveStream -v
func TestLiveStream(t *testing.T) {
	endpoint := os.Getenv("BTC_RPC_ENDPOINT")
	if endpoint == "" {
		t.Skip("BTC_RPC_ENDPOINT is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	client, err := btc.NewClient(endpoint)
	require.NoError(t, err)

	head, err := client.GetBlockCount(ctx)
	require.NoError(t, err)

	const n = 3
	s := New(client, Config{Chain: "main", FromHeight: head - n + 1, FullTransactions: true})

	blocks := make(chan *Block, n)
	s.OnBlock(func(b *Block) {
		blocks <- b
	})

	runCtx, stop := context.WithCancel(ctx)
	defer stop()

	go func() {
		_ = s.Run(runCtx)
	}()

	var prev *Block
	for i := 0; i < n; i++ {
		select {
		case b := <-blocks:
			t.Logf("block %d %s txs=%d", b.Height, b.Hash, len(b.Tx))

			assert.Equal(t, head-n+1+int64(i), b.Height)
			assert.Len(t, b.TxIDs, len(b.Tx))
			require.NotEmpty(t, b.Tx)
			assert.True(t, b.Tx[0].IsCoinbase())

			if prev != nil {
				require.NotNil(t, b.PreviousBlockHash)
				assert.Equal(t, prev.Hash, *b.PreviousBlockHash)
			}

			prev = b
		case <-ctx.Done():
			t.Fatal("timed out")
		}
	}
}
