package curve

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/chain/evm/multicall"
)

// poolReads are the registry calls that make up one Pool, in order.
var poolReads = []string{
	"get_pool_name",
	"get_coins",
	"get_decimals",
	methodBalances,
	"get_underlying_coins",
	"get_underlying_decimals",
	methodUnderlyingBalances,
	"get_n_coins",
	"get_fees",
	"is_meta",
	"get_lp_token",
	"get_base_pool",
}

// poolSelfReads are the calls that go to the pool rather than to the registry.
// The registry has a get_A, but it reverts for every pool tried on all three
// chains, while the pool's own A answers: 3pool reports 4000.
var poolSelfReads = []string{"A"}

// callsPerPool is one window of the batched Pools read.
var callsPerPool = len(poolReads) + len(poolSelfReads)

// Pool reads the metadata of one pool.
func (c *Client) Pool(ctx context.Context, address common.Address) (*Pool, error) {
	pools, err := c.Pools(ctx, []common.Address{address})
	if err != nil {
		return nil, err
	}

	if len(pools) == 0 {
		return nil, fmt.Errorf("the pool at %s is not indexed by the curve metaregistry on %s",
			address.Hex(), c.chain.Name)
	}

	return pools[0], nil
}

// Pools reads the metadata of several pools in one batched call.
//
// Every call allows failure, because which of them a pool answers depends on
// its implementation: get_pool_name reverts for the NG crypto pools and get_A
// reverts for every crypto pool. A pool that answers none of them is dropped.
func (c *Client) Pools(ctx context.Context, addresses []common.Address) ([]*Pool, error) {
	if len(addresses) == 0 {
		return nil, nil
	}

	calls := make([]multicall.Call, 0, len(addresses)*callsPerPool)
	for _, address := range addresses {
		for _, method := range poolReads {
			data, err := metaRegistryABI.Pack(method, address)
			if err != nil {
				return nil, err
			}

			calls = append(calls, multicall.Call{
				Target: c.metaRegistry, AllowFailure: true, CallData: data,
			})
		}

		for _, method := range poolSelfReads {
			data, err := stablePoolABI.Pack(method)
			if err != nil {
				return nil, err
			}

			calls = append(calls, multicall.Call{
				Target: address, AllowFailure: true, CallData: data,
			})
		}
	}

	results, err := multicall.Aggregate3Chunked(ctx, c.caller, c.multicall, calls,
		callsPerPool*poolsPerMetadataBatch)
	if err != nil {
		return nil, err
	}

	out := make([]*Pool, 0, len(addresses))

	for i, address := range addresses {
		window := results[i*callsPerPool : (i+1)*callsPerPool]

		pool := c.decodePool(address, window)
		if pool == nil {
			log.Debugf("curve: the pool at %s answered no registry read", address.Hex())
			continue
		}

		out = append(out, pool)
	}

	return out, nil
}

func (c *Client) decodePool(address common.Address, results []multicall.Result) *Pool {
	pool := &Pool{Address: address, Index: c.cachedIndexKind(address)}

	value := func(index int) (any, bool) {
		result := results[index]
		if !result.Success || len(result.ReturnData) == 0 {
			return nil, false
		}

		values, err := metaRegistryABI.Unpack(poolReads[index], result.ReturnData)
		if err != nil || len(values) == 0 {
			return nil, false
		}

		return values[0], true
	}

	answered := 0

	if v, ok := value(0); ok {
		if name, isString := v.(string); isString {
			pool.Name = name
			answered++
		}
	}

	if v, ok := value(1); ok {
		pool.Coins = decodeAddressArray(v)
		answered++
	}

	coinCount := len(pool.Coins)

	// a registry handler that cannot enumerate the coins can still report how
	// many there are, which is enough to size the arrays below
	if coinCount == 0 {
		if v, ok := value(7); ok {
			if n, isBig := v.(*big.Int); isBig {
				coinCount = int(n.Int64())
				answered++
			}
		}
	}

	if v, ok := value(2); ok {
		pool.Decimals = toDecimals(decodeUintArray(v, coinCount))
		answered++
	}

	if v, ok := value(3); ok {
		pool.Balances = decodeUintArray(v, coinCount)
		answered++
	}

	if v, ok := value(4); ok {
		underlying := decodeAddressArray(v)

		// a plain pool reports its own coins as its underlying ones, which is
		// noise rather than information
		if !sameAddresses(underlying, pool.Coins) {
			pool.UnderlyingCoins = underlying
			answered++
		}
	}

	underlyingCount := len(pool.UnderlyingCoins)

	if v, ok := value(5); ok && underlyingCount > 0 {
		pool.UnderlyingDecimals = toDecimals(decodeUintArray(v, underlyingCount))
	}

	if v, ok := value(6); ok && underlyingCount > 0 {
		pool.UnderlyingBalances = decodeUintArray(v, underlyingCount)
	}

	if v, ok := value(8); ok {
		pool.Fees = decodeFeeArray(v)
		answered++
	}

	if v, ok := value(9); ok {
		if isMeta, isBool := v.(bool); isBool {
			pool.IsMeta = isMeta
			answered++
		}
	}

	if v, ok := value(10); ok {
		if address, isAddress := v.(common.Address); isAddress {
			pool.LPToken = address
			answered++
		}
	}

	if v, ok := value(11); ok {
		if address, isAddress := v.(common.Address); isAddress {
			pool.BasePool = address
			answered++
		}
	}

	// the amplification coefficient comes from the pool itself, and a
	// CryptoSwap pool reports it on its own scale
	if result := results[len(poolReads)]; result.Success && len(result.ReturnData) >= 32 {
		if values, err := stablePoolABI.Unpack("A", result.ReturnData); err == nil && len(values) > 0 {
			if a, isBig := values[0].(*big.Int); isBig {
				pool.A = a
				answered++
			}
		}
	}

	if answered == 0 {
		return nil
	}

	return pool
}

func toDecimals(values []*big.Int) []int {
	out := make([]int, 0, len(values))
	for _, v := range values {
		if v == nil {
			out = append(out, 0)
			continue
		}

		out = append(out, int(v.Int64()))
	}

	return out
}

func sameAddresses(a, b []common.Address) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}
