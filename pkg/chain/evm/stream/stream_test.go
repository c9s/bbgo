package stream

import (
	"context"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/chain/evm"
)

var testToken = common.HexToAddress("0xdAC17F958D2ee523a2206206994597C13D831ec7")

// fakeChain is an in-memory canonical chain that can be extended and reorged.
type fakeChain struct {
	mu sync.Mutex

	blocks []*evm.Block
	logs   map[common.Hash][]ethtypes.Log

	// lag hides the newest blocks from BlockByNumber while BlockNumber still
	// reports them, like an endpoint behind the one that announced the head
	lag uint64

	// indexLag simulates a load-balanced log backend behind the block
	// backend: a query by hash answers nothing, a query by number fails
	indexLag bool

	logQueries []common.Hash
}

func newFakeChain(n int) *fakeChain {
	f := &fakeChain{logs: map[common.Hash][]ethtypes.Log{}}
	f.extend(n, 0)
	return f
}

// extend appends n blocks with one token log each; fork tags them so a
// reorged block hashes differently from the one it replaces.
func (f *fakeChain) extend(n int, fork byte) {
	f.extendBlocks(n, fork, true)
}

// extendWithoutLogs appends blocks whose bloom matches the token although
// they have no log: a bloom false positive.
func (f *fakeChain) extendWithoutLogs(n int) {
	f.extendBlocks(n, 0, false)
}

func (f *fakeChain) extendBlocks(n int, fork byte, withLogs bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var bloom ethtypes.Bloom
	bloom.Add(testToken.Bytes())

	for i := 0; i < n; i++ {
		number := uint64(len(f.blocks))

		var parent common.Hash
		if number > 0 {
			parent = f.blocks[number-1].Hash
		}

		header := &ethtypes.Header{
			ParentHash: parent,
			Number:     new(big.Int).SetUint64(number),
			Difficulty: big.NewInt(0),
			Extra:      []byte{fork},
			Bloom:      bloom,
		}

		block := &evm.Block{Hash: header.Hash(), Header: header, TxHashes: []common.Hash{}}
		f.blocks = append(f.blocks, block)

		if withLogs {
			f.logs[block.Hash] = []ethtypes.Log{{
				Address:     testToken,
				BlockNumber: number,
				BlockHash:   block.Hash,
			}}
		}
	}
}

func (f *fakeChain) setIndexLag(lag bool) {
	f.mu.Lock()
	f.indexLag = lag
	f.mu.Unlock()
}

// reorg replaces every block from number on with n blocks of another fork.
func (f *fakeChain) reorg(number uint64, n int, fork byte) {
	f.mu.Lock()
	f.blocks = f.blocks[:number]
	f.mu.Unlock()

	f.extend(n, fork)
}

func (f *fakeChain) setLag(lag uint64) {
	f.mu.Lock()
	f.lag = lag
	f.mu.Unlock()
}

func (f *fakeChain) block(number uint64) *evm.Block {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.blocks[number]
}

func (f *fakeChain) head() uint64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return uint64(len(f.blocks) - 1)
}

func (f *fakeChain) Chain() *evm.Chain {
	chain, _ := evm.LookupChain("ethereum")
	return chain
}

func (f *fakeChain) BlockNumber(ctx context.Context) (uint64, error) {
	return f.head(), nil
}

func (f *fakeChain) BlockByNumber(ctx context.Context, number *big.Int, fullTx bool) (*evm.Block, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	n := number.Uint64()
	if n >= uint64(len(f.blocks))-f.lag {
		return nil, ethereum.NotFound
	}

	return f.blocks[n], nil
}

func (f *fakeChain) FilterLogs(ctx context.Context, query ethereum.FilterQuery) ([]ethtypes.Log, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if query.BlockHash != nil {
		f.logQueries = append(f.logQueries, *query.BlockHash)
		if f.indexLag {
			return []ethtypes.Log{}, nil
		}

		return append([]ethtypes.Log(nil), f.logs[*query.BlockHash]...), nil
	}

	// the stream only falls back to a single-block range query
	if query.FromBlock == nil || query.ToBlock == nil || query.FromBlock.Cmp(query.ToBlock) != 0 {
		return nil, errors.New("unexpected log query")
	}

	if f.indexLag {
		return nil, errors.New("block range extends beyond current head block")
	}

	n := query.FromBlock.Uint64()
	if n >= uint64(len(f.blocks)) {
		return nil, errors.New("block range extends beyond current head block")
	}

	return append([]ethtypes.Log(nil), f.logs[f.blocks[n].Hash]...), nil
}

// recorder collects the stream events.
type recorder struct {
	blocks chan *Block
	logs   chan ethtypes.Log
	reorgs chan Reorg
}

func record(s *Stream) *recorder {
	r := &recorder{
		blocks: make(chan *Block, 100),
		logs:   make(chan ethtypes.Log, 100),
		reorgs: make(chan Reorg, 10),
	}

	s.OnBlock(func(b *Block) { r.blocks <- b })
	s.OnLog(func(l ethtypes.Log) { r.logs <- l })
	s.OnReorg(func(reorg Reorg) { r.reorgs <- reorg })
	return r
}

func (r *recorder) nextBlocks(t *testing.T, n int) []*Block {
	t.Helper()

	out := make([]*Block, 0, n)
	for len(out) < n {
		select {
		case b := <-r.blocks:
			out = append(out, b)
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out after %d of %d blocks", len(out), n)
		}
	}

	return out
}

func (r *recorder) nextReorg(t *testing.T) Reorg {
	t.Helper()

	select {
	case reorg := <-r.reorgs:
		return reorg
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for a reorg")
	}

	return Reorg{}
}

func (r *recorder) assertQuiet(t *testing.T) {
	t.Helper()

	select {
	case b := <-r.blocks:
		t.Fatalf("unexpected block %d", b.Number())
	case <-time.After(300 * time.Millisecond):
	}
}

func numbers(blocks []*Block) []uint64 {
	out := make([]uint64, 0, len(blocks))
	for _, b := range blocks {
		out = append(out, b.Number())
	}

	return out
}

// run starts the stream and returns a stop function that cancels it and
// returns the error Run ended with.
func run(t *testing.T, s *Stream) func() error {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()

	stopped := false
	var result error
	stop := func() error {
		if stopped {
			return result
		}

		stopped = true
		cancel()

		select {
		case result = <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("the stream did not stop")
		}

		return result
	}

	t.Cleanup(func() { _ = stop() })
	return stop
}

func TestStream_BackfillsFromBlockWithLogs(t *testing.T) {
	chain := newFakeChain(10)

	s := New(chain, Config{
		FromBlock:    3,
		PollInterval: time.Hour,
		Logs:         &LogFilter{Addresses: []common.Address{testToken}},
	})
	r := record(s)
	stop := run(t, s)

	blocks := r.nextBlocks(t, 7)
	assert.Equal(t, []uint64{3, 4, 5, 6, 7, 8, 9}, numbers(blocks))

	for _, b := range blocks {
		require.Len(t, b.Logs, 1)
		assert.Equal(t, b.Hash, b.Logs[0].BlockHash)
		assert.Equal(t, chain.block(b.Number()).Hash, b.Hash)
	}

	assert.Len(t, r.logs, 7)

	cursor, ok := s.Cursor()
	require.True(t, ok)
	assert.Equal(t, uint64(9), cursor.Number)

	assert.ErrorIs(t, stop(), context.Canceled)
}

func TestStream_FollowsHeadByPolling(t *testing.T) {
	chain := newFakeChain(5)

	s := New(chain, Config{PollInterval: 10 * time.Millisecond})
	r := record(s)
	run(t, s)

	// without FromBlock the stream starts at the head
	assert.Equal(t, []uint64{4}, numbers(r.nextBlocks(t, 1)))

	chain.extend(3, 0)
	assert.Equal(t, []uint64{5, 6, 7}, numbers(r.nextBlocks(t, 3)))
	assert.False(t, s.Connected())
}

func TestStream_StaysBehindByConfirmations(t *testing.T) {
	chain := newFakeChain(10)

	s := New(chain, Config{Confirmations: 2, PollInterval: 10 * time.Millisecond})
	r := record(s)
	run(t, s)

	assert.Equal(t, []uint64{7}, numbers(r.nextBlocks(t, 1)))
	r.assertQuiet(t)

	chain.extend(1, 0)
	assert.Equal(t, []uint64{8}, numbers(r.nextBlocks(t, 1)))
}

func TestStream_Reorg(t *testing.T) {
	chain := newFakeChain(6)
	old4, old5 := chain.block(4).Hash, chain.block(5).Hash
	ancestor := chain.block(3).Hash

	s := New(chain, Config{
		FromBlock:    1,
		PollInterval: 10 * time.Millisecond,
		Logs:         &LogFilter{},
	})
	r := record(s)
	run(t, s)

	assert.Equal(t, []uint64{1, 2, 3, 4, 5}, numbers(r.nextBlocks(t, 5)))

	chain.reorg(4, 3, 1)

	reorg := r.nextReorg(t)
	assert.Equal(t, BlockRef{Number: 3, Hash: ancestor}, reorg.Ancestor)
	assert.Equal(t, []BlockRef{{Number: 5, Hash: old5}, {Number: 4, Hash: old4}}, reorg.Removed)

	blocks := r.nextBlocks(t, 3)
	assert.Equal(t, []uint64{4, 5, 6}, numbers(blocks))
	assert.Equal(t, chain.block(4).Hash, blocks[0].Hash)
	assert.NotEqual(t, old4, blocks[0].Hash)
	assert.Equal(t, blocks[0].Hash, blocks[0].Logs[0].BlockHash)
}

func TestStream_ReorgDeeperThanWindowStops(t *testing.T) {
	chain := newFakeChain(6)

	s := New(chain, Config{
		FromBlock:     1,
		PollInterval:  10 * time.Millisecond,
		MaxReorgDepth: 3,
	})
	r := record(s)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()

	r.nextBlocks(t, 5)
	chain.reorg(2, 5, 1)

	select {
	case err := <-done:
		assert.ErrorIs(t, err, ErrReorgTooDeep)
	case <-ctx.Done():
		t.Fatal("the stream did not stop on a deep reorg")
	}
}

func TestStream_WaitsForLaggingEndpoint(t *testing.T) {
	chain := newFakeChain(5)

	s := New(chain, Config{PollInterval: 10 * time.Millisecond})
	r := record(s)
	run(t, s)

	r.nextBlocks(t, 1)

	// the head moves to 6 but the blocks are not readable yet
	chain.setLag(2)
	chain.extend(2, 0)
	r.assertQuiet(t)

	chain.setLag(0)
	assert.Equal(t, []uint64{5, 6}, numbers(r.nextBlocks(t, 2)))
}

func TestStream_WaitsForLaggingLogIndex(t *testing.T) {
	chain := newFakeChain(5)

	s := New(chain, Config{
		PollInterval: 10 * time.Millisecond,
		Logs:         &LogFilter{Addresses: []common.Address{testToken}},
	})
	r := record(s)
	run(t, s)

	require.Len(t, r.nextBlocks(t, 1)[0].Logs, 1)

	// an empty answer by hash must not be delivered as "no logs"
	chain.setIndexLag(true)
	chain.extend(1, 0)
	r.assertQuiet(t)

	chain.setIndexLag(false)
	blocks := r.nextBlocks(t, 1)
	assert.Equal(t, []uint64{5}, numbers(blocks))
	require.Len(t, blocks[0].Logs, 1)
	assert.Equal(t, blocks[0].Hash, blocks[0].Logs[0].BlockHash)
}

func TestStream_BloomFalsePositiveDoesNotStall(t *testing.T) {
	chain := newFakeChain(5)

	s := New(chain, Config{
		PollInterval: 10 * time.Millisecond,
		Logs:         &LogFilter{Addresses: []common.Address{testToken}},
	})
	r := record(s)
	run(t, s)

	r.nextBlocks(t, 1)

	chain.extendWithoutLogs(2)
	blocks := r.nextBlocks(t, 2)
	assert.Equal(t, []uint64{5, 6}, numbers(blocks))
	assert.Empty(t, blocks[0].Logs)
	assert.Empty(t, blocks[1].Logs)
}

func TestBloomMayMatch(t *testing.T) {
	other := common.HexToAddress("0x55d398326f99059fF775485246999027B3197955")
	transfer := common.HexToHash("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef")
	approval := common.HexToHash("0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925")

	var bloom ethtypes.Bloom
	bloom.Add(testToken.Bytes())
	bloom.Add(transfer.Bytes())

	assert.False(t, bloomMayMatch(ethtypes.Bloom{}, &LogFilter{}), "an empty bloom has no logs")
	assert.True(t, bloomMayMatch(bloom, &LogFilter{}))
	assert.True(t, bloomMayMatch(bloom, &LogFilter{Addresses: []common.Address{other, testToken}}))
	assert.False(t, bloomMayMatch(bloom, &LogFilter{Addresses: []common.Address{other}}))
	assert.True(t, bloomMayMatch(bloom, &LogFilter{Topics: [][]common.Hash{{approval, transfer}}}))
	assert.False(t, bloomMayMatch(bloom, &LogFilter{Topics: [][]common.Hash{{approval}}}))
	assert.True(t, bloomMayMatch(bloom, &LogFilter{Topics: [][]common.Hash{{transfer}, {}}}))
}

// ethService is the eth namespace of a fake node: eth_chainId and the newHeads
// subscription.
type ethService struct {
	chainID uint64

	mu   sync.Mutex
	subs []headSub
}

type headSub struct {
	notifier *rpc.Notifier
	id       rpc.ID
}

func (s *ethService) ChainId() hexutil.Uint64 {
	return hexutil.Uint64(s.chainID)
}

func (s *ethService) NewHeads(ctx context.Context) (*rpc.Subscription, error) {
	notifier, ok := rpc.NotifierFromContext(ctx)
	if !ok {
		return nil, rpc.ErrNotificationsUnsupported
	}

	sub := notifier.CreateSubscription()

	s.mu.Lock()
	s.subs = append(s.subs, headSub{notifier: notifier, id: sub.ID})
	s.mu.Unlock()

	return sub, nil
}

func (s *ethService) publish(number uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, sub := range s.subs {
		// subscriptions of closed connections just fail
		_ = sub.notifier.Notify(sub.id, map[string]any{"number": hexutil.Uint64(number)})
	}
}

func (s *ethService) subscribers() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.subs)
}

// fakeNode serves the eth service over WebSocket and can drop every
// connection by swapping the server.
type fakeNode struct {
	service *ethService
	server  atomic.Pointer[rpc.Server]
	http    *httptest.Server
}

func newFakeNode(t *testing.T, chainID uint64) *fakeNode {
	t.Helper()

	node := &fakeNode{service: &ethService{chainID: chainID}}
	node.server.Store(node.newServer(t))
	node.http = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		node.server.Load().WebsocketHandler([]string{"*"}).ServeHTTP(w, r)
	}))

	t.Cleanup(func() {
		node.http.Close()
		node.server.Load().Stop()
	})

	return node
}

func (n *fakeNode) newServer(t *testing.T) *rpc.Server {
	server := rpc.NewServer()
	require.NoError(t, server.RegisterName("eth", n.service))
	return server
}

func (n *fakeNode) url() string {
	return "ws" + strings.TrimPrefix(n.http.URL, "http")
}

// dropConnections closes every live connection; new ones are accepted.
func (n *fakeNode) dropConnections(t *testing.T) {
	old := n.server.Swap(n.newServer(t))
	old.Stop()
}

func TestStream_WebSocketHeadsAndReconnect(t *testing.T) {
	chain := newFakeChain(5)
	node := newFakeNode(t, 1)

	s := New(chain, Config{
		WSEndpoint: node.url(),
		// only the subscription can drive the stream
		PollInterval: time.Hour,
	})
	r := record(s)

	connects := make(chan struct{}, 10)
	disconnects := make(chan error, 10)
	s.OnConnect(func() { connects <- struct{}{} })
	s.OnDisconnect(func(err error) { disconnects <- err })

	run(t, s)

	waitConnect := func() {
		t.Helper()
		select {
		case <-connects:
		case <-time.After(5 * time.Second):
			t.Fatal("the stream did not connect")
		}
	}

	waitConnect()
	assert.True(t, s.Connected())
	assert.Equal(t, []uint64{4}, numbers(r.nextBlocks(t, 1)))

	chain.extend(2, 0)
	r.assertQuiet(t)

	node.service.publish(chain.head())
	assert.Equal(t, []uint64{5, 6}, numbers(r.nextBlocks(t, 2)))

	// blocks produced while disconnected are backfilled after the reconnect
	node.dropConnections(t)
	select {
	case err := <-disconnects:
		assert.Error(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("the disconnect was not reported")
	}

	chain.extend(3, 0)
	waitConnect()

	require.Eventually(t, func() bool { return node.service.subscribers() >= 2 }, 5*time.Second, 10*time.Millisecond)
	node.service.publish(chain.head())
	assert.Equal(t, []uint64{7, 8, 9}, numbers(r.nextBlocks(t, 3)))
}

func TestStream_WebSocketWrongChainStops(t *testing.T) {
	chain := newFakeChain(5)
	node := newFakeNode(t, 56)

	s := New(chain, Config{WSEndpoint: node.url(), PollInterval: time.Hour})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := s.Run(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chain id 56")
}

func TestStream_RunTwice(t *testing.T) {
	s := New(newFakeChain(2), Config{PollInterval: time.Hour})
	run(t, s)

	require.Eventually(t, func() bool {
		_, ok := s.Cursor()
		return ok
	}, 5*time.Second, 10*time.Millisecond)

	assert.Error(t, s.Run(context.Background()))
}
