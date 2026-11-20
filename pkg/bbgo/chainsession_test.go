package bbgo

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/c9s/bbgo/pkg/chain/evm"
	"github.com/c9s/bbgo/pkg/chain/evm/stream"
	"github.com/c9s/bbgo/pkg/chain/evm/token"
	"github.com/c9s/bbgo/pkg/types"
)

// the well-known Hardhat / Anvil test account #0, never used for real funds
const testPrivateKey = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
const testPrivateKeyAddress = "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"

func TestEnvVarName(t *testing.T) {
	assert.Equal(t, "ETH_PRIVATE_KEY", envVarName("eth", "PRIVATE_KEY"))
	assert.Equal(t, "MY_CHAIN_RPC_URL", envVarName("my-chain", "RPC_URL"))
	assert.Equal(t, "BSC_RPC_ENDPOINTS", envVarName("BSC", "RPC_ENDPOINTS"))
}

func TestChainSessionInitUnknownChain(t *testing.T) {
	session := &ChainSession{Chain: "solana"}
	err := session.Init(context.Background(), "sol")
	assert.ErrorContains(t, err, "unknown chain")
}

func TestChainSessionInitWithoutEndpoint(t *testing.T) {
	session := &ChainSession{Chain: "ethereum"}
	err := session.Init(context.Background(), "eth")
	assert.ErrorContains(t, err, "ETH_RPC_ENDPOINTS")
}

func TestChainSessionResolveEndpoints(t *testing.T) {
	session := &ChainSession{Chain: "bsc", RPCEndpoints: []string{"https://configured.example"}}
	assert.Equal(t, []string{"https://configured.example"}, session.resolveEndpoints())

	// the session name takes precedence over the chain name
	t.Setenv("MYBSC_RPC_ENDPOINTS", "https://a.example,https://b.example")
	t.Setenv("BSC_RPC_URL", "https://chain.example")

	session = &ChainSession{Name: "mybsc", Chain: "bsc"}
	assert.Equal(t, []string{"https://a.example", "https://b.example"}, session.resolveEndpoints())

	chain, ok := evm.LookupChain("bsc")
	assert.True(t, ok)

	// and the chain name is the last resort
	session = &ChainSession{Name: "other", Chain: "bsc", chain: chain}
	assert.Equal(t, []string{"https://chain.example"}, session.resolveEndpoints())
}

func TestChainSessionResolveWSEndpoint(t *testing.T) {
	session := &ChainSession{Chain: "bsc", WSEndpoint: " wss://configured.example "}
	assert.Equal(t, "wss://configured.example", session.resolveWSEndpoint())

	session = &ChainSession{Name: "nows", Chain: "bsc"}
	assert.Equal(t, "", session.resolveWSEndpoint())

	// the session name takes precedence over the chain name
	t.Setenv("MYBSC_WS_ENDPOINT", "wss://session.example")
	t.Setenv("BSC_WS_ENDPOINT", "wss://chain.example")

	session = &ChainSession{Name: "mybsc", Chain: "bsc"}
	assert.Equal(t, "wss://session.example", session.resolveWSEndpoint())

	chain, ok := evm.LookupChain("bsc")
	assert.True(t, ok)

	session = &ChainSession{Name: "other", Chain: "bsc", chain: chain}
	assert.Equal(t, "wss://chain.example", session.resolveWSEndpoint())
}

func TestChainSessionNewStreamRequiresInit(t *testing.T) {
	_, err := (&ChainSession{Name: "bsc", Chain: "bsc"}).NewStream(stream.Config{})
	assert.Error(t, err)
}

func TestChainSessionLoadSigner(t *testing.T) {
	t.Run("read-only without a key", func(t *testing.T) {
		session := &ChainSession{Name: "eth", Chain: "ethereum", Address: testPrivateKeyAddress}
		assert.NoError(t, session.loadSigner())
		assert.False(t, session.CanSign())
		assert.True(t, session.HasAddress())
		assert.Equal(t, testPrivateKeyAddress, session.AddressHex())
	})

	t.Run("no key and no address", func(t *testing.T) {
		session := &ChainSession{Name: "eth", Chain: "ethereum"}
		assert.NoError(t, session.loadSigner())
		assert.False(t, session.HasAddress())
		assert.Equal(t, "(none)", session.AddressHex())
	})

	t.Run("derives the address from the key", func(t *testing.T) {
		t.Setenv("ETH_PRIVATE_KEY", testPrivateKey)

		session := &ChainSession{Name: "eth", Chain: "ethereum"}
		assert.NoError(t, session.loadSigner())
		assert.True(t, session.CanSign())
		assert.Equal(t, testPrivateKeyAddress, session.AddressHex())
	})

	t.Run("accepts the 0x prefix and a custom var", func(t *testing.T) {
		t.Setenv("MY_KEY", "0x"+testPrivateKey)

		session := &ChainSession{Name: "eth", Chain: "ethereum", PrivateKeyVar: "MY_KEY"}
		assert.NoError(t, session.loadSigner())
		assert.Equal(t, testPrivateKeyAddress, session.AddressHex())
	})

	t.Run("rejects an address that disagrees with the key", func(t *testing.T) {
		t.Setenv("ETH_PRIVATE_KEY", testPrivateKey)

		session := &ChainSession{
			Name:    "eth",
			Chain:   "ethereum",
			Address: "0x1111111111111111111111111111111111111111",
		}

		err := session.loadSigner()
		assert.ErrorContains(t, err, "does not match")
	})

	t.Run("rejects an invalid key", func(t *testing.T) {
		t.Setenv("ETH_PRIVATE_KEY", "not-a-key")

		session := &ChainSession{Name: "eth", Chain: "ethereum"}
		err := session.loadSigner()
		assert.ErrorContains(t, err, "ETH_PRIVATE_KEY")

		// the key material must never leak into the error message
		assert.NotContains(t, err.Error(), "not-a-key")
	})

	t.Run("rejects an invalid address", func(t *testing.T) {
		session := &ChainSession{Name: "eth", Chain: "ethereum", Address: "0xnope"}
		assert.Error(t, session.loadSigner())
	})
}

func TestChainSessionConfigUnmarshal(t *testing.T) {
	const configYAML = `
chainSessions:
  eth:
    chain: ethereum
    rpcEndpoints:
      - https://rpc.example/one
      - https://rpc.example/two
    confirmations: 5
    tokens:
      MYTOKEN: "0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599"
  bsc:
    chain: bsc
    address: "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"
`

	var config Config
	assert.NoError(t, yaml.Unmarshal([]byte(configYAML), &config))
	assert.Len(t, config.ChainSessions, 2)

	eth := config.ChainSessions["eth"]
	if assert.NotNil(t, eth) {
		assert.Equal(t, "ethereum", eth.Chain)
		assert.Equal(t, []string{"https://rpc.example/one", "https://rpc.example/two"}, eth.RPCEndpoints)
		assert.Equal(t, 5, eth.Confirmations)
		assert.Equal(t, "0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599", eth.Tokens["MYTOKEN"])
	}

	bsc := config.ChainSessions["bsc"]
	if assert.NotNil(t, bsc) {
		assert.Equal(t, "bsc", bsc.Chain)
		assert.Equal(t, testPrivateKeyAddress, bsc.Address)
	}
}

func TestChainSessionMap(t *testing.T) {
	sessions := ChainSessionMap{
		"polygon": {Chain: "polygon"},
		"eth":     {Chain: "ethereum"},
	}

	assert.Equal(t, []string{"eth", "polygon"}, sessions.Names())
	assert.Equal(t, []string{"eth"}, sessions.Filter([]string{"eth", "missing"}).Names())
}

func TestChainTxConfigSenderConfig(t *testing.T) {
	// a nil config still carries the session confirmations through
	var nilConfig *ChainTxConfig
	senderConfig := nilConfig.SenderConfig(7)
	assert.Equal(t, 7, senderConfig.Confirmations)
	assert.Zero(t, senderConfig.BumpInterval)

	config := &ChainTxConfig{
		Fee:                evm.FeeConfig{MaxFeePerGasGwei: 50, BumpPercent: 20},
		GasLimitMultiplier: 1.5,
		BumpInterval:       types.Duration(30 * time.Second),
		PollInterval:       types.Duration(3 * time.Second),
		Timeout:            types.Duration(5 * time.Minute),
		PrivateRelay:       "https://relay.example",
	}

	senderConfig = config.SenderConfig(12)
	assert.Equal(t, 12, senderConfig.Confirmations)
	assert.Equal(t, 1.5, senderConfig.GasLimitMultiplier)
	assert.Equal(t, 30*time.Second, senderConfig.BumpInterval)
	assert.Equal(t, 3*time.Second, senderConfig.PollInterval)
	assert.Equal(t, 5*time.Minute, senderConfig.Timeout)
	assert.Equal(t, "https://relay.example", senderConfig.PrivateRelay)
	assert.Equal(t, 20, senderConfig.Fee.BumpPercent)
	assert.Equal(t, big.NewInt(50_000_000_000), senderConfig.Fee.MaxFeePerGasWei())
}

func TestChainSessionTxConfigUnmarshal(t *testing.T) {
	const configYAML = `
chainSessions:
  eth:
    chain: ethereum
    rpcEndpoints: [ "https://rpc.example/one" ]
    tx:
      gasLimitMultiplier: 1.3
      bumpInterval: 1m
      timeout: 10m
      fee:
        maxFeePerGasGwei: 120
        bumpPercent: 25
      allowance:
        policy: unlimited
        resetBeforeApprove: true
`

	var config Config
	require.NoError(t, yaml.Unmarshal([]byte(configYAML), &config))

	session := config.ChainSessions["eth"]
	require.NotNil(t, session)
	require.NotNil(t, session.Tx)

	assert.Equal(t, 1.3, session.Tx.GasLimitMultiplier)
	assert.Equal(t, time.Minute, session.Tx.BumpInterval.Duration())
	assert.Equal(t, 10*time.Minute, session.Tx.Timeout.Duration())
	assert.Equal(t, 120.0, session.Tx.Fee.MaxFeePerGasGwei)
	assert.Equal(t, 25, session.Tx.Fee.BumpPercent)
	assert.Equal(t, token.AllowanceUnlimited, session.Tx.Allowance.Policy)
	assert.True(t, session.Tx.Allowance.ResetBeforeApprove)

	assert.Equal(t, token.AllowanceUnlimited, session.AllowanceConfig().ResolvedPolicy())
}

func TestChainSessionReadOnlyHasNoSender(t *testing.T) {
	session := &ChainSession{Name: "eth", Chain: "ethereum", Address: testPrivateKeyAddress}
	assert.Nil(t, session.Sender())
	assert.Equal(t, token.AllowanceExact, session.AllowanceConfig().ResolvedPolicy())
}
