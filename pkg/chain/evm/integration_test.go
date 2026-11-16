package evm_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/chain/evm"
	"github.com/c9s/bbgo/pkg/chain/evm/token"
)

// TestIntegration exercises the client, the token registry and the Multicall3
// batching against a live RPC endpoint. It needs no API key and no private key,
// only a public endpoint:
//
//	TEST_EVM_CHAIN=bsc TEST_EVM_RPC_ENDPOINTS=https://... go test ./pkg/chain/evm -run TestIntegration -v
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := evm.NewClient(ctx, chain, strings.Split(endpoints, ","))
	require.NoError(t, err)

	defer client.Close()

	blockNumber, err := client.BlockNumber(ctx)
	require.NoError(t, err)
	assert.NotZero(t, blockNumber)
	t.Logf("chain=%s chainID=%d block=%d", chain.Name, uint64(chain.ID), blockNumber)

	// the wrapped native token is a contract that exists on every chain here,
	// and reading it exercises the batched metadata lookup
	registry := token.NewRegistry(client, chain.Multicall3)
	registry.AddKnownSymbols(uint64(chain.ID))

	wrapped, err := registry.Resolve(ctx, chain.WrappedNative)
	require.NoError(t, err)
	assert.Equal(t, 18, wrapped.Decimals)
	t.Logf("wrapped native: %s", wrapped)

	// decimals must come from the chain: the same symbol differs per chain
	if usdt, err := registry.Resolve(ctx, "USDT"); err == nil {
		t.Logf("USDT: %s", usdt)

		switch chain.ID {
		case evm.ChainIDEthereum, evm.ChainIDPolygon:
			assert.Equal(t, 6, usdt.Decimals)
		case evm.ChainIDBSC:
			assert.Equal(t, 18, usdt.Decimals)
		}
	}

	// a batched balance read over every known token of this chain
	tokens, err := registry.ResolveAll(ctx, registry.Symbols())
	require.NoError(t, err)

	// the wrapped native contract holds its own native deposits, so its own
	// balance of itself is a stable non-empty read target
	balances, err := registry.Balances(ctx, wrapped.Address, tokens)
	require.NoError(t, err)
	assert.NotEmpty(t, balances)

	for _, balance := range balances {
		if balance.Raw.Sign() > 0 {
			t.Logf("balance: %s", balance)
		}
	}

	nativeBalance, err := client.NativeBalance(ctx, wrapped.Address)
	require.NoError(t, err)
	assert.Positive(t, nativeBalance.Sign(), "the wrapped native contract should hold native balance")
	t.Logf("native: %s %s", token.FormatRaw(nativeBalance, chain.NativeDecimals), chain.NativeSymbol)

	// a contract that does not exist must fail cleanly rather than silently
	_, err = registry.Lookup(ctx, common.HexToAddress("0x000000000000000000000000000000000000dEaD"))
	assert.Error(t, err)
}
