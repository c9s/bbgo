package v3_test

import (
	"context"
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
	v3 "github.com/c9s/bbgo/pkg/defi/uniswap/v3"
)

// defaultPairs is the pair probed on each chain, chosen for depth.
var defaultPairs = map[evm.ChainID][2]string{
	evm.ChainIDEthereum: {"WETH", "USDC"},
	evm.ChainIDBSC:      {"WBNB", "USDT"},
	evm.ChainIDPolygon:  {"WPOL", "USDC"},
}

// TestIntegration validates the deployment addresses and every ABI in this
// package against live pools:
//
//	TEST_EVM_CHAIN=bsc TEST_EVM_RPC_ENDPOINTS=https://... go test ./pkg/defi/uniswap/v3 -run TestIntegration -v
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

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	client, err := evm.NewClient(ctx, chain, strings.Split(endpoints, ","))
	require.NoError(t, err)

	defer client.Close()

	registry := token.NewRegistry(client, chain.Multicall3)
	registry.AddKnownSymbols(uint64(chain.ID))

	pair, ok := defaultPairs[chain.ID]
	require.True(t, ok, "no default pair for chain %s", chain.Name)

	base, err := registry.Resolve(ctx, pair[0])
	require.NoError(t, err)

	quote, err := registry.Resolve(ctx, pair[1])
	require.NoError(t, err)

	// every configured deployment of this chain must answer
	for _, deployment := range v3.Deployments(chain.ID) {
		t.Run(deployment.Name, func(t *testing.T) {
			dex, err := v3.NewClient(ctx, chain, client, v3.Options{
				Deployment: deployment.Name,
				Tokens:     registry,
			})
			require.NoError(t, err)

			assert.False(t, dex.CanWrite())
			t.Logf("%s", dex.Deployment())

			pools, err := dex.FindPools(ctx, base.Address, quote.Address)
			require.NoError(t, err)
			require.NotEmpty(t, pools, "%s/%s has no pool on %s", base.Symbol, quote.Symbol, deployment.Name)

			tiers := map[uint32]bool{}
			for _, fee := range deployment.FeeTiers {
				tiers[fee] = true
			}

			for _, pool := range pools {
				assert.True(t, tiers[pool.Fee], "fee tier %d is not in the deployment ladder", pool.Fee)
				assert.True(t, pool.Contains(base.Address, quote.Address))
				assert.Positive(t, pool.SqrtPriceX96.Sign(), "pool %s has no price", pool.Address.Hex())

				// Uniswap sorts the pair, so token0 is always the lower address
				assert.Negative(t, bytesCompare(pool.Token0, pool.Token1),
					"token0 must be the lower address")

				t.Logf("%s fee=%.2f%% liquidity=%s", pool.Address.Hex(),
					v3.FeeTierFraction(pool.Fee)*100, pool.Liquidity)
			}

			deepest := pools[0]

			// the spot price of the deepest pool, oriented base -> quote
			decimals0, decimals1 := base.Decimals, quote.Decimals
			if deepest.Token0 != base.Address {
				decimals0, decimals1 = quote.Decimals, base.Decimals
			}

			spot := deepest.Price(decimals0, decimals1)
			spotFloat, _ := spot.Float64()

			if deepest.Token0 != base.Address {
				spotFloat = 1 / spotFloat
			}

			t.Logf("spot: 1 %s = %.6f %s", base.Symbol, spotFloat, quote.Symbol)
			assert.Positive(t, spotFloat)

			// one unit of the base token, which is small enough everywhere here
			// that the quote should land close to spot
			amountIn, err := token.ParseAmount("1", base.Decimals)
			require.NoError(t, err)

			best, err := dex.BestQuote(ctx, base.Address, quote.Address, amountIn)
			require.NoError(t, err)
			require.Positive(t, best.AmountOut.Sign())

			realized, _ := best.Price(base.Decimals, quote.Decimals).Float64()
			t.Logf("quote: 1 %s -> %s %s at fee %.2f%% (ticks crossed %d)",
				base.Symbol, quote.FormatRaw(best.AmountOut), quote.Symbol,
				v3.FeeTierFraction(best.Fee)*100, best.InitializedTicksCrossed)

			// a small trade must price within a few percent of spot; a decimals
			// or a sqrtPriceX96 mistake is orders of magnitude off
			assert.InEpsilon(t, spotFloat, realized, 0.05,
				"the quoted price %.6f is far from the spot price %.6f", realized, spotFloat)

			// the exact-output quote must round-trip to roughly the same size
			out, err := dex.QuoteExactOut(ctx, base.Address, quote.Address, best.Fee, best.AmountOut)
			require.NoError(t, err)

			roundTrip := new(big.Float).Quo(
				new(big.Float).SetInt(out.AmountIn), new(big.Float).SetInt(amountIn))
			ratio, _ := roundTrip.Float64()
			assert.InDelta(t, 1.0, ratio, 0.01, "the exact-output quote disagrees with the exact-input one")

			// the slippage bound is below the quote and above zero
			minOut := best.AmountOutMinimum(50)
			assert.Negative(t, minOut.Cmp(best.AmountOut))
			assert.Positive(t, minOut.Sign())

			// a pair with no pool must be reported as such rather than as a
			// zero quote
			nonsense := common.HexToAddress("0x000000000000000000000000000000000000dEaD")
			_, err = dex.FindPool(ctx, base.Address, nonsense)
			assert.ErrorIs(t, err, v3.ErrNoPool)
		})
	}
}

func bytesCompare(a, b common.Address) int {
	return strings.Compare(strings.ToLower(a.Hex()), strings.ToLower(b.Hex()))
}
