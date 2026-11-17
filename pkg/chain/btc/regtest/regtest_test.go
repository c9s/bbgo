package regtest_test

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/chain/btc"
	"github.com/c9s/bbgo/pkg/chain/btc/balance"
	"github.com/c9s/bbgo/pkg/chain/btc/regtest"
	"github.com/c9s/bbgo/pkg/chain/btc/stream"
	"github.com/c9s/bbgo/pkg/chain/btc/watch"
)

const coin btc.Amount = btc.SatoshiPerBitcoin

func testContext(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	return ctx
}

func newAddress(t *testing.T, ctx context.Context, n *regtest.Node) string {
	a, err := n.NewAddress(ctx)
	require.NoError(t, err)
	return a
}

func mine(t *testing.T, ctx context.Context, n *regtest.Node, count int, address string) []btc.Hash {
	hashes, err := n.Mine(ctx, count, address)
	require.NoError(t, err)
	require.Len(t, hashes, count)
	return hashes
}

// mempoolAccept builds and signs a transaction spending the outpoint and asks
// the node whether its mempool would accept it, without broadcasting it.
func mempoolAccept(t *testing.T, ctx context.Context, n *regtest.Node, in btc.Outpoint, to string, value btc.Amount) (bool, string) {
	var raw string
	require.NoError(t, n.Call(ctx, &raw, "createrawtransaction",
		[]map[string]any{{"txid": in.TxID.String(), "vout": in.Vout}},
		[]map[string]any{{to: value.String()}}))

	var signed struct {
		Hex      string `json:"hex"`
		Complete bool   `json:"complete"`
	}
	require.NoError(t, n.Call(ctx, &signed, "signrawtransactionwithwallet", raw))
	require.True(t, signed.Complete)

	var results []struct {
		Allowed      bool   `json:"allowed"`
		RejectReason string `json:"reject-reason"`
	}
	require.NoError(t, n.Call(ctx, &results, "testmempoolaccept", []string{signed.Hex}))
	require.Len(t, results, 1)
	return results[0].Allowed, results[0].RejectReason
}

type walletBalances struct {
	Mine struct {
		Trusted          btc.Amount `json:"trusted"`
		UntrustedPending btc.Amount `json:"untrusted_pending"`
		Immature         btc.Amount `json:"immature"`
	} `json:"mine"`
}

func getBalances(t *testing.T, ctx context.Context, n *regtest.Node) walletBalances {
	var b walletBalances
	require.NoError(t, n.Call(ctx, &b, "getbalances"))
	return b
}

// TestCoinbaseMaturity checks UTXO.Mature against consensus: the node's own
// mempool must reject a coinbase spend exactly when Mature is false.
func TestCoinbaseMaturity(t *testing.T) {
	t.Parallel()
	ctx := testContext(t)
	n := regtest.Start(t)

	miner := newAddress(t, ctx, n)
	other := newAddress(t, ctx, n)

	coinbaseBlock := mine(t, ctx, n, 1, miner)[0]
	mine(t, ctx, n, btc.CoinbaseMaturity-2, other) // tip 99: the coinbase has 99 confirmations

	source := &balance.CoreSource{Client: n.Client}

	snapshot, err := source.Snapshot(ctx, []string{miner})
	require.NoError(t, err)
	require.Equal(t, int64(99), snapshot.Height)
	require.Len(t, snapshot.UTXOs, 1)

	cb := snapshot.UTXOs[0]
	assert.True(t, cb.Coinbase)
	assert.Equal(t, int64(1), cb.Height)
	assert.Equal(t, coinbaseBlock, cb.BlockHash)
	assert.Equal(t, int64(99), cb.Confirmations(snapshot.Height))
	assert.False(t, cb.Mature(snapshot.Height))

	b := balance.Of(balance.Compute(snapshot, 1), miner)
	assert.Equal(t, 50*coin, b.Immature)
	assert.Equal(t, btc.Amount(0), b.Confirmed)

	allowed, reason := mempoolAccept(t, ctx, n, cb.Outpoint, other, 50*coin-10_000)
	assert.False(t, allowed)
	assert.Equal(t, "bad-txns-premature-spend-of-coinbase", reason)

	mine(t, ctx, n, 1, other) // tip 100: 100 confirmations, spendable in block 101

	snapshot, err = source.Snapshot(ctx, []string{miner})
	require.NoError(t, err)
	require.Len(t, snapshot.UTXOs, 1)
	assert.True(t, snapshot.UTXOs[0].Mature(snapshot.Height))

	b = balance.Of(balance.Compute(snapshot, 1), miner)
	assert.Equal(t, 50*coin, b.Confirmed)
	assert.Equal(t, btc.Amount(0), b.Immature)

	allowed, reason = mempoolAccept(t, ctx, n, cb.Outpoint, other, 50*coin-10_000)
	assert.True(t, allowed, reason)

	// The wallet waits one block more than consensus (COINBASE_MATURITY+1
	// confirmations) before counting a coinbase output as trusted, so it
	// still leaves it out of its spendable outputs.
	var unspent []json.RawMessage
	require.NoError(t, n.Call(ctx, &unspent, "listunspent", 0, 9999999, []string{miner}))
	assert.Empty(t, unspent)

	mine(t, ctx, n, 1, other)
	require.NoError(t, n.Call(ctx, &unspent, "listunspent", 0, 9999999, []string{miner}))
	assert.Len(t, unspent, 1)
}

// TestCoreSourceMatchesWallet scans the wallet's own descriptors and checks
// the computed balances against the wallet, and the block fees against the
// wallet's record of what it paid.
func TestCoreSourceMatchesWallet(t *testing.T) {
	t.Parallel()
	ctx := testContext(t)
	n := regtest.Start(t)

	miner := newAddress(t, ctx, n)
	alice := newAddress(t, ctx, n)
	bob := newAddress(t, ctx, n)

	mine(t, ctx, n, btc.CoinbaseMaturity+10, miner)

	txAlice, err := n.Send(ctx, alice, 150_000_000)
	require.NoError(t, err)

	txBob, err := n.Send(ctx, bob, 25_000_000)
	require.NoError(t, err)

	tip := mine(t, ctx, n, 1, miner)[0]

	// fees: the block's prevout-derived fee of each send matches the wallet's
	block, err := n.GetBlock(ctx, tip)
	require.NoError(t, err)

	fees := map[btc.Hash]btc.Amount{}
	for i := range block.Tx {
		if tx := &block.Tx[i]; tx.Fee != nil {
			fees[tx.TxID] = *tx.Fee
		}
	}

	for _, txid := range []btc.Hash{txAlice, txBob} {
		var wtx struct {
			Fee btc.Amount `json:"fee"`
		}
		require.NoError(t, n.Call(ctx, &wtx, "gettransaction", txid.String()))
		require.Contains(t, fees, txid)
		assert.Equal(t, -wtx.Fee, fees[txid], "fee of %s", txid)
	}

	// the wallet's active descriptors cover every address it owns
	var listed struct {
		Descriptors []struct {
			Desc   string `json:"desc"`
			Active bool   `json:"active"`
		} `json:"descriptors"`
	}
	require.NoError(t, n.Call(ctx, &listed, "listdescriptors"))

	var descriptors []string
	for _, d := range listed.Descriptors {
		if d.Active {
			descriptors = append(descriptors, d.Desc)
		}
	}

	source := &balance.CoreSource{Client: n.Client}
	snapshot, err := source.Snapshot(ctx, descriptors)
	require.NoError(t, err)

	balances := balance.Compute(snapshot, 1)
	total := balance.Sum(balances)
	wb := getBalances(t, ctx, n)

	assert.Equal(t, btc.Amount(0), total.Pending)
	assert.Equal(t, wb.Mine.UntrustedPending, btc.Amount(0))
	assert.Equal(t, wb.Mine.Trusted+wb.Mine.Immature, total.Confirmed+total.Immature)

	// the wallet counts one more block of coinbase as immature (see
	// TestCoinbaseMaturity)
	assert.Equal(t, wb.Mine.Immature-50*coin, total.Immature)

	// scantxoutset reports no address for a ranged descriptor's match, only
	// the derived key in desc, so those balances are keyed by script
	aliceBalance := balance.Of(balances, alice)
	assert.Equal(t, btc.Amount(0), aliceBalance.Total())

	// plain addresses are scanned as addr() descriptors and keyed by address
	snapshot, err = source.Snapshot(ctx, []string{alice, bob})
	require.NoError(t, err)

	balances = balance.Compute(snapshot, 1)
	assert.Equal(t, btc.Amount(150_000_000), balance.Of(balances, alice).Confirmed)
	assert.Equal(t, btc.Amount(25_000_000), balance.Of(balances, bob).Confirmed)
}

type recorder struct {
	mu      sync.Mutex
	events  []watch.Event
	reverts []watch.Event
	reorgs  []stream.Reorg
}

func (r *recorder) counts() (int, int, int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.events), len(r.reverts), len(r.reorgs)
}

func outpointSet(utxos []btc.UTXO) map[btc.Outpoint]btc.Amount {
	m := map[btc.Outpoint]btc.Amount{}
	for _, u := range utxos {
		m[u.Outpoint] = u.Value
	}
	return m
}

// TestWatcherFollowsReorgs seeds a watcher from scantxoutset, follows the
// stream through a receive, a spend of the seeded output and a same-height
// reorg, and checks it ends on the same UTXO set as a fresh scan.
func TestWatcherFollowsReorgs(t *testing.T) {
	t.Parallel()
	ctx := testContext(t)
	n := regtest.Start(t)

	miner := newAddress(t, ctx, n)
	watched := newAddress(t, ctx, n)
	other := newAddress(t, ctx, n)

	mine(t, ctx, n, btc.CoinbaseMaturity+10, miner)

	seededTx, err := n.Send(ctx, watched, 2*coin)
	require.NoError(t, err)
	mine(t, ctx, n, 1, miner)

	// seed from the UTXO set, then stream from the block after it
	source := &balance.CoreSource{Client: n.Client}
	seed, err := source.Snapshot(ctx, []string{watched})
	require.NoError(t, err)
	require.Len(t, seed.UTXOs, 1)
	require.Equal(t, seededTx, seed.UTXOs[0].Outpoint.TxID)

	w := watch.New(watch.Config{Addresses: []string{watched}})
	w.Seed(seed.UTXOs)

	s := stream.New(n.Client, stream.Config{
		Chain:            "regtest",
		FromHeight:       seed.Height + 1,
		FullTransactions: true,
		PollInterval:     250 * time.Millisecond,
	})
	require.NoError(t, w.Attach(s))

	rec := &recorder{}
	w.OnEvent(func(e watch.Event) {
		rec.mu.Lock()
		rec.events = append(rec.events, e)
		rec.mu.Unlock()
	})
	w.OnRevert(func(e watch.Event) {
		rec.mu.Lock()
		rec.reverts = append(rec.reverts, e)
		rec.mu.Unlock()
	})
	s.OnReorg(func(r stream.Reorg) {
		rec.mu.Lock()
		rec.reorgs = append(rec.reorgs, r)
		rec.mu.Unlock()
	})

	runCtx, stop := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- s.Run(runCtx) }()
	t.Cleanup(func() {
		stop()
		<-done
	})

	waitTip := func(hash btc.Hash) {
		require.Eventually(t, func() bool {
			tip, ok := w.Tip()
			return ok && tip.Hash == hash
		}, 15*time.Second, 50*time.Millisecond, "watcher did not reach %s", hash)
	}

	// a spend of the seeded output and a receive in one block; spend first,
	// or the wallet may pick the seeded output to fund the receive
	spendTx, err := n.SpendAll(ctx, other, seed.UTXOs[0].Outpoint)
	require.NoError(t, err)

	receiveTx, err := n.Send(ctx, watched, coin)
	require.NoError(t, err)

	first := mine(t, ctx, n, 1, miner)[0]
	waitTip(first)

	rec.mu.Lock()
	require.Len(t, rec.events, 2)
	byKind := map[watch.Kind]watch.Event{}
	for _, e := range rec.events {
		byKind[e.Kind] = e
	}
	rec.mu.Unlock()

	assert.Equal(t, receiveTx, byKind[watch.Received].TxID)
	assert.Equal(t, coin, byKind[watch.Received].Value)
	assert.Equal(t, spendTx, byKind[watch.Spent].TxID)
	assert.Equal(t, seed.UTXOs[0].Outpoint, byKind[watch.Spent].Outpoint)
	assert.True(t, byKind[watch.Spent].Known, "the seeded output is tracked")

	// same-height reorg: the block is replaced by a competing one at the
	// same height, which mines the same transactions again from the mempool
	require.NoError(t, n.InvalidateBlock(ctx, first))
	replacement := mine(t, ctx, n, 1, other)[0]
	require.NotEqual(t, first, replacement)
	waitTip(replacement)

	events, reverts, reorgs := rec.counts()
	assert.Equal(t, 1, reorgs)
	assert.Equal(t, 2, reverts)
	assert.Equal(t, 4, events, "the replayed block emits both events again")

	rec.mu.Lock()
	assert.Equal(t, []stream.BlockRef{{Height: seed.Height + 1, Hash: first}}, rec.reorgs[0].Removed)
	rec.mu.Unlock()

	// a deeper reorg: two blocks replaced by three
	mine(t, ctx, n, 1, miner)
	tip := mine(t, ctx, n, 1, miner)[0]
	waitTip(tip)

	height, err := n.GetBlockCount(ctx)
	require.NoError(t, err)

	fork, err := n.GetBlockHash(ctx, height-1)
	require.NoError(t, err)
	require.NoError(t, n.InvalidateBlock(ctx, fork))

	tip = mine(t, ctx, n, 3, other)[2]
	waitTip(tip)

	_, _, reorgs = rec.counts()
	assert.Equal(t, 2, reorgs)

	rec.mu.Lock()
	assert.Len(t, rec.reorgs[1].Removed, 2)
	rec.mu.Unlock()

	// the watcher ends on the same UTXOs as a fresh scan at the same tip
	scan, err := source.Snapshot(ctx, []string{watched})
	require.NoError(t, err)

	seen := w.Snapshot()
	assert.Equal(t, scan.Height, seen.Height)
	assert.Equal(t, outpointSet(scan.UTXOs), outpointSet(seen.UTXOs))

	got := balance.Compute(seen, 1)
	want := balance.Compute(scan, 1)
	sort.Slice(got, func(i, j int) bool { return got[i].Address < got[j].Address })
	sort.Slice(want, func(i, j int) bool { return want[i].Address < want[j].Address })
	assert.Equal(t, want, got)
}
