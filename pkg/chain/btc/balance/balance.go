// Package balance computes address balances from UTXOs.
//
// Bitcoin has no balances on chain: the balance of an address is the sum of
// the unspent outputs locked to it. How spendable each output is depends on
// the chain tip, so a balance is computed from a Snapshot, a set of UTXOs as
// of a tip height, and split by confirmations and coinbase maturity.
//
// The UTXOs come from a Source. The node itself can only scan its whole UTXO
// set (scantxoutset, CoreSource); an indexer such as Esplora answers per
// address (EsploraSource); a watch.Watcher keeps the UTXOs it has seen.
package balance

import (
	"context"
	"sort"
	"strings"

	"github.com/c9s/bbgo/pkg/chain/btc"
	"github.com/c9s/bbgo/pkg/chain/btc/esplora"
)

// Snapshot is a set of UTXOs as of the chain tip at Height.
type Snapshot struct {
	Height int64
	UTXOs  []btc.UTXO
}

// Source lists the current UTXOs of addresses.
type Source interface {
	Snapshot(ctx context.Context, addresses []string) (*Snapshot, error)
}

// Balance is the balance of one address, split by spendability.
type Balance struct {
	// Address is the address, or the script hex of outputs without one.
	Address string

	// Confirmed has at least the required confirmations and can be spent.
	Confirmed btc.Amount

	// Pending has fewer confirmations than required, mempool outputs included.
	Pending btc.Amount

	// Immature is coinbase outputs that cannot be spent yet.
	Immature btc.Amount

	UTXOs int
}

// Total is the sum of every part of the balance.
func (b *Balance) Total() btc.Amount {
	return b.Confirmed + b.Pending + b.Immature
}

func (b *Balance) add(o *Balance) {
	b.Confirmed += o.Confirmed
	b.Pending += o.Pending
	b.Immature += o.Immature
	b.UTXOs += o.UTXOs
}

// Compute sums the snapshot's UTXOs per address. An output counts as
// Confirmed with at least minConfirmations confirmations; zero or less counts
// mempool outputs too. Immature coinbase outputs are Immature whatever their
// confirmations. The result is sorted by address.
func Compute(s *Snapshot, minConfirmations int64) []Balance {
	byKey := map[string]*Balance{}
	for i := range s.UTXOs {
		u := &s.UTXOs[i]

		key := u.Address
		if key == "" {
			key = u.Script
		}

		b, ok := byKey[key]
		if !ok {
			b = &Balance{Address: key}
			byKey[key] = b
		}

		b.UTXOs++

		switch {
		case !u.Mature(s.Height):
			b.Immature += u.Value
		case u.Confirmations(s.Height) >= minConfirmations:
			b.Confirmed += u.Value
		default:
			b.Pending += u.Value
		}
	}

	out := make([]Balance, 0, len(byKey))
	for _, b := range byKey {
		out = append(out, *b)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Address < out[j].Address
	})

	return out
}

// Sum adds balances up; the result has no Address.
func Sum(balances []Balance) Balance {
	var total Balance
	for i := range balances {
		total.add(&balances[i])
	}

	return total
}

// Of returns the balance of address from balances, zero when it has none.
// Bech32 addresses compare case-insensitively.
func Of(balances []Balance, address string) Balance {
	for _, b := range balances {
		if b.Address == address || (isBech32(address) && strings.EqualFold(b.Address, address)) {
			return b
		}
	}

	return Balance{Address: address}
}

func isBech32(a string) bool {
	lower := strings.ToLower(a)
	return strings.HasPrefix(lower, "bc1") || strings.HasPrefix(lower, "tb1") || strings.HasPrefix(lower, "bcrt1")
}

// CoreSource reads UTXOs with Bitcoin Core's scantxoutset. It walks the
// whole UTXO set, which takes tens of seconds on mainnet, sees confirmed
// outputs only, and is disabled by most public providers. Arguments
// containing "(" are passed as descriptors, e.g. "wpkh(xpub.../0/*)".
type CoreSource struct {
	Client *btc.Client
}

func (s *CoreSource) Snapshot(ctx context.Context, addresses []string) (*Snapshot, error) {
	descriptors := make([]string, len(addresses))
	for i, a := range addresses {
		descriptors[i] = a
		if !strings.Contains(a, "(") {
			descriptors[i] = "addr(" + a + ")"
		}
	}

	result, err := s.Client.ScanTxOutSet(ctx, descriptors)
	if err != nil {
		return nil, err
	}

	snapshot := &Snapshot{Height: result.Height, UTXOs: make([]btc.UTXO, 0, len(result.Unspents))}
	for i := range result.Unspents {
		snapshot.UTXOs = append(snapshot.UTXOs, result.Unspents[i].UTXO())
	}

	return snapshot, nil
}

// EsploraSource reads UTXOs from an Esplora indexer, one request per
// address. It includes mempool outputs and leaves out those spent by mempool
// transactions. Esplora does not flag coinbase outputs, so an immature one is
// counted as Confirmed; this only matters for a miner's payout address.
type EsploraSource struct {
	Client *esplora.Client
}

func (s *EsploraSource) Snapshot(ctx context.Context, addresses []string) (*Snapshot, error) {
	snapshot := &Snapshot{}
	for _, a := range addresses {
		utxos, err := s.Client.AddressUTXOs(ctx, a)
		if err != nil {
			return nil, err
		}

		snapshot.UTXOs = append(snapshot.UTXOs, utxos...)
	}

	// read after the UTXOs, so the tip is at least as high as their blocks
	tip, err := s.Client.TipHeight(ctx)
	if err != nil {
		return nil, err
	}

	snapshot.Height = tip
	for i := range snapshot.UTXOs {
		snapshot.Height = max(snapshot.Height, snapshot.UTXOs[i].Height)
	}

	return snapshot, nil
}
