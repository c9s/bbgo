package watch

import (
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/chain/btc"
	"github.com/c9s/bbgo/pkg/chain/btc/balance"
	"github.com/c9s/bbgo/pkg/chain/btc/stream"
)

const (
	alice = "bc1qyz5ts3dl4wxntavwfkugsj25zaaj8hcwmjkl6h"
	bob   = "1AfCc4F9c4VTYSE31PUe2kUEKs6ZxiDjxm"
	carol = "bc1qh2aq97wafs7px4jnajnpleu0njaqmque8dku3p"

	multisigScript = "5121aa21bb52ae"
)

func hashOf(kind byte, n int64) btc.Hash {
	var h btc.Hash
	h[0] = kind
	binary.BigEndian.PutUint64(h[1:9], uint64(n))
	return h
}

func spk(address string) btc.ScriptPubKey {
	return btc.ScriptPubKey{Address: address, Hex: "00" + address, Type: btc.ScriptTypeWitnessV0KeyHash}
}

func out(n uint32, address string, sats btc.Amount) btc.TxOut {
	return btc.TxOut{N: n, Value: sats, ScriptPubKey: spk(address)}
}

// in spends txid:vout; prevAddress, when set, fills the prevout as verbosity=3 does.
func in(txid btc.Hash, vout uint32, prevAddress string, sats btc.Amount) btc.TxIn {
	input := btc.TxIn{TxID: txid, Vout: vout}
	if prevAddress != "" {
		input.Prevout = &btc.Prevout{Height: 1, Value: sats, ScriptPubKey: spk(prevAddress)}
	}

	return input
}

func tx(id int64, ins []btc.TxIn, outs ...btc.TxOut) btc.Tx {
	return btc.Tx{TxID: hashOf('t', id), Vin: ins, Vout: outs}
}

func coinbase(id int64, outs ...btc.TxOut) btc.Tx {
	return btc.Tx{TxID: hashOf('t', id), Vin: []btc.TxIn{{Coinbase: "03"}}, Vout: outs}
}

func block(height int64, fork byte, txs ...btc.Tx) *stream.Block {
	return &stream.Block{Block: &btc.Block{
		BlockHeader: btc.BlockHeader{Hash: hashOf(fork, height), Height: height, NTx: int64(len(txs))},
		Tx:          txs,
	}}
}

type recorded struct {
	events   []Event
	reverted []Event
}

func record(w *Watcher) *recorded {
	r := &recorded{}
	w.OnEvent(func(e Event) { r.events = append(r.events, e) })
	w.OnRevert(func(e Event) { r.reverted = append(r.reverted, e) })
	return r
}

func kinds(events []Event) []string {
	var out []string
	for _, e := range events {
		out = append(out, fmt.Sprintf("%s %d", e.Kind, e.Outpoint.Vout))
	}

	return out
}

func TestReceiveAndSpend(t *testing.T) {
	w := New(Config{Addresses: []string{alice}})
	r := record(w)

	// block 1: a coinbase pays bob, tx 2 pays alice 1 BTC with change to carol
	require.NoError(t, w.ProcessBlock(block(1, 'a',
		coinbase(1, out(0, bob, 312500000)),
		tx(2, []btc.TxIn{in(hashOf('x', 1), 0, carol, 150000000)},
			out(0, alice, 100000000), out(1, carol, 49990000)),
	)))

	require.Len(t, r.events, 1)
	e := r.events[0]
	assert.Equal(t, Received, e.Kind)
	assert.Equal(t, btc.Outpoint{TxID: hashOf('t', 2), Vout: 0}, e.Outpoint)
	assert.Equal(t, btc.Amount(100000000), e.Value)
	assert.Equal(t, alice, e.Address)
	assert.Equal(t, hashOf('t', 2), e.TxID)
	assert.Equal(t, stream.BlockRef{Height: 1, Hash: hashOf('a', 1)}, e.Block)
	assert.True(t, e.Known)
	assert.Equal(t, map[string]btc.Amount{alice: 100000000}, w.Balances())

	// block 2: alice spends it to bob
	require.NoError(t, w.ProcessBlock(block(2, 'a',
		coinbase(3, out(0, bob, 312500000)),
		tx(4, []btc.TxIn{in(hashOf('t', 2), 0, alice, 100000000)}, out(0, bob, 99990000)),
	)))

	require.Len(t, r.events, 2)
	e = r.events[1]
	assert.Equal(t, Spent, e.Kind)
	assert.Equal(t, hashOf('t', 4), e.TxID, "the spending transaction")
	assert.Equal(t, int64(1), e.Height, "where the output was created")
	assert.Equal(t, int64(2), e.Block.Height, "where it was spent")
	assert.True(t, e.Known)
	assert.Empty(t, w.UTXOs())

	tip, ok := w.Tip()
	require.True(t, ok)
	assert.Equal(t, int64(2), tip.Height)
}

func TestCreatedAndSpentInOneBlock(t *testing.T) {
	w := New(Config{Addresses: []string{alice}})
	r := record(w)

	require.NoError(t, w.ProcessBlock(block(1, 'a',
		coinbase(1, out(0, bob, 1)),
		tx(2, []btc.TxIn{in(hashOf('x', 1), 0, "", 0)}, out(0, alice, 5000)),
		tx(3, []btc.TxIn{in(hashOf('t', 2), 0, "", 0)}, out(0, alice, 4000)),
	)))

	assert.Equal(t, []string{"received 0", "spent 0", "received 0"}, kinds(r.events))
	assert.True(t, r.events[1].Known, "spent by outpoint even without a prevout")

	utxos := w.UTXOs()
	require.Len(t, utxos, 1)
	assert.Equal(t, hashOf('t', 3), utxos[0].Outpoint.TxID)
}

func TestSpendOfUnknownOutput(t *testing.T) {
	w := New(Config{Addresses: []string{alice}})
	r := record(w)

	// alice spends an output received before the stream started
	require.NoError(t, w.ProcessBlock(block(5, 'a',
		coinbase(1, out(0, bob, 1)),
		tx(2, []btc.TxIn{in(hashOf('x', 9), 3, alice, 777)}, out(0, bob, 700)),
	)))

	require.Len(t, r.events, 1)
	e := r.events[0]
	assert.Equal(t, Spent, e.Kind)
	assert.False(t, e.Known)
	assert.Equal(t, btc.Amount(777), e.Value)
	assert.Equal(t, btc.Outpoint{TxID: hashOf('x', 9), Vout: 3}, e.Outpoint)
	assert.Empty(t, w.Balances())
}

func TestSeed(t *testing.T) {
	w := New(Config{Addresses: []string{alice}})
	r := record(w)

	seeded := btc.UTXO{Outpoint: btc.Outpoint{TxID: hashOf('x', 9), Vout: 3}, Address: alice, Value: 777, Height: 4}
	w.Seed([]btc.UTXO{seeded})
	assert.Equal(t, map[string]btc.Amount{alice: 777}, w.Balances())

	require.NoError(t, w.ProcessBlock(block(5, 'a',
		coinbase(1, out(0, bob, 1)),
		tx(2, []btc.TxIn{in(hashOf('x', 9), 3, alice, 777)}, out(0, bob, 700)),
	)))

	require.Len(t, r.events, 1)
	assert.True(t, r.events[0].Known)
	assert.Equal(t, int64(4), r.events[0].Height)
	assert.Empty(t, w.Balances())
}

func TestRevert(t *testing.T) {
	w := New(Config{Addresses: []string{alice}})
	r := record(w)

	b1 := block(1, 'a', coinbase(1, out(0, bob, 1)))
	b2 := block(2, 'a', coinbase(2, out(0, bob, 1)),
		tx(3, []btc.TxIn{in(hashOf('x', 1), 0, "", 0)}, out(0, alice, 5000), out(1, alice, 6000)))
	b3 := block(3, 'a', coinbase(4, out(0, bob, 1)),
		tx(5, []btc.TxIn{in(hashOf('t', 3), 0, alice, 5000)}, out(0, bob, 4900)))

	for _, b := range []*stream.Block{b1, b2, b3} {
		require.NoError(t, w.ProcessBlock(b))
	}

	assert.Equal(t, map[string]btc.Amount{alice: 6000}, w.Balances())

	// block 3 is replaced: the spend is undone
	w.Revert(stream.Reorg{Ancestor: b2.Ref(), Removed: []stream.BlockRef{b3.Ref()}})
	assert.Equal(t, []string{"spent 0"}, kinds(r.reverted))
	assert.Equal(t, map[string]btc.Amount{alice: 11000}, w.Balances())

	tip, _ := w.Tip()
	assert.Equal(t, b2.Ref(), tip)

	// the new block 3 spends nothing; then blocks 3b and 2 are replaced too
	b3b := block(3, 'b', coinbase(6, out(0, bob, 1)))
	require.NoError(t, w.ProcessBlock(b3b))

	r.reverted = nil
	w.Revert(stream.Reorg{Ancestor: b1.Ref(), Removed: []stream.BlockRef{b3b.Ref(), b2.Ref()}})
	assert.Equal(t, []string{"received 1", "received 0"}, kinds(r.reverted), "newest first")
	assert.Empty(t, w.Balances())

	// a removed block the watcher never saw is skipped
	w.Revert(stream.Reorg{Ancestor: stream.BlockRef{Height: 0}, Removed: []stream.BlockRef{{Height: 1, Hash: hashOf('z', 1)}}})
	assert.Empty(t, w.Balances())
}

func TestRevertKeepsReorgDepth(t *testing.T) {
	w := New(Config{Addresses: []string{alice}, MaxReorgDepth: 2})

	for h := int64(1); h <= 4; h++ {
		require.NoError(t, w.ProcessBlock(block(h, 'a', coinbase(h, out(0, alice, btc.Amount(h))))))
	}

	assert.Len(t, w.journal, 2)
	assert.Equal(t, int64(3), w.journal[0].ref.Height)
}

func TestMatching(t *testing.T) {
	// a bech32 address is case-insensitive; base58 is not
	w := New(Config{Addresses: []string{"BC1QYZ5TS3DL4WXNTAVWFKUGSJ25ZAAJ8HCWMJKL6H", bob}, Scripts: []string{"5121AA21BB52AE"}})
	r := record(w)

	multisig := btc.TxOut{N: 2, Value: 3, ScriptPubKey: btc.ScriptPubKey{Hex: multisigScript, Type: btc.ScriptTypeMultisig}}
	require.NoError(t, w.ProcessBlock(block(1, 'a',
		coinbase(1, out(0, alice, 1), out(1, bob, 2), multisig, out(3, carol, 4)),
	)))

	require.Len(t, r.events, 3)
	assert.Equal(t, map[string]btc.Amount{alice: 1, bob: 2, multisigScript: 3}, w.Balances())
}

func TestCoinbaseMaturity(t *testing.T) {
	w := New(Config{Addresses: []string{alice}})
	require.NoError(t, w.ProcessBlock(block(100, 'a', coinbase(1, out(0, alice, 312500000)))))

	utxos := w.UTXOs()
	require.Len(t, utxos, 1)
	assert.True(t, utxos[0].Coinbase)
	assert.False(t, utxos[0].Mature(100))
	assert.False(t, utxos[0].Mature(198))
	assert.True(t, utxos[0].Mature(199), "spendable in block 200, 100 blocks later")

	plain := btc.UTXO{Height: 100}
	assert.True(t, plain.Mature(100))
}

func TestProcessBlockWithoutTransactions(t *testing.T) {
	w := New(Config{Addresses: []string{alice}})

	b := &stream.Block{Block: &btc.Block{BlockHeader: btc.BlockHeader{Height: 1, NTx: 2000}}}
	assert.Error(t, w.ProcessBlock(b))
}

func TestAttachNeedsFullTransactions(t *testing.T) {
	w := New(Config{Addresses: []string{alice}})

	assert.Error(t, w.Attach(stream.New(nil, stream.Config{})))
	assert.NoError(t, w.Attach(stream.New(nil, stream.Config{FullTransactions: true, MaxReorgDepth: 10})))
	assert.Equal(t, 10, w.maxReorgDepth)
}

func TestSnapshot(t *testing.T) {
	w := New(Config{Addresses: []string{alice}})
	require.NoError(t, w.ProcessBlock(block(100, 'a', coinbase(1, out(0, alice, 5000)))))
	require.NoError(t, w.ProcessBlock(block(101, 'a', coinbase(2, out(0, bob, 1)),
		tx(3, []btc.TxIn{in(hashOf('x', 1), 0, "", 0)}, out(0, alice, 700)))))

	s := w.Snapshot()
	assert.Equal(t, int64(101), s.Height)
	require.Len(t, s.UTXOs, 2)

	b := balance.Of(balance.Compute(s, 1), alice)
	assert.Equal(t, btc.Amount(700), b.Confirmed)
	assert.Equal(t, btc.Amount(5000), b.Immature, "a coinbase output 2 blocks deep")
}
