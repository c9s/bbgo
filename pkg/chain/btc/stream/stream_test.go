package stream

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/chain/btc"
)

// fakeChain is an in-memory node. Every block ever produced stays readable
// by hash, as on a real node; active is the chain the node follows.
type fakeChain struct {
	mu     sync.Mutex
	chain  string
	blocks map[btc.Hash]*btc.Block
	active []btc.Hash

	// lagging makes the next calls of GetBlockHash fail as on a backend that
	// does not have the height yet
	lagging int
}

func newFakeChain(t *testing.T, length int) *fakeChain {
	c := &fakeChain{chain: "regtest", blocks: map[btc.Hash]*btc.Block{}}
	c.extend(length, 'a')
	return c
}

func blockHash(height int64, fork byte) btc.Hash {
	var h btc.Hash
	binary.BigEndian.PutUint64(h[:8], uint64(height))
	h[8] = fork
	return h
}

func txID(block btc.Hash, i int) btc.Hash {
	h := block
	h[31] = byte(i + 1)
	return h
}

// extend appends n blocks of the given fork to the active chain.
func (c *fakeChain) extend(n int, fork byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i := 0; i < n; i++ {
		height := int64(len(c.active))
		hash := blockHash(height, fork)

		block := &btc.Block{
			BlockHeader: btc.BlockHeader{Hash: hash, Height: height, NTx: 2},
			Tx: []btc.Tx{
				{TxID: txID(hash, 0), Vin: []btc.TxIn{{Coinbase: "03"}}},
				{TxID: txID(hash, 1)},
			},
		}

		if height > 0 {
			prev := c.active[height-1]
			block.PreviousBlockHash = &prev
		}

		c.blocks[hash] = block
		c.active = append(c.active, hash)
	}
}

// replace drops the active blocks from height on and appends n blocks of fork.
func (c *fakeChain) replace(height int64, n int, fork byte) {
	c.mu.Lock()
	c.active = c.active[:height]
	c.mu.Unlock()

	c.extend(n, fork)
}

func (c *fakeChain) GetBlockchainInfo(ctx context.Context) (*btc.BlockchainInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return &btc.BlockchainInfo{
		Chain:         c.chain,
		Blocks:        int64(len(c.active) - 1),
		BestBlockHash: c.active[len(c.active)-1],
	}, nil
}

func (c *fakeChain) GetBestBlockHash(ctx context.Context) (btc.Hash, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.active[len(c.active)-1], nil
}

func (c *fakeChain) GetBlockHeader(ctx context.Context, hash btc.Hash) (*btc.BlockHeader, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	block, ok := c.blocks[hash]
	if !ok {
		return nil, &btc.RPCError{Code: btc.ErrCodeInvalidAddressOrKey, Message: "Block not found"}
	}

	header := block.BlockHeader
	return &header, nil
}

func (c *fakeChain) GetBlockHash(ctx context.Context, height int64) (btc.Hash, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.lagging > 0 {
		c.lagging--
		return btc.Hash{}, &btc.RPCError{Code: btc.ErrCodeInvalidParameter, Message: "Block height out of range"}
	}

	if height < 0 || height >= int64(len(c.active)) {
		return btc.Hash{}, &btc.RPCError{Code: btc.ErrCodeInvalidParameter, Message: "Block height out of range"}
	}

	return c.active[height], nil
}

func (c *fakeChain) GetBlock(ctx context.Context, hash btc.Hash) (*btc.Block, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	block, ok := c.blocks[hash]
	if !ok {
		return nil, &btc.RPCError{Code: btc.ErrCodeInvalidAddressOrKey, Message: "Block not found"}
	}

	copied := *block
	return &copied, nil
}

func (c *fakeChain) GetBlockTxIDs(ctx context.Context, hash btc.Hash) (*btc.BlockHeader, []btc.Hash, error) {
	block, err := c.GetBlock(ctx, hash)
	if err != nil {
		return nil, nil, err
	}

	txids := make([]btc.Hash, len(block.Tx))
	for i := range block.Tx {
		txids[i] = block.Tx[i].TxID
	}

	return &block.BlockHeader, txids, nil
}

// recorder collects what the stream emits.
type recorder struct {
	events chan string
}

func record(s *Stream) *recorder {
	r := &recorder{events: make(chan string, 256)}

	s.OnBlock(func(b *Block) {
		r.events <- fmt.Sprintf("block %d%c", b.Height, b.Hash[8])
	})

	s.OnReorg(func(reorg Reorg) {
		removed := ""
		for _, ref := range reorg.Removed {
			removed += fmt.Sprintf(" %d%c", ref.Height, ref.Hash[8])
		}

		r.events <- fmt.Sprintf("reorg %d%c:%s", reorg.Ancestor.Height, reorg.Ancestor.Hash[8], removed)
	})

	return r
}

func (r *recorder) expect(t *testing.T, events ...string) {
	t.Helper()

	for _, want := range events {
		select {
		case got := <-r.events:
			require.Equal(t, want, got)
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for %q", want)
		}
	}
}

func (r *recorder) expectNothing(t *testing.T) {
	t.Helper()

	select {
	case got := <-r.events:
		t.Fatalf("unexpected event %q", got)
	case <-time.After(100 * time.Millisecond):
	}
}

func run(t *testing.T, chain *fakeChain, config Config) (*Stream, *recorder, <-chan error) {
	t.Helper()

	if config.PollInterval == 0 {
		config.PollInterval = 10 * time.Millisecond
	}

	s := New(chain, config)
	r := record(s)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	finished := make(chan struct{})
	go func() {
		done <- s.Run(ctx)
		close(finished)
	}()

	t.Cleanup(func() {
		cancel()
		<-finished
	})

	return s, r, done
}

func TestStreamFollowsHead(t *testing.T) {
	chain := newFakeChain(t, 11) // blocks 0..10
	s, r, _ := run(t, chain, Config{})

	r.expect(t, "block 10a")

	chain.extend(2, 'a')
	r.expect(t, "block 11a", "block 12a")
	r.expectNothing(t)

	cursor, ok := s.Cursor()
	require.True(t, ok)
	assert.Equal(t, int64(12), cursor.Height)
	assert.Equal(t, int64(12), s.Head().Height)
}

func TestStreamBackfillFromHeight(t *testing.T) {
	chain := newFakeChain(t, 11)
	_, r, _ := run(t, chain, Config{FromHeight: 7})

	r.expect(t, "block 7a", "block 8a", "block 9a", "block 10a")
	r.expectNothing(t)
}

func TestStreamConfirmations(t *testing.T) {
	chain := newFakeChain(t, 11)
	_, r, _ := run(t, chain, Config{Confirmations: 2})

	r.expect(t, "block 8a")
	r.expectNothing(t)

	chain.extend(1, 'a')
	r.expect(t, "block 9a")
	r.expectNothing(t)
}

func TestStreamTipReplacedAtSameHeight(t *testing.T) {
	chain := newFakeChain(t, 11)
	_, r, _ := run(t, chain, Config{})

	r.expect(t, "block 10a")

	// a competing block wins at the same height: the height does not move
	chain.replace(10, 1, 'b')
	r.expect(t, "reorg 9a: 10a", "block 10b")
	r.expectNothing(t)
}

func TestStreamDeeperReorg(t *testing.T) {
	chain := newFakeChain(t, 11)
	_, r, _ := run(t, chain, Config{FromHeight: 8})

	r.expect(t, "block 8a", "block 9a", "block 10a")

	chain.replace(8, 4, 'b') // 8b..11b
	r.expect(t, "reorg 7a: 10a 9a 8a", "block 8b", "block 9b", "block 10b", "block 11b")
	r.expectNothing(t)
}

func TestStreamReorgToShorterChain(t *testing.T) {
	chain := newFakeChain(t, 11)
	_, r, _ := run(t, chain, Config{FromHeight: 9})

	r.expect(t, "block 9a", "block 10a")

	// a chain with more work and fewer blocks: the cursor's height is gone
	chain.replace(9, 1, 'b')
	r.expectNothing(t)

	chain.extend(1, 'b')
	r.expect(t, "reorg 8a: 10a 9a", "block 9b", "block 10b")
}

func TestStreamReorgTooDeep(t *testing.T) {
	chain := newFakeChain(t, 11)
	_, r, done := run(t, chain, Config{FromHeight: 5, MaxReorgDepth: 3})

	r.expect(t, "block 5a", "block 6a", "block 7a", "block 8a", "block 9a", "block 10a")

	chain.replace(3, 9, 'b')

	select {
	case err := <-done:
		assert.ErrorIs(t, err, ErrReorgTooDeep)
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not stop")
	}
}

func TestStreamLaggingBackend(t *testing.T) {
	chain := newFakeChain(t, 11)
	_, r, _ := run(t, chain, Config{})

	r.expect(t, "block 10a")

	chain.mu.Lock()
	chain.lagging = 3
	chain.mu.Unlock()

	chain.extend(1, 'a')
	r.expect(t, "block 11a")
	r.expectNothing(t)
}

func TestStreamFullTransactions(t *testing.T) {
	chain := newFakeChain(t, 11)

	for _, full := range []bool{false, true} {
		t.Run(fmt.Sprintf("full=%v", full), func(t *testing.T) {
			s := New(chain, Config{FullTransactions: full, PollInterval: 10 * time.Millisecond})

			blocks := make(chan *Block, 1)
			s.OnBlock(func(b *Block) {
				blocks <- b
			})

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			go func() {
				_ = s.Run(ctx)
			}()

			select {
			case b := <-blocks:
				hash := blockHash(10, 'a')
				assert.Equal(t, []btc.Hash{txID(hash, 0), txID(hash, 1)}, b.TxIDs)
				require.NotNil(t, b.PreviousBlockHash)
				assert.Equal(t, blockHash(9, 'a'), *b.PreviousBlockHash)

				if full {
					require.Len(t, b.Tx, 2)
					assert.True(t, b.Tx[0].IsCoinbase())
				} else {
					assert.Empty(t, b.Tx)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("no block")
			}
		})
	}
}

func TestStreamWrongChain(t *testing.T) {
	chain := newFakeChain(t, 11)
	s := New(chain, Config{Chain: "main"})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := s.Run(ctx)
	require.Error(t, err)
	assert.False(t, errors.Is(err, context.DeadlineExceeded), "a wrong chain is not retried")
	assert.Contains(t, err.Error(), `"regtest"`)
}

func TestStreamAlreadyRunning(t *testing.T) {
	chain := newFakeChain(t, 11)
	s, r, _ := run(t, chain, Config{})
	r.expect(t, "block 10a")

	err := s.Run(context.Background())
	assert.ErrorContains(t, err, "already running")
}
