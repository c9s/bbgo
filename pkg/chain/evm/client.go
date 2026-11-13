package evm

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// Client is the read-side EVM client. It is bound to one chain and one endpoint
// pool, and it implements bind.ContractCaller so that abigen-generated bindings
// can be constructed directly from it.
type Client struct {
	chain *Chain
	pool  *EndpointPool
}

// NewClient dials the given endpoints and verifies that the remote chain id
// matches the expected chain. A mismatch is a configuration error that would
// otherwise show up much later as unexplained reverts.
func NewClient(ctx context.Context, chain *Chain, endpoints []string) (*Client, error) {
	if chain == nil {
		return nil, fmt.Errorf("chain is required")
	}

	pool, err := NewEndpointPool(endpoints)
	if err != nil {
		return nil, err
	}

	client := &Client{chain: chain, pool: pool}

	remoteChainID, err := client.RemoteChainID(ctx)
	if err != nil {
		pool.Close()
		return nil, err
	}

	if remoteChainID != chain.ID {
		pool.Close()
		return nil, fmt.Errorf("chain id mismatch: config says %s (%d), rpc endpoint reports %d",
			chain.Name, uint64(chain.ID), uint64(remoteChainID))
	}

	return client, nil
}

func (c *Client) Chain() *Chain {
	return c.chain
}

func (c *Client) ChainID() ChainID {
	return c.chain.ID
}

func (c *Client) Endpoints() []string {
	return c.pool.Endpoints()
}

func (c *Client) Close() {
	c.pool.Close()
}

// RemoteChainID queries the chain id reported by the RPC endpoint.
func (c *Client) RemoteChainID(ctx context.Context) (ChainID, error) {
	var id *big.Int
	err := c.pool.Do(ctx, func(client *ethclient.Client) error {
		v, err := client.ChainID(ctx)
		if err != nil {
			return err
		}

		id = v
		return nil
	})
	if err != nil {
		return 0, err
	}

	if !id.IsUint64() {
		return 0, fmt.Errorf("unexpected chain id %s", id.String())
	}

	return ChainID(id.Uint64()), nil
}

func (c *Client) BlockNumber(ctx context.Context) (uint64, error) {
	var n uint64
	err := c.pool.Do(ctx, func(client *ethclient.Client) error {
		v, err := client.BlockNumber(ctx)
		if err != nil {
			return err
		}

		n = v
		return nil
	})

	return n, err
}

func (c *Client) HeaderByNumber(ctx context.Context, number *big.Int) (*ethtypes.Header, error) {
	var header *ethtypes.Header
	err := c.pool.Do(ctx, func(client *ethclient.Client) error {
		v, err := client.HeaderByNumber(ctx, number)
		if err != nil {
			return err
		}

		header = v
		return nil
	})

	return header, err
}

// NativeBalance returns the native (gas) currency balance in wei.
func (c *Client) NativeBalance(ctx context.Context, address common.Address) (*big.Int, error) {
	var balance *big.Int
	err := c.pool.Do(ctx, func(client *ethclient.Client) error {
		v, err := client.BalanceAt(ctx, address, nil)
		if err != nil {
			return err
		}

		balance = v
		return nil
	})

	return balance, err
}

func (c *Client) NonceAt(ctx context.Context, address common.Address, blockNumber *big.Int) (uint64, error) {
	var nonce uint64
	err := c.pool.Do(ctx, func(client *ethclient.Client) error {
		v, err := client.NonceAt(ctx, address, blockNumber)
		if err != nil {
			return err
		}

		nonce = v
		return nil
	})

	return nonce, err
}

// PendingNonceAt returns the next nonce including the pending pool.
func (c *Client) PendingNonceAt(ctx context.Context, address common.Address) (uint64, error) {
	var nonce uint64
	err := c.pool.Do(ctx, func(client *ethclient.Client) error {
		v, err := client.PendingNonceAt(ctx, address)
		if err != nil {
			return err
		}

		nonce = v
		return nil
	})

	return nonce, err
}

// CodeAt implements bind.ContractCaller.
func (c *Client) CodeAt(ctx context.Context, contract common.Address, blockNumber *big.Int) ([]byte, error) {
	var code []byte
	err := c.pool.Do(ctx, func(client *ethclient.Client) error {
		v, err := client.CodeAt(ctx, contract, blockNumber)
		if err != nil {
			return err
		}

		code = v
		return nil
	})

	return code, err
}

// CallContract implements bind.ContractCaller.
func (c *Client) CallContract(ctx context.Context, call ethereum.CallMsg, blockNumber *big.Int) ([]byte, error) {
	var out []byte
	err := c.pool.Do(ctx, func(client *ethclient.Client) error {
		v, err := client.CallContract(ctx, call, blockNumber)
		if err != nil {
			return err
		}

		out = v
		return nil
	})

	return out, err
}

// FilterLogs splits the requested block range into chunks of at most
// Chain.MaxLogRange blocks, because public RPC providers cap the range of a
// single eth_getLogs request.
func (c *Client) FilterLogs(ctx context.Context, query ethereum.FilterQuery) ([]ethtypes.Log, error) {
	if query.BlockHash != nil {
		return c.filterLogs(ctx, query)
	}

	from := uint64(0)
	if query.FromBlock != nil {
		if !query.FromBlock.IsUint64() {
			return nil, fmt.Errorf("unsupported fromBlock %s", query.FromBlock.String())
		}

		from = query.FromBlock.Uint64()
	}

	var to uint64
	if query.ToBlock != nil && query.ToBlock.IsUint64() {
		to = query.ToBlock.Uint64()
	} else {
		head, err := c.BlockNumber(ctx)
		if err != nil {
			return nil, err
		}

		to = head
	}

	if to < from {
		return nil, fmt.Errorf("invalid block range: fromBlock %d > toBlock %d", from, to)
	}

	maxRange := c.chain.MaxLogRange
	if maxRange == 0 {
		maxRange = 1000
	}

	var allLogs []ethtypes.Log
	for start := from; start <= to; start += maxRange {
		end := start + maxRange - 1
		if end > to {
			end = to
		}

		chunk := query
		chunk.FromBlock = new(big.Int).SetUint64(start)
		chunk.ToBlock = new(big.Int).SetUint64(end)

		logs, err := c.filterLogs(ctx, chunk)
		if err != nil {
			return nil, fmt.Errorf("failed to query logs in range [%d, %d]: %w", start, end, err)
		}

		allLogs = append(allLogs, logs...)
	}

	return allLogs, nil
}

func (c *Client) filterLogs(ctx context.Context, query ethereum.FilterQuery) ([]ethtypes.Log, error) {
	var logs []ethtypes.Log
	err := c.pool.Do(ctx, func(client *ethclient.Client) error {
		v, err := client.FilterLogs(ctx, query)
		if err != nil {
			return err
		}

		logs = v
		return nil
	})

	return logs, err
}

func (c *Client) EstimateGas(ctx context.Context, call ethereum.CallMsg) (uint64, error) {
	var gas uint64
	err := c.pool.Do(ctx, func(client *ethclient.Client) error {
		v, err := client.EstimateGas(ctx, call)
		if err != nil {
			return err
		}

		gas = v
		return nil
	})

	return gas, err
}

func (c *Client) SuggestGasPrice(ctx context.Context) (*big.Int, error) {
	var price *big.Int
	err := c.pool.Do(ctx, func(client *ethclient.Client) error {
		v, err := client.SuggestGasPrice(ctx)
		if err != nil {
			return err
		}

		price = v
		return nil
	})

	return price, err
}

func (c *Client) SuggestGasTipCap(ctx context.Context) (*big.Int, error) {
	var tip *big.Int
	err := c.pool.Do(ctx, func(client *ethclient.Client) error {
		v, err := client.SuggestGasTipCap(ctx)
		if err != nil {
			return err
		}

		tip = v
		return nil
	})

	return tip, err
}

func (c *Client) FeeHistory(
	ctx context.Context, blockCount uint64, lastBlock *big.Int, rewardPercentiles []float64,
) (*ethereum.FeeHistory, error) {
	var history *ethereum.FeeHistory
	err := c.pool.Do(ctx, func(client *ethclient.Client) error {
		v, err := client.FeeHistory(ctx, blockCount, lastBlock, rewardPercentiles)
		if err != nil {
			return err
		}

		history = v
		return nil
	})

	return history, err
}

// SendTransaction broadcasts a signed transaction. Failing over is safe here:
// the transaction is already signed, so a second endpoint produces the same
// hash and answers "already known", which is treated as non-retryable.
func (c *Client) SendTransaction(ctx context.Context, tx *ethtypes.Transaction) error {
	return c.pool.Do(ctx, func(client *ethclient.Client) error {
		return client.SendTransaction(ctx, tx)
	})
}

// TransactionReceipt returns the receipt of a transaction. It returns
// ethereum.NotFound while the transaction is still pending.
func (c *Client) TransactionReceipt(ctx context.Context, hash common.Hash) (*ethtypes.Receipt, error) {
	var receipt *ethtypes.Receipt
	err := c.pool.Do(ctx, func(client *ethclient.Client) error {
		v, err := client.TransactionReceipt(ctx, hash)
		if err != nil {
			return err
		}

		receipt = v
		return nil
	})

	return receipt, err
}

// Client must stay usable as the sender's backend.
var _ TxClient = (*Client)(nil)
