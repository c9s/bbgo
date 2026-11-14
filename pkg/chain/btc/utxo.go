package btc

import "strings"

// UTXO is an unspent output together with where it was created: the common
// shape of the UTXOs a watcher tracks, scantxoutset finds or an indexer lists.
type UTXO struct {
	Outpoint Outpoint

	// Address is empty for a script without an address.
	Address string

	// Script is the scriptPubKey hex; empty when the source does not report it.
	Script string

	Value Amount

	// Height is the block the output was created in, 0 while its transaction
	// is in the mempool. BlockHash is zero when unconfirmed or not reported.
	Height    int64
	BlockHash Hash

	// Coinbase outputs are spendable only after CoinbaseMaturity blocks.
	Coinbase bool
}

// Confirmed reports whether the output's transaction is in a block.
func (u *UTXO) Confirmed() bool {
	return u.Height > 0
}

// Confirmations returns the confirmations of the output when the chain tip
// is at tip: 1 in the tip block, 0 in the mempool.
func (u *UTXO) Confirmations(tip int64) int64 {
	if !u.Confirmed() || tip < u.Height {
		return 0
	}

	return tip - u.Height + 1
}

// Mature reports whether the output can be spent in the block after tip.
// Only coinbase outputs can be immature.
func (u *UTXO) Mature(tip int64) bool {
	if !u.Coinbase {
		return true
	}

	return u.Confirmed() && tip+1-u.Height >= CoinbaseMaturity
}

// DescriptorAddress returns X of an "addr(X)#checksum" descriptor, as
// scantxoutset reports, or "" for any other descriptor.
func DescriptorAddress(desc string) string {
	desc, _, _ = strings.Cut(desc, "#")
	if !strings.HasPrefix(desc, "addr(") || !strings.HasSuffix(desc, ")") {
		return ""
	}

	return desc[len("addr(") : len(desc)-1]
}

// UTXO converts a scantxoutset result into a UTXO.
func (u *ScanUnspent) UTXO() UTXO {
	out := UTXO{
		Outpoint: u.Outpoint(),
		Address:  DescriptorAddress(u.Desc),
		Script:   u.ScriptPubKey,
		Value:    u.Amount,
		Height:   u.Height,
		Coinbase: u.Coinbase,
	}

	if u.BlockHash != nil {
		out.BlockHash = *u.BlockHash
	}

	return out
}
