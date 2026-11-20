package evm

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

// Block is a block as the RPC node reports it.
//
// It is decoded here rather than through ethclient.BlockByNumber for two
// reasons. Hash is the hash the node reports, not one recomputed from the
// header fields: a recomputed hash is wrong as soon as a chain adds a header
// field go-ethereum does not know, and a wrong hash breaks parent-hash reorg
// detection. And a transaction type go-ethereum cannot decode, such as the
// Polygon state-sync transaction, is skipped and counted instead of failing the
// whole block.
type Block struct {
	Hash   common.Hash
	Header *ethtypes.Header

	// Transactions is filled only when the block was fetched with full
	// transactions; otherwise TxHashes is.
	Transactions []*Transaction
	TxHashes     []common.Hash

	// SkippedTransactions counts the transactions that could not be decoded.
	SkippedTransactions int
}

func (b *Block) Number() uint64 {
	return b.Header.Number.Uint64()
}

func (b *Block) ParentHash() common.Hash {
	return b.Header.ParentHash
}

// Time returns the block timestamp in unix seconds.
func (b *Block) Time() uint64 {
	return b.Header.Time
}

// TxCount returns the number of transactions in the block, including the ones
// that could not be decoded.
func (b *Block) TxCount() int {
	if b.TxHashes != nil {
		return len(b.TxHashes)
	}

	return len(b.Transactions) + b.SkippedTransactions
}

// Transaction is a transaction together with the sender the node reports, so
// the caller does not have to recover it from the signature.
type Transaction struct {
	*ethtypes.Transaction

	From common.Address
}

// BlockByNumber fetches a block. A nil number means the latest block. It
// returns ethereum.NotFound when the endpoint does not have the block yet,
// which happens when a failover lands on an endpoint lagging behind.
func (c *Client) BlockByNumber(ctx context.Context, number *big.Int, fullTx bool) (*Block, error) {
	return c.getBlock(ctx, "eth_getBlockByNumber", toBlockNumArg(number), fullTx)
}

// BlockByHash fetches a block by hash. It returns ethereum.NotFound for a hash
// the endpoint does not know, including a block that was reorged away.
func (c *Client) BlockByHash(ctx context.Context, hash common.Hash, fullTx bool) (*Block, error) {
	return c.getBlock(ctx, "eth_getBlockByHash", hash, fullTx)
}

func (c *Client) getBlock(ctx context.Context, method string, arg any, fullTx bool) (*Block, error) {
	var raw json.RawMessage
	err := c.pool.Do(ctx, func(client *ethclient.Client) error {
		return client.Client().CallContext(ctx, &raw, method, arg, fullTx)
	})
	if err != nil {
		return nil, err
	}

	return DecodeBlock(raw, fullTx)
}

// DecodeBlock decodes the result of eth_getBlockByNumber / eth_getBlockByHash.
// A null result is ethereum.NotFound.
func DecodeBlock(raw json.RawMessage, fullTx bool) (*Block, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, ethereum.NotFound
	}

	var header ethtypes.Header
	if err := json.Unmarshal(raw, &header); err != nil {
		return nil, fmt.Errorf("failed to decode the block header: %w", err)
	}

	var body struct {
		Hash         *common.Hash      `json:"hash"`
		Transactions []json.RawMessage `json:"transactions"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, fmt.Errorf("failed to decode the block body: %w", err)
	}

	// a pending block has no hash; this layer only deals with sealed blocks
	if body.Hash == nil {
		return nil, fmt.Errorf("block %s has no hash", header.Number)
	}

	block := &Block{
		Hash:   *body.Hash,
		Header: &header,
	}

	if !fullTx {
		block.TxHashes = make([]common.Hash, 0, len(body.Transactions))
		for _, rawTx := range body.Transactions {
			var hash common.Hash
			if err := json.Unmarshal(rawTx, &hash); err != nil {
				return nil, fmt.Errorf("failed to decode a transaction hash of block %d: %w", block.Number(), err)
			}

			block.TxHashes = append(block.TxHashes, hash)
		}

		return block, nil
	}

	block.Transactions = make([]*Transaction, 0, len(body.Transactions))
	for _, rawTx := range body.Transactions {
		tx, err := decodeTransaction(rawTx)
		if err != nil {
			block.SkippedTransactions++
			continue
		}

		block.Transactions = append(block.Transactions, tx)
	}

	return block, nil
}

func decodeTransaction(raw json.RawMessage) (*Transaction, error) {
	var tx ethtypes.Transaction
	if err := tx.UnmarshalJSON(raw); err != nil {
		return nil, err
	}

	var extra struct {
		From common.Address `json:"from"`
	}
	if err := json.Unmarshal(raw, &extra); err != nil {
		return nil, err
	}

	return &Transaction{Transaction: &tx, From: extra.From}, nil
}

func toBlockNumArg(number *big.Int) string {
	if number == nil {
		return "latest"
	}

	if number.Sign() >= 0 {
		return hexutil.EncodeBig(number)
	}

	// negative numbers are the go-ethereum encoding of the block tags
	return rpc.BlockNumber(number.Int64()).String()
}
