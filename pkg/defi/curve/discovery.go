package curve

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/chain/evm/multicall"
)

// FindPoolAddresses lists the pools that trade a pair.
//
// The registry's own find_pools_for_coins is used when it works. It is built
// from a coin pair index that not every chain's registry populates: on BSC it
// reverts for one argument order and returns an empty list for the other,
// although the pools are there.
//
// So an empty answer is only believed when both argument orders agree on it
// without reverting. Anything else falls back to walking pool_list and matching
// the coins locally, which is why that cost is only paid on a chain whose
// registry cannot answer.
func (c *Client) FindPoolAddresses(
	ctx context.Context, from, to common.Address,
) ([]common.Address, error) {
	if from == to {
		return nil, fmt.Errorf("the input and the output coin are the same")
	}

	reliable := true

	for _, order := range [][2]common.Address{{from, to}, {to, from}} {
		values, err := call(ctx, c.caller, metaRegistryABI, c.metaRegistry,
			"find_pools_for_coins", order[0], order[1])
		if err != nil {
			reliable = false
			continue
		}

		if list, ok := values[0].([]common.Address); ok && len(list) > 0 {
			return dedupeAddresses(list), nil
		}
	}

	if reliable {
		// the registry answered, both ways, that it knows no such pool
		return nil, nil
	}

	log.Debugf("curve: find_pools_for_coins did not answer for %s/%s, scanning the pool list",
		from.Hex(), to.Hex())

	return c.scanPoolsForCoins(ctx, from, to)
}

// scanPoolsForCoins builds the coin index locally, once per client, and matches
// the pair against it.
func (c *Client) scanPoolsForCoins(
	ctx context.Context, from, to common.Address,
) ([]common.Address, error) {
	pools, err := c.poolScan(ctx)
	if err != nil {
		return nil, err
	}

	var out []common.Address

	for _, pool := range pools {
		if containsBoth(pool.coins, from, to) || containsBoth(pool.underlying, from, to) {
			out = append(out, pool.address)
		}
	}

	return out, nil
}

// poolScan reads the coins of every pool the registry lists, caching the result
// for the life of the client.
func (c *Client) poolScan(ctx context.Context) ([]scannedPool, error) {
	c.mu.Lock()
	cached := c.scanned
	c.mu.Unlock()

	if cached != nil {
		return cached, nil
	}

	if c.poolCount <= 0 {
		return nil, fmt.Errorf("the curve metaregistry on %s lists no pools", c.chain.Name)
	}

	log.Infof("curve: scanning the coins of %d pools on %s, which happens once per client",
		c.poolCount, c.chain.Name)

	addresses, err := c.poolList(ctx)
	if err != nil {
		return nil, err
	}

	const callsPerScannedPool = 2

	calls := make([]multicall.Call, 0, len(addresses)*callsPerScannedPool)

	for _, address := range addresses {
		for _, method := range []string{"get_coins", "get_underlying_coins"} {
			data, err := metaRegistryABI.Pack(method, address)
			if err != nil {
				return nil, err
			}

			calls = append(calls, multicall.Call{
				Target: c.metaRegistry, AllowFailure: true, CallData: data,
			})
		}
	}

	results, err := multicall.Aggregate3Chunked(ctx, c.caller, c.multicall, calls,
		callsPerScannedPool*poolsPerScanBatch)
	if err != nil {
		return nil, err
	}

	pools := make([]scannedPool, 0, len(addresses))

	for i, address := range addresses {
		pool := scannedPool{address: address}

		if coins, ok := unpackAddressArray(results[i*callsPerScannedPool], "get_coins"); ok {
			pool.coins = coins
		}

		if coins, ok := unpackAddressArray(results[i*callsPerScannedPool+1], "get_underlying_coins"); ok {
			pool.underlying = coins
		}

		if len(pool.coins) == 0 && len(pool.underlying) == 0 {
			continue
		}

		pools = append(pools, pool)
	}

	c.mu.Lock()
	c.scanned = pools
	c.mu.Unlock()

	return pools, nil
}

// poolList reads every pool address the registry indexes.
func (c *Client) poolList(ctx context.Context) ([]common.Address, error) {
	calls := make([]multicall.Call, 0, c.poolCount)
	for i := int64(0); i < c.poolCount; i++ {
		data, err := metaRegistryABI.Pack("pool_list", big.NewInt(i))
		if err != nil {
			return nil, err
		}

		calls = append(calls, multicall.Call{
			Target: c.metaRegistry, AllowFailure: true, CallData: data,
		})
	}

	results, err := multicall.Aggregate3Chunked(ctx, c.caller, c.multicall, calls, 0)
	if err != nil {
		return nil, err
	}

	addresses := make([]common.Address, 0, len(results))

	for _, result := range results {
		if !result.Success || len(result.ReturnData) == 0 {
			continue
		}

		values, err := metaRegistryABI.Unpack("pool_list", result.ReturnData)
		if err != nil {
			continue
		}

		address, err := asAddress(values, 0)
		if err != nil || address == (common.Address{}) {
			continue
		}

		addresses = append(addresses, address)
	}

	return dedupeAddresses(addresses), nil
}

func unpackAddressArray(result multicall.Result, method string) ([]common.Address, bool) {
	if !result.Success || len(result.ReturnData) == 0 {
		return nil, false
	}

	values, err := metaRegistryABI.Unpack(method, result.ReturnData)
	if err != nil || len(values) == 0 {
		return nil, false
	}

	coins := decodeAddressArray(values[0])

	return coins, len(coins) > 0
}

func containsBoth(coins []common.Address, from, to common.Address) bool {
	var hasFrom, hasTo bool

	for _, coin := range coins {
		switch coin {
		case from:
			hasFrom = true
		case to:
			hasTo = true
		}
	}

	return hasFrom && hasTo
}

func dedupeAddresses(list []common.Address) []common.Address {
	seen := make(map[common.Address]bool, len(list))
	out := make([]common.Address, 0, len(list))

	for _, address := range list {
		if address == (common.Address{}) || seen[address] {
			continue
		}

		seen[address] = true
		out = append(out, address)
	}

	return out
}
