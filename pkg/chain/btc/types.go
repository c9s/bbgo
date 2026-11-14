package btc

import (
	"encoding/json"
	"fmt"
)

// The types in this file mirror the JSON of Bitcoin Core's RPC, field for
// field, so the shape of the node's API stays visible. Fields a node version
// may omit are pointers or documented as zero when absent.

// ScriptType is the "type" of a scriptPubKey as Core classifies it.
type ScriptType string

const (
	ScriptTypeNonStandard         ScriptType = "nonstandard"
	ScriptTypePubKey              ScriptType = "pubkey"
	ScriptTypePubKeyHash          ScriptType = "pubkeyhash"            // P2PKH, 1...
	ScriptTypeScriptHash          ScriptType = "scripthash"            // P2SH, 3...
	ScriptTypeMultisig            ScriptType = "multisig"              // bare multisig
	ScriptTypeNullData            ScriptType = "nulldata"              // OP_RETURN
	ScriptTypeWitnessV0KeyHash    ScriptType = "witness_v0_keyhash"    // P2WPKH, bc1q...
	ScriptTypeWitnessV0ScriptHash ScriptType = "witness_v0_scripthash" // P2WSH, bc1q...
	ScriptTypeWitnessV1Taproot    ScriptType = "witness_v1_taproot"    // P2TR, bc1p...
	ScriptTypeAnchor              ScriptType = "anchor"                // P2A, Core 28+
	ScriptTypeWitnessUnknown      ScriptType = "witness_unknown"
)

// ScriptPubKey is the locking script of an output.
type ScriptPubKey struct {
	Asm string `json:"asm"`

	// Desc is the output descriptor, e.g. "addr(bc1q...)#checksum" (Core 22+).
	Desc string `json:"desc,omitempty"`

	Hex string `json:"hex"`

	// Address is empty for scripts without an address, such as OP_RETURN
	// and bare multisig.
	Address string `json:"address,omitempty"`

	Type ScriptType `json:"type"`
}

// ScriptSig is the unlocking script of an input.
type ScriptSig struct {
	Asm string `json:"asm"`
	Hex string `json:"hex"`
}

// Outpoint identifies an output: the transaction that created it and its index.
// An unspent outpoint is a UTXO.
type Outpoint struct {
	TxID Hash   `json:"txid"`
	Vout uint32 `json:"vout"`
}

func (o Outpoint) String() string {
	return fmt.Sprintf("%s:%d", o.TxID, o.Vout)
}

// Prevout is the output an input spends, as Core reports it with
// getblock verbosity=3 or getrawtransaction verbosity=2 (Core 23+ / 25+).
type Prevout struct {
	// Generated is true when the output was created by a coinbase transaction.
	Generated    bool         `json:"generated"`
	Height       int64        `json:"height"`
	Value        Amount       `json:"value"`
	ScriptPubKey ScriptPubKey `json:"scriptPubKey"`
}

// TxIn is a transaction input.
type TxIn struct {
	// Coinbase is the hex of the coinbase field; set only on the single
	// input of a coinbase transaction, which spends no output.
	Coinbase string `json:"coinbase,omitempty"`

	// TxID and Vout name the spent output; zero for a coinbase input.
	TxID Hash   `json:"txid"`
	Vout uint32 `json:"vout"`

	ScriptSig   *ScriptSig `json:"scriptSig,omitempty"`
	TxInWitness []string   `json:"txinwitness,omitempty"`

	// Prevout is nil unless the node was asked for it and could find it.
	Prevout *Prevout `json:"prevout,omitempty"`

	Sequence uint32 `json:"sequence"`
}

// IsCoinbase reports whether the input creates new coins instead of
// spending an output.
func (in *TxIn) IsCoinbase() bool {
	return in.Coinbase != ""
}

// Outpoint returns the output this input spends.
func (in *TxIn) Outpoint() Outpoint {
	return Outpoint{TxID: in.TxID, Vout: in.Vout}
}

// TxOut is a transaction output.
type TxOut struct {
	Value        Amount       `json:"value"`
	N            uint32       `json:"n"`
	ScriptPubKey ScriptPubKey `json:"scriptPubKey"`
}

// Tx is a decoded transaction (getrawtransaction verbose, getblock verbosity>=2).
type Tx struct {
	TxID Hash `json:"txid"`

	// Hash is the witness txid (wtxid); equal to TxID without witness data.
	Hash Hash `json:"hash"`

	Version  uint32 `json:"version"`
	Size     int64  `json:"size"`
	VSize    int64  `json:"vsize"`
	Weight   int64  `json:"weight"`
	LockTime uint32 `json:"locktime"`

	Vin  []TxIn  `json:"vin"`
	Vout []TxOut `json:"vout"`

	// Fee is set only when the node knows every prevout, so never for a
	// coinbase transaction.
	Fee *Amount `json:"fee,omitempty"`

	// Hex is the serialized transaction.
	Hex string `json:"hex,omitempty"`

	// The fields below are set by getrawtransaction for a confirmed
	// transaction; getblock leaves them out.
	InActiveChain *bool `json:"in_active_chain,omitempty"`
	BlockHash     *Hash `json:"blockhash,omitempty"`
	Confirmations int64 `json:"confirmations,omitempty"`
	Time          int64 `json:"time,omitempty"`
	BlockTime     int64 `json:"blocktime,omitempty"`
}

// IsCoinbase reports whether tx is a coinbase transaction, the first of
// every block, which pays the subsidy and the fees to the miner.
func (tx *Tx) IsCoinbase() bool {
	return len(tx.Vin) == 1 && tx.Vin[0].IsCoinbase()
}

// OutputValue sums the outputs.
func (tx *Tx) OutputValue() Amount {
	var total Amount
	for i := range tx.Vout {
		total += tx.Vout[i].Value
	}

	return total
}

// InputValue sums the prevouts of the inputs. ok is false when any prevout
// is missing, which is always the case for a coinbase transaction.
func (tx *Tx) InputValue() (total Amount, ok bool) {
	for i := range tx.Vin {
		if tx.Vin[i].Prevout == nil {
			return 0, false
		}

		total += tx.Vin[i].Prevout.Value
	}

	return total, true
}

// BlockHeader is the result of getblockheader verbose, and the header part of
// getblock.
type BlockHeader struct {
	Hash Hash `json:"hash"`

	// Confirmations is -1 when the block is not on the active chain, i.e. it
	// was reorganized away.
	Confirmations int64 `json:"confirmations"`

	Height     int64   `json:"height"`
	Version    int32   `json:"version"`
	VersionHex string  `json:"versionHex"`
	MerkleRoot Hash    `json:"merkleroot"`
	Time       int64   `json:"time"`
	MedianTime int64   `json:"mediantime"`
	Nonce      uint32  `json:"nonce"`
	Bits       string  `json:"bits"`
	Target     string  `json:"target,omitempty"` // Core 29+
	Difficulty float64 `json:"difficulty"`
	ChainWork  string  `json:"chainwork"`
	NTx        int64   `json:"nTx"`

	// PreviousBlockHash is nil only for the genesis block.
	PreviousBlockHash *Hash `json:"previousblockhash,omitempty"`

	// NextBlockHash is nil for the chain tip.
	NextBlockHash *Hash `json:"nextblockhash,omitempty"`
}

// Block is the result of getblock verbosity=2 or 3.
type Block struct {
	BlockHeader

	StrippedSize int64 `json:"strippedsize"`
	Size         int64 `json:"size"`
	Weight       int64 `json:"weight"`

	// Tx holds every transaction, the coinbase first.
	Tx []Tx `json:"tx"`
}

// BlockchainInfo is the result of getblockchaininfo.
type BlockchainInfo struct {
	// Chain is "main", "test", "testnet4", "signet" or "regtest".
	Chain string `json:"chain"`

	Blocks               int64   `json:"blocks"`
	Headers              int64   `json:"headers"`
	BestBlockHash        Hash    `json:"bestblockhash"`
	Bits                 string  `json:"bits,omitempty"`
	Target               string  `json:"target,omitempty"`
	Difficulty           float64 `json:"difficulty"`
	Time                 int64   `json:"time,omitempty"`
	MedianTime           int64   `json:"mediantime"`
	VerificationProgress float64 `json:"verificationprogress"`
	InitialBlockDownload bool    `json:"initialblockdownload"`
	ChainWork            string  `json:"chainwork"`
	SizeOnDisk           int64   `json:"size_on_disk"`
	Pruned               bool    `json:"pruned"`
	PruneHeight          int64   `json:"pruneheight,omitempty"`

	// Warnings is a string before Core 28 and an array of strings after.
	Warnings json.RawMessage `json:"warnings,omitempty"`
}

// UnspentTxOut is the result of gettxout: an output that is not spent.
type UnspentTxOut struct {
	BestBlock Hash `json:"bestblock"`

	// Confirmations is 0 for an output created by a mempool transaction.
	Confirmations int64 `json:"confirmations"`

	Value        Amount       `json:"value"`
	ScriptPubKey ScriptPubKey `json:"scriptPubKey"`

	// Coinbase is true for an output of a coinbase transaction, which can
	// only be spent after 100 confirmations.
	Coinbase bool `json:"coinbase"`
}

// ScanUnspent is one UTXO found by scantxoutset.
type ScanUnspent struct {
	TxID Hash   `json:"txid"`
	Vout uint32 `json:"vout"`

	// ScriptPubKey is the hex of the locking script.
	ScriptPubKey string `json:"scriptPubKey"`

	// Desc is the matching descriptor with key origin information.
	Desc string `json:"desc"`

	Amount   Amount `json:"amount"`
	Coinbase bool   `json:"coinbase"`
	Height   int64  `json:"height"`

	// BlockHash and Confirmations are reported by Core 27+.
	BlockHash     *Hash `json:"blockhash,omitempty"`
	Confirmations int64 `json:"confirmations,omitempty"`
}

// Outpoint returns the outpoint of the UTXO.
func (u *ScanUnspent) Outpoint() Outpoint {
	return Outpoint{TxID: u.TxID, Vout: u.Vout}
}

// ScanTxOutSetResult is the result of scantxoutset "start".
type ScanTxOutSetResult struct {
	Success     bool          `json:"success"`
	TxOuts      int64         `json:"txouts"`
	Height      int64         `json:"height"`
	BestBlock   Hash          `json:"bestblock"`
	Unspents    []ScanUnspent `json:"unspents"`
	TotalAmount Amount        `json:"total_amount"`
}

// MempoolFees are the fees of a mempool entry.
type MempoolFees struct {
	Base       Amount `json:"base"`
	Modified   Amount `json:"modified"`
	Ancestor   Amount `json:"ancestor"`
	Descendant Amount `json:"descendant"`
}

// MempoolEntry is the result of getmempoolentry.
type MempoolEntry struct {
	VSize  int64 `json:"vsize"`
	Weight int64 `json:"weight"`

	// Time is when the transaction entered this node's mempool, in unix seconds.
	Time int64 `json:"time"`

	// Height is the chain height when the transaction entered the mempool.
	Height int64 `json:"height"`

	DescendantCount int64 `json:"descendantcount"`
	DescendantSize  int64 `json:"descendantsize"`
	AncestorCount   int64 `json:"ancestorcount"`
	AncestorSize    int64 `json:"ancestorsize"`

	WTxID Hash        `json:"wtxid"`
	Fees  MempoolFees `json:"fees"`

	// Depends are the unconfirmed parents; SpentBy the unconfirmed children.
	Depends []Hash `json:"depends"`
	SpentBy []Hash `json:"spentby"`

	// BIP125Replaceable is true for every transaction once full-RBF is the
	// default (Core 28).
	BIP125Replaceable bool `json:"bip125-replaceable"`
	Unbroadcast       bool `json:"unbroadcast"`
}
