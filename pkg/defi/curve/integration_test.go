package curve_test

import (
	"context"
	"errors"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/chain/evm"
	"github.com/c9s/bbgo/pkg/chain/evm/token"
	"github.com/c9s/bbgo/pkg/defi/curve"
)

// probe is one pair to trade on one chain.
type probe struct {
	from   string
	to     string
	amount string

	// parity says that the pair is pegged and that the chain has a pool deep
	// enough to price it near 1.0. It is false where Curve is deployed but
	// holds dust: on BSC the deepest USDT/USDC pool is drained far enough that
	// 1000 USDT quotes 294 USDC, which is a fact about that deployment rather
	// than about this adapter, so only the mechanics are asserted there.
	parity bool
}

const (
	symbolUSDC = "USDC"
	symbolUSDT = "USDT"
)

// stableProbes is the pegged pair of each chain, where the realized price is
// the check on the decimals handling.
var stableProbes = map[evm.ChainID]probe{
	evm.ChainIDEthereum: {from: symbolUSDC, to: symbolUSDT, amount: "1000", parity: true},
	evm.ChainIDPolygon:  {from: "USDC.E", to: symbolUSDT, amount: "1000", parity: true},
	evm.ChainIDBSC:      {from: symbolUSDT, to: symbolUSDC, amount: "1000", parity: false},
}

// cryptoProbes is the volatile pair, which on Curve lives in the CryptoSwap
// pools and so exercises the uint256 index variant.
var cryptoProbes = map[evm.ChainID]probe{
	evm.ChainIDEthereum: {from: "WETH", to: symbolUSDC, amount: "1"},
	evm.ChainIDPolygon:  {from: "WETH", to: symbolUSDT, amount: "1"},
	evm.ChainIDBSC:      {from: "ETH", to: symbolUSDT, amount: "1"},
}

// TestIntegration validates the AddressProvider address, the MetaRegistry ABI
// and the pool ABIs of both index families against live pools:
//
//	TEST_EVM_CHAIN=bsc TEST_EVM_RPC_ENDPOINTS=https://... go test ./pkg/defi/curve -run TestIntegration -v
func TestIntegration(t *testing.T) {
	endpoints, ok := os.LookupEnv("TEST_EVM_RPC_ENDPOINTS")
	if !ok || strings.TrimSpace(endpoints) == "" {
		t.Skip("TEST_EVM_RPC_ENDPOINTS is not set")
	}

	chainName := os.Getenv("TEST_EVM_CHAIN")
	if chainName == "" {
		chainName = "ethereum"
	}

	chain, ok := evm.LookupChain(chainName)
	require.True(t, ok, "unknown chain %s", chainName)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	client, err := evm.NewClient(ctx, chain, strings.Split(endpoints, ","))
	require.NoError(t, err)

	defer client.Close()

	registry := token.NewRegistry(client, chain.Multicall3)
	registry.AddKnownSymbols(uint64(chain.ID))

	// the AddressProvider has to answer with a MetaRegistry that indexes pools,
	// which is the whole of this adapter's configuration
	dex, err := curve.NewClient(ctx, chain, client, curve.Options{Tokens: registry})
	require.NoError(t, err)

	assert.False(t, dex.CanWrite())
	assert.NotEqual(t, common.Address{}, dex.MetaRegistryAddress())
	assert.Positive(t, dex.PoolCount(), "the metaregistry indexes no pools")

	t.Logf("metaregistry %s on %s indexes %d pools",
		dex.MetaRegistryAddress().Hex(), chain.Name, dex.PoolCount())

	stable, ok := stableProbes[chain.ID]
	require.True(t, ok, "no stable probe for chain %s", chain.Name)

	from, to := resolvePair(ctx, t, registry, stable)

	t.Run("stable", func(t *testing.T) {
		quotes, amountIn := quotePair(ctx, t, dex, from, to, stable.amount)

		var parity *curve.Quote

		for _, quote := range quotes {
			price, _ := quote.Price(from.Decimals, to.Decimals).Float64()

			t.Logf("  %-44s %18.8f  %s", quote.Route.Pool.Hex(), price, quote.Route)

			if parity == nil && price > 0.9 && price < 1.1 {
				parity = quote
			}
		}

		assertQuoteInvariants(ctx, t, dex, quotes, amountIn)

		if !stable.parity {
			t.Logf("%s holds no %s/%s pool deep enough to price %s %s near parity",
				chain.Name, from.Symbol, to.Symbol, stable.amount, from.Symbol)
			return
		}

		// a pegged pair has a pool at parity, and a decimals mistake between a
		// 6 and an 18 decimal stablecoin would be a factor of 1e12, so nothing
		// in the table would be anywhere near 1.0.
		//
		// Parity is asserted of the table rather than of its head because a
		// quote is only the pool's own arithmetic: Curve keeps deprecated pools
		// of wrapped coins that can no longer be redeemed, and those quote
		// prices far from parity that they cannot honour.
		require.NotNil(t, parity,
			"no %s/%s pool quotes near parity, which a decimals mistake would explain",
			from.Symbol, to.Symbol)

		// the search cannot return less than a pool it quoted
		assert.GreaterOrEqual(t, quotes[0].AmountOut.Cmp(parity.AmountOut), 0)
	})

	t.Run("crypto", func(t *testing.T) {
		crypto, ok := cryptoProbes[chain.ID]
		if !ok {
			t.Skipf("no crypto probe for chain %s", chain.Name)
		}

		base, quote := resolvePair(ctx, t, registry, crypto)

		quotes, amountIn := quotePair(ctx, t, dex, base, quote, crypto.amount)

		assertQuoteInvariants(ctx, t, dex, quotes, amountIn)

		price, _ := quotes[0].Price(base.Decimals, quote.Decimals).Float64()
		assert.Positive(t, price)

		t.Logf("%s %s -> %s %s at %.4f via %s",
			crypto.amount, base.Symbol, quote.FormatRaw(quotes[0].AmountOut), quote.Symbol,
			price, quotes[0].Route)

		kinds := map[curve.IndexKind]int{}
		for _, q := range quotes {
			kinds[q.Route.Index]++
		}

		t.Logf("index kinds detected: %d stableswap, %d cryptoswap",
			kinds[curve.IndexInt128], kinds[curve.IndexUint256])
	})

	t.Run("pool", func(t *testing.T) {
		routes, err := dex.Routes(ctx, from.Address, to.Address)
		require.NoError(t, err)
		require.NotEmpty(t, routes)

		pool, err := dex.Pool(ctx, routes[0].Pool)
		require.NoError(t, err)

		t.Logf("%s", pool)

		// the pair has to be among the pool's coins, or among the base pool's
		// coins when the registry routed it through the underlying ones
		coins := pool.Coins
		if routes[0].Underlying {
			coins = pool.UnderlyingCoins
		}

		assert.Contains(t, coins, from.Address)
		assert.Contains(t, coins, to.Address)

		require.Len(t, pool.Decimals, len(pool.Coins))

		for i, decimals := range pool.Decimals {
			assert.GreaterOrEqual(t, decimals, 0)
			assert.LessOrEqual(t, decimals, 18, "coin %d has %d decimals", i, decimals)
		}

		// a swap fee above 1% would mean the 1e10 denominator is wrong
		assert.Less(t, pool.Fee(), 0.01, "the swap fee %.6f is implausible", pool.Fee())
		assert.GreaterOrEqual(t, pool.Fee(), 0.0)
	})

	t.Run("nopool", func(t *testing.T) {
		nonsense := common.HexToAddress("0x000000000000000000000000000000000000dEaD")

		_, err := dex.Routes(ctx, from.Address, nonsense)
		assert.ErrorIs(t, err, curve.ErrNoPool)
	})
}

func resolvePair(
	ctx context.Context, t *testing.T, registry *token.Registry, p probe,
) (from, to token.Token) {
	t.Helper()

	from, err := registry.Resolve(ctx, p.from)
	require.NoError(t, err)

	to, err = registry.Resolve(ctx, p.to)
	require.NoError(t, err)

	return from, to
}

// quotePair quotes a pair and skips the test when the chain has no pool for it,
// or none that can price the size: a deployment holding dust is not an adapter
// failure, and ErrNoPool and ErrNoQuote are how the two are told apart.
func quotePair(
	ctx context.Context, t *testing.T, dex *curve.Client, from, to token.Token, amount string,
) ([]*curve.Quote, *big.Int) {
	t.Helper()

	amountIn, err := token.ParseAmount(amount, from.Decimals)
	require.NoError(t, err)

	routes, err := dex.Routes(ctx, from.Address, to.Address)
	if errors.Is(err, curve.ErrNoPool) {
		t.Skipf("%s/%s has no curve pool on %s", from.Symbol, to.Symbol, dex.ChainName())
	}

	require.NoError(t, err)
	require.NotEmpty(t, routes)

	t.Logf("%s/%s: %d routes on %s", from.Symbol, to.Symbol, len(routes), dex.ChainName())

	// the routes are ordered by the pool's balance of the output coin, which is
	// the upper bound that lets the search stop early
	for i, route := range routes {
		assert.NotEqual(t, route.I, route.J, "route %d trades a coin with itself", i)
		assert.GreaterOrEqual(t, route.OutBalance.Sign(), 0)

		if i > 0 {
			assert.GreaterOrEqual(t, routes[i-1].OutBalance.Cmp(route.OutBalance), 0,
				"the routes are not ordered by the output balance")
		}
	}

	quotes, err := dex.Quotes(ctx, from.Address, to.Address, amountIn)
	if errors.Is(err, curve.ErrNoQuote) {
		t.Skipf("no %s/%s pool on %s quotes %s %s",
			from.Symbol, to.Symbol, dex.ChainName(), amount, from.Symbol)
	}

	require.NoError(t, err)
	require.NotEmpty(t, quotes)

	return quotes, amountIn
}

// assertQuoteInvariants checks what holds of every quote table, whatever the
// state of the pools behind it.
func assertQuoteInvariants(
	ctx context.Context, t *testing.T, dex *curve.Client, quotes []*curve.Quote, amountIn *big.Int,
) {
	t.Helper()

	best := quotes[0]

	require.Positive(t, best.AmountOut.Sign())

	for i, quote := range quotes {
		assert.Zero(t, new(big.Int).Sub(quote.AmountIn, amountIn).Sign())

		// the index type is detected by calling the pool, so a quote that
		// answered always leaves it known
		assert.NotEqual(t, curve.IndexUnknown, quote.Route.Index,
			"quote %d left the index kind undetected", i)

		if i > 0 {
			assert.GreaterOrEqual(t, quotes[i-1].AmountOut.Cmp(quote.AmountOut), 0,
				"the quotes are not ordered by output")
		}

		// a pool cannot pay out more of a coin than it holds, which is the
		// bound the search prunes with. It only holds for a direct route: the
		// registry does not convert a lending pool's underlying balances.
		if quote.Route.OutBalanceExact && quote.Route.OutBalance.Sign() > 0 {
			assert.LessOrEqual(t, quote.AmountOut.Cmp(quote.Route.OutBalance), 0,
				"the quote of %s exceeds its balance of the output coin", quote.Route.Pool.Hex())
		}
	}

	// quoting the best route on its own must agree with the search
	direct, err := dex.Quote(ctx, best.Route, amountIn)
	require.NoError(t, err)
	assert.Zero(t, new(big.Int).Sub(direct.AmountOut, best.AmountOut).Sign())

	// the slippage floor is below the quote and above zero
	floor := best.AmountOutMinimum(50)
	assert.Negative(t, floor.Cmp(best.AmountOut))
	assert.Positive(t, floor.Sign())
}
