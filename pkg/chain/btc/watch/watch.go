// Package watch tracks the UTXOs of a set of addresses over a block stream.
//
// For every block the watcher reports the outputs paid to a watched address
// (Received) and the watched outputs that were spent (Spent), and keeps the
// set of the watched addresses' unspent outputs. A reorg reverts what the
// removed blocks did, newest first, so the UTXO set always matches the
// stream's chain.
//
// The node has no address index, so the watcher only knows what it saw: an
// output received before the stream started is unknown unless Seed adds it,
// e.g. from scantxoutset at the height the stream starts after. A spend of
// such an output is still reported, from the input's prevout, with Known set
// to false.
package watch

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/chain/btc"
	"github.com/c9s/bbgo/pkg/chain/btc/balance"
	"github.com/c9s/bbgo/pkg/chain/btc/stream"
)

const defaultMaxReorgDepth = 64

// Kind is what happened to a watched output.
type Kind string

const (
	// Received is an output paid to a watched address.
	Received Kind = "received"

	// Spent is a watched output spent by an input.
	Spent Kind = "spent"
)

// Event is one change of a watched output.
type Event struct {
	Kind Kind

	// UTXO is the output received or spent.
	btc.UTXO

	// TxID is the transaction that created the output (Received) or spent
	// it (Spent), and Block the block it is in.
	TxID  btc.Hash
	Block stream.BlockRef

	// Known is false for a Spent output the watcher had not tracked: it was
	// received before the stream started and not seeded. Its details come
	// from the input's prevout. Always true for Received.
	Known bool
}

func (e *Event) String() string {
	label := e.Address
	if label == "" {
		label = "script " + e.Script
	}

	return fmt.Sprintf("%s %s BTC %s %s in tx %s at %s", e.Kind, e.Value, label, e.Outpoint, e.TxID, e.Block)
}

type Config struct {
	// Addresses are the addresses to watch, as the node writes them.
	Addresses []string

	// Scripts are scriptPubKeys to watch, in hex, for outputs without an
	// address such as bare multisig.
	Scripts []string

	// MaxReorgDepth is how many blocks of changes are kept to revert a
	// reorg; match the stream's. It defaults to 64.
	MaxReorgDepth int
}

// Watcher tracks the UTXOs of watched addresses. It is safe to query from
// other goroutines while a stream feeds it.
//
//go:generate callbackgen -type Watcher
type Watcher struct {
	mu sync.Mutex

	addresses map[string]struct{}
	scripts   map[string]struct{}
	utxos     map[btc.Outpoint]btc.UTXO

	// journal holds the events of the last processed blocks, oldest first,
	// to revert them on a reorg.
	journal       []blockJournal
	maxReorgDepth int

	tip    stream.BlockRef
	hasTip bool

	eventCallbacks  []func(event Event)
	revertCallbacks []func(event Event)
}

type blockJournal struct {
	ref    stream.BlockRef
	events []Event
}

func New(config Config) *Watcher {
	w := &Watcher{
		addresses:     map[string]struct{}{},
		scripts:       map[string]struct{}{},
		utxos:         map[btc.Outpoint]btc.UTXO{},
		maxReorgDepth: config.MaxReorgDepth,
	}

	if w.maxReorgDepth <= 0 {
		w.maxReorgDepth = defaultMaxReorgDepth
	}

	for _, a := range config.Addresses {
		w.addresses[NormalizeAddress(a)] = struct{}{}
	}

	for _, s := range config.Scripts {
		w.scripts[strings.ToLower(s)] = struct{}{}
	}

	return w
}

// NormalizeAddress lowercases bech32 addresses, which are case-insensitive
// and written lowercase by the node. Base58 addresses are case-sensitive.
func NormalizeAddress(a string) string {
	lower := strings.ToLower(a)
	for _, hrp := range []string{"bc1", "tb1", "bcrt1"} {
		if strings.HasPrefix(lower, hrp) {
			return lower
		}
	}

	return a
}

// Attach feeds the watcher from s. The stream must read full transactions.
func (w *Watcher) Attach(s *stream.Stream) error {
	config := s.Config()
	if !config.FullTransactions {
		return errors.New("btc watch: the stream must read full transactions")
	}

	w.mu.Lock()
	w.maxReorgDepth = config.MaxReorgDepth
	w.mu.Unlock()

	s.OnBlock(func(b *stream.Block) {
		if err := w.ProcessBlock(b); err != nil {
			log.WithError(err).Errorf("btc watch: failed to process block %s", b.Ref())
		}
	})

	s.OnReorg(w.Revert)
	return nil
}

// Seed adds UTXOs known from elsewhere, e.g. scantxoutset, before the
// stream starts after their height. A reorg cannot revert seeded outputs.
func (w *Watcher) Seed(utxos []btc.UTXO) {
	w.mu.Lock()
	defer w.mu.Unlock()

	for _, u := range utxos {
		w.utxos[u.Outpoint] = u
	}
}

func (w *Watcher) matches(spk *btc.ScriptPubKey) bool {
	if spk.Address != "" {
		if _, ok := w.addresses[spk.Address]; ok {
			return true
		}
	}

	_, ok := w.scripts[strings.ToLower(spk.Hex)]
	return ok
}

// ProcessBlock applies a block: outputs paid to watched addresses are added,
// watched outputs spent by its inputs are removed. Transactions are applied
// in block order, so an output created and spent in the same block yields
// both events.
func (w *Watcher) ProcessBlock(b *stream.Block) error {
	if len(b.Tx) == 0 && b.NTx > 0 {
		return errors.New("btc watch: the block has no transactions; the stream must read full transactions")
	}

	ref := b.Ref()

	w.mu.Lock()
	var events []Event
	for i := range b.Tx {
		events = w.applyTx(&b.Tx[i], ref, events)
	}

	w.journal = append(w.journal, blockJournal{ref: ref, events: events})
	if over := len(w.journal) - w.maxReorgDepth; over > 0 {
		w.journal = append(w.journal[:0], w.journal[over:]...)
	}

	w.tip = ref
	w.hasTip = true
	w.mu.Unlock()

	for _, e := range events {
		w.EmitEvent(e)
	}

	return nil
}

func (w *Watcher) applyTx(tx *btc.Tx, ref stream.BlockRef, events []Event) []Event {
	for i := range tx.Vin {
		in := &tx.Vin[i]
		if in.IsCoinbase() {
			continue
		}

		outpoint := in.Outpoint()
		if u, ok := w.utxos[outpoint]; ok {
			delete(w.utxos, outpoint)
			events = append(events, Event{Kind: Spent, UTXO: u, TxID: tx.TxID, Block: ref, Known: true})
			continue
		}

		// an output from before the stream started
		if in.Prevout != nil && w.matches(&in.Prevout.ScriptPubKey) {
			events = append(events, Event{
				Kind: Spent,
				UTXO: btc.UTXO{
					Outpoint: outpoint,
					Address:  in.Prevout.ScriptPubKey.Address,
					Script:   in.Prevout.ScriptPubKey.Hex,
					Value:    in.Prevout.Value,
					Height:   in.Prevout.Height,
					Coinbase: in.Prevout.Generated,
				},
				TxID:  tx.TxID,
				Block: ref,
			})
		}
	}

	coinbase := tx.IsCoinbase()
	for i := range tx.Vout {
		out := &tx.Vout[i]
		if !w.matches(&out.ScriptPubKey) {
			continue
		}

		u := btc.UTXO{
			Outpoint:  btc.Outpoint{TxID: tx.TxID, Vout: out.N},
			Address:   out.ScriptPubKey.Address,
			Script:    out.ScriptPubKey.Hex,
			Value:     out.Value,
			Height:    ref.Height,
			BlockHash: ref.Hash,
			Coinbase:  coinbase,
		}

		w.utxos[u.Outpoint] = u
		events = append(events, Event{Kind: Received, UTXO: u, TxID: tx.TxID, Block: ref, Known: true})
	}

	return events
}

// Revert undoes the blocks a reorg removed, newest first, and emits a revert
// for each of their events in reverse order. Blocks the watcher never
// processed, such as the one before the stream's first block, are skipped.
func (w *Watcher) Revert(reorg stream.Reorg) {
	w.mu.Lock()
	var reverted []Event
	for _, ref := range reorg.Removed {
		last := len(w.journal) - 1
		if last < 0 || w.journal[last].ref != ref {
			continue
		}

		events := w.journal[last].events
		for i := len(events) - 1; i >= 0; i-- {
			e := events[i]
			switch {
			case e.Kind == Received:
				delete(w.utxos, e.Outpoint)
			case e.Kind == Spent && e.Known:
				w.utxos[e.Outpoint] = e.UTXO
			}

			reverted = append(reverted, e)
		}

		w.journal = w.journal[:last]
	}

	w.tip = reorg.Ancestor
	w.hasTip = true
	w.mu.Unlock()

	for _, e := range reverted {
		w.EmitRevert(e)
	}
}

// Tip returns the last block applied or the ancestor of the last reorg.
func (w *Watcher) Tip() (stream.BlockRef, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.tip, w.hasTip
}

// UTXOs returns the tracked unspent outputs, by height then outpoint.
func (w *Watcher) UTXOs() []btc.UTXO {
	w.mu.Lock()
	out := make([]btc.UTXO, 0, len(w.utxos))
	for _, u := range w.utxos {
		out = append(out, u)
	}
	w.mu.Unlock()

	sort.Slice(out, func(i, j int) bool {
		if out[i].Height != out[j].Height {
			return out[i].Height < out[j].Height
		}

		if out[i].Outpoint.TxID != out[j].Outpoint.TxID {
			return out[i].Outpoint.TxID.String() < out[j].Outpoint.TxID.String()
		}

		return out[i].Outpoint.Vout < out[j].Outpoint.Vout
	})

	return out
}

// Balances sums the tracked unspent outputs per address; outputs without an
// address are keyed by their script hex.
func (w *Watcher) Balances() map[string]btc.Amount {
	w.mu.Lock()
	defer w.mu.Unlock()

	balances := map[string]btc.Amount{}
	for _, u := range w.utxos {
		key := u.Address
		if key == "" {
			key = u.Script
		}

		balances[key] += u.Value
	}

	return balances
}

// Snapshot returns the tracked UTXOs as of the watcher's tip, for
// balance.Compute to split by confirmations and coinbase maturity.
func (w *Watcher) Snapshot() *balance.Snapshot {
	utxos := w.UTXOs()
	tip, _ := w.Tip()
	return &balance.Snapshot{Height: tip.Height, UTXOs: utxos}
}
