package v3_test

import (
	"context"
	"math"
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
	v3 "github.com/c9s/bbgo/pkg/defi/aave/v3"
)

// aaveV3Launch predates every v3 deployment, so any reserve timestamp must be
// after it.
var aaveV3Launch = time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)

// TestIntegration validates the hardcoded provider addresses and every ABI in
// this package against a live market. It needs no API key and no private key:
//
//	TEST_EVM_CHAIN=bsc TEST_EVM_RPC_ENDPOINTS=https://... go test ./pkg/defi/aave/v3 -run TestIntegration -v
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

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	client, err := evm.NewClient(ctx, chain, strings.Split(endpoints, ","))
	require.NoError(t, err)

	defer client.Close()

	registry := token.NewRegistry(client, chain.Multicall3)
	registry.AddKnownSymbols(uint64(chain.ID))

	aave, err := v3.NewClient(ctx, chain, client, v3.Options{Tokens: registry})
	require.NoError(t, err)

	addresses := aave.Addresses()
	t.Logf("%s", addresses)

	assert.NotEqual(t, common.Address{}, addresses.Pool)
	assert.NotEqual(t, common.Address{}, addresses.DataProvider)
	assert.NotEqual(t, common.Address{}, addresses.Oracle)

	// every v3 market prices in USD with 8 decimals
	assert.Equal(t, 8, addresses.BaseCurrencyDecimals())

	assets, err := aave.ReservesList(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, assets)
	t.Logf("%d reserves listed", len(assets))

	// the market wide state of every reserve, in one batched call
	reserves, err := aave.Reserves(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, reserves)

	prices, err := aave.AssetPrices(ctx, assets)
	require.NoError(t, err)

	var totalSupplied float64

	for _, reserve := range reserves {
		asset, err := registry.Lookup(ctx, reserve.Asset)
		require.NoError(t, err)

		supplyAPY := reserve.SupplyAPY()
		borrowAPY := reserve.BorrowAPY()

		// rates are per-second RAY values: a sane market is far below 1000% APY
		assert.GreaterOrEqual(t, supplyAPY, 0.0, "%s supply apy", asset.Symbol)
		assert.Less(t, supplyAPY, 10.0, "%s supply apy looks like a unit error", asset.Symbol)
		assert.GreaterOrEqual(t, borrowAPY, 0.0, "%s borrow apy", asset.Symbol)
		assert.Less(t, borrowAPY, 10.0, "%s borrow apy looks like a unit error", asset.Symbol)

		// the liquidity index only grows from its 1.0 starting point
		assert.GreaterOrEqual(t, v3.RayToFloat(reserve.LiquidityIndex), 1.0, "%s liquidity index", asset.Symbol)

		// a dormant reserve can be months stale, so this only checks that the
		// uint40 timestamp was decoded at the right scale
		assert.True(t, reserve.LastUpdated.After(aaveV3Launch), "%s was last updated at %s",
			asset.Symbol, reserve.LastUpdated)
		assert.True(t, reserve.LastUpdated.Before(time.Now().Add(24*time.Hour)),
			"%s was last updated in the future, at %s", asset.Symbol, reserve.LastUpdated)

		price := prices[reserve.Asset]
		require.NotNil(t, price, "%s has no oracle price", asset.Symbol)
		assert.Positive(t, price.Sign(), "%s oracle price", asset.Symbol)

		supplied := asset.ToFixedPointOrZero(reserve.TotalAToken)
		priceUSD, _ := new(big.Float).Quo(
			new(big.Float).SetInt(price), new(big.Float).SetInt(addresses.BaseCurrencyUnit)).Float64()

		totalSupplied += supplied * priceUSD

		t.Logf("%-8s supplied=%-18s debt=%-18s supplyAPY=%6.3f%% borrowAPY=%6.3f%% price=%.4f",
			asset.Symbol,
			asset.FormatRaw(reserve.TotalAToken),
			asset.FormatRaw(reserve.TotalDebt()),
			supplyAPY*100, borrowAPY*100, priceUSD)
	}

	t.Logf("total supplied across the market: %.0f USD", totalSupplied)
	assert.Positive(t, totalSupplied, "the market should hold some liquidity")

	// the reserve configuration of the first listed asset
	config, err := aave.ReserveConfig(ctx, assets[0])
	require.NoError(t, err)
	assert.Positive(t, config.Decimals)
	assert.LessOrEqual(t, v3.BpsToFloat(config.LTVBps), 1.0)
	assert.LessOrEqual(t, v3.BpsToFloat(config.LTVBps), v3.BpsToFloat(config.LiquidationThresholdBps)+1e-9,
		"the ltv must not exceed the liquidation threshold")
	t.Logf("first reserve config: %+v", *config)

	tokens, err := aave.ReserveTokens(ctx, assets[0])
	require.NoError(t, err)
	assert.NotEqual(t, common.Address{}, tokens.AToken)
	assert.NotEqual(t, common.Address{}, tokens.VariableDebtToken)

	// the burn address never borrows, and Aave reports an account without debt
	// as having an infinite health factor
	burned := common.HexToAddress("0x000000000000000000000000000000000000dEaD")

	account, err := aave.AccountData(ctx, burned)
	require.NoError(t, err)
	assert.False(t, account.HasDebt())
	assert.True(t, math.IsInf(account.HealthFactorFloat(), 1), "an account with no debt has an infinite health factor")
	assert.False(t, account.IsLiquidatable())
	t.Logf("burn address account: %s", account)

	// aTokens do get sent to the burn address, so this only checks the filter:
	// every returned position must actually hold something
	positions, err := aave.UserReserves(ctx, burned)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(positions), len(assets))

	for _, position := range positions {
		assert.False(t, position.IsEmpty(), "%s was returned but is empty", position.Asset.Hex())

		asset, err := registry.Lookup(ctx, position.Asset)
		require.NoError(t, err)
		t.Logf("burn address holds %s %s (debt %s)",
			asset.FormatRaw(position.ATokenBalance), asset.Symbol, asset.FormatRaw(position.TotalDebt()))
	}

	// the pool itself is read-only here
	assert.False(t, aave.CanWrite())
}
