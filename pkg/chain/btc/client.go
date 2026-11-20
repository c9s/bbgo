package btc

import (
	"context"
	"errors"
)

// ErrNotFound is returned by methods whose RPC answers null for a missing
// object, such as gettxout for a spent output.
var ErrNotFound = errors.New("btc: not found")

// Client wraps the Bitcoin Core RPC methods for reading blocks, transactions,
// UTXOs and the mempool. Each method maps to one RPC call with the same name
// and returns the node's answer as is.
//
// The node keeps no address index: there is no way to ask it for the
// balance or the history of an address. GetTxOut answers for one known
// outpoint and ScanTxOutSet walks the whole UTXO set; anything else needs an
// external indexer (electrs, Fulcrum, Esplora).
type Client struct {
	*RPCClient
}

// NewClient creates a client for a Bitcoin Core compatible RPC endpoint.
func NewClient(endpoint string, options ...RPCOption) (*Client, error) {
	rc, err := NewRPCClient(endpoint, options...)
	if err != nil {
		return nil, err
	}

	return &Client{RPCClient: rc}, nil
}

// GetBlockchainInfo calls getblockchaininfo.
func (c *Client) GetBlockchainInfo(ctx context.Context) (*BlockchainInfo, error) {
	var info BlockchainInfo
	if err := c.Call(ctx, &info, "getblockchaininfo"); err != nil {
		return nil, err
	}

	return &info, nil
}

// GetBlockCount calls getblockcount: the height of the chain tip.
func (c *Client) GetBlockCount(ctx context.Context) (int64, error) {
	var height int64
	err := c.Call(ctx, &height, "getblockcount")
	return height, err
}

// GetBestBlockHash calls getbestblockhash: the hash of the chain tip.
func (c *Client) GetBestBlockHash(ctx context.Context) (Hash, error) {
	var h Hash
	err := c.Call(ctx, &h, "getbestblockhash")
	return h, err
}

// GetBlockHash calls getblockhash: the hash of the active chain's block at height.
func (c *Client) GetBlockHash(ctx context.Context, height int64) (Hash, error) {
	var h Hash
	err := c.Call(ctx, &h, "getblockhash", height)
	return h, err
}

// GetBlockHeader calls getblockheader with verbose=true.
func (c *Client) GetBlockHeader(ctx context.Context, hash Hash) (*BlockHeader, error) {
	var header BlockHeader
	if err := c.Call(ctx, &header, "getblockheader", hash, true); err != nil {
		return nil, err
	}

	return &header, nil
}

// GetBlock calls getblock with verbosity=3: every transaction decoded, with
// the prevout of every input and the fee of every transaction.
//
// Nodes older than Core 23 answer as for verbosity=2, without prevouts, so
// TxIn.Prevout and Tx.Fee are nil. A pruned node that no longer has the
// undo data of the block also leaves them out.
func (c *Client) GetBlock(ctx context.Context, hash Hash) (*Block, error) {
	var block Block
	if err := c.Call(ctx, &block, "getblock", hash, 3); err != nil {
		return nil, err
	}

	return &block, nil
}

// GetBlockTxIDs calls getblock with verbosity=1 and returns the header and
// the transaction ids, a fraction of the size of GetBlock.
func (c *Client) GetBlockTxIDs(ctx context.Context, hash Hash) (*BlockHeader, []Hash, error) {
	var result struct {
		BlockHeader
		Tx []Hash `json:"tx"`
	}

	if err := c.Call(ctx, &result, "getblock", hash, 1); err != nil {
		return nil, nil, err
	}

	return &result.BlockHeader, result.Tx, nil
}

// GetRawTransaction calls getrawtransaction with verbosity=2, which adds
// prevouts and the fee on Core 25+.
//
// Without txindex the node only finds mempool transactions, or a confirmed
// one when blockHash names its block; pass nil to leave it out.
func (c *Client) GetRawTransaction(ctx context.Context, txid Hash, blockHash *Hash) (*Tx, error) {
	params := []any{txid, 2}
	if blockHash != nil {
		params = append(params, *blockHash)
	}

	var tx Tx
	if err := c.Call(ctx, &tx, "getrawtransaction", params...); err != nil {
		return nil, err
	}

	return &tx, nil
}

// GetTxOut calls gettxout: the output at outpoint if it is unspent.
// It returns ErrNotFound when the output is spent or never existed; the node
// cannot tell the two apart. With includeMempool, an output created by a
// mempool transaction is found and one spent by a mempool transaction is not.
func (c *Client) GetTxOut(ctx context.Context, outpoint Outpoint, includeMempool bool) (*UnspentTxOut, error) {
	var out *UnspentTxOut
	if err := c.Call(ctx, &out, "gettxout", outpoint.TxID, outpoint.Vout, includeMempool); err != nil {
		return nil, err
	}

	if out == nil {
		return nil, ErrNotFound
	}

	return out, nil
}

// ScanTxOutSet calls scantxoutset "start": it walks the whole UTXO set for
// the outputs matching the descriptors, e.g. "addr(bc1q...)" or
// "wpkh(xpub.../0/*)". A scan takes tens of seconds on mainnet, one scan
// runs at a time per node, and public providers usually disable it.
func (c *Client) ScanTxOutSet(ctx context.Context, descriptors []string) (*ScanTxOutSetResult, error) {
	var result ScanTxOutSetResult
	if err := c.Call(ctx, &result, "scantxoutset", "start", descriptors); err != nil {
		return nil, err
	}

	return &result, nil
}

// AbortScanTxOutSet calls scantxoutset "abort" and reports whether a scan was aborted.
func (c *Client) AbortScanTxOutSet(ctx context.Context) (bool, error) {
	var aborted bool
	err := c.Call(ctx, &aborted, "scantxoutset", "abort")
	return aborted, err
}

// GetRawMempool calls getrawmempool: the ids of the transactions in the
// node's mempool. Every node has its own mempool; there is no global one.
func (c *Client) GetRawMempool(ctx context.Context) ([]Hash, error) {
	var txids []Hash
	if err := c.Call(ctx, &txids, "getrawmempool", false); err != nil {
		return nil, err
	}

	return txids, nil
}

// GetMempoolEntry calls getmempoolentry.
func (c *Client) GetMempoolEntry(ctx context.Context, txid Hash) (*MempoolEntry, error) {
	var entry MempoolEntry
	if err := c.Call(ctx, &entry, "getmempoolentry", txid); err != nil {
		return nil, err
	}

	return &entry, nil
}
