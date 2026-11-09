package evm_test

import (
	"context"
	"fmt"
	"math/big"
	"net"
	"os/exec"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/chain/evm"
)

// the first two accounts of the standard anvil / hardhat mnemonic
const (
	anvilKey0     = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	anvilAccount1 = "0x70997970C51812dc3A010C7d01b50e0d17dc79C8"
)

// startAnvil boots a local anvil node pretending to be BSC, so the chain
// registry entry applies. The test is skipped when foundry is not installed:
//
//	curl -L https://foundry.paradigm.xyz | bash && foundryup
func startAnvil(t *testing.T) string {
	t.Helper()

	binary, err := exec.LookPath("anvil")
	if err != nil {
		t.Skip("anvil is not installed, install foundry to run this test")
	}

	port, err := freePort()
	require.NoError(t, err)

	endpoint := fmt.Sprintf("http://127.0.0.1:%d", port)

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, binary,
		"--port", fmt.Sprintf("%d", port),
		"--chain-id", "56",
		"--silent",
	)

	require.NoError(t, cmd.Start())

	t.Cleanup(func() {
		cancel()
		_ = cmd.Wait()
	})

	// wait for the RPC port to accept connections
	deadline := time.Now().Add(20 * time.Second)
	for {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 200*time.Millisecond)
		if err == nil {
			conn.Close()
			return endpoint
		}

		if time.Now().After(deadline) {
			t.Fatalf("anvil did not start listening on port %d", port)
		}

		time.Sleep(100 * time.Millisecond)
	}
}

func freePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}

	defer listener.Close()

	return listener.Addr().(*net.TCPAddr).Port, nil
}

// TestAnvilSendTransaction exercises the whole lifecycle against a real node:
// nonce allocation, simulation, gas estimation, fee pricing, signing,
// broadcasting and confirmation.
func TestAnvilSendTransaction(t *testing.T) {
	endpoint := startAnvil(t)

	chain, ok := evm.LookupChain("bsc")
	require.True(t, ok)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, err := evm.NewClient(ctx, chain, []string{endpoint})
	require.NoError(t, err)

	defer client.Close()

	signer, err := evm.NewPrivateKeySigner(anvilKey0)
	require.NoError(t, err)

	sender := evm.NewSender(client, signer, evm.SenderConfig{
		Confirmations: 1,
		PollInterval:  time.Second,
		Timeout:       30 * time.Second,
	})

	recipient := common.HexToAddress(anvilAccount1)

	before, err := client.NativeBalance(ctx, recipient)
	require.NoError(t, err)

	amount := big.NewInt(1_000_000_000_000_000) // 0.001 native

	result, err := sender.SendAndWait(ctx, evm.SendRequest{
		To:    &recipient,
		Value: amount,
		Label: "anvil transfer",
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.True(t, result.Succeeded())
	assert.Equal(t, uint64(0), result.Nonce)
	assert.Equal(t, uint64(21000), result.GasUsed)
	assert.Positive(t, result.GasCost.Sign())
	assert.Empty(t, result.ReplacedHashes)
	t.Logf("result: %s (finalizedByTag=%v)", result, result.FinalizedByTag)

	after, err := client.NativeBalance(ctx, recipient)
	require.NoError(t, err)
	assert.Equal(t, amount, new(big.Int).Sub(after, before))

	// a second transaction must take the next nonce
	second, err := sender.SendAndWait(ctx, evm.SendRequest{
		To:    &recipient,
		Value: amount,
		Label: "anvil transfer 2",
	})
	require.NoError(t, err)
	assert.Equal(t, uint64(1), second.Nonce)
	assert.True(t, second.Succeeded())
}

// TestAnvilRejectedBroadcastReleasesTheNonce uses a gas limit below the
// intrinsic cost, which every node rejects deterministically, and checks that
// the allocated nonce is handed back so the next transaction is not stuck.
func TestAnvilRejectedBroadcastReleasesTheNonce(t *testing.T) {
	endpoint := startAnvil(t)

	chain, _ := evm.LookupChain("bsc")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, err := evm.NewClient(ctx, chain, []string{endpoint})
	require.NoError(t, err)

	defer client.Close()

	signer, err := evm.NewPrivateKeySigner(anvilKey0)
	require.NoError(t, err)

	sender := evm.NewSender(client, signer, evm.SenderConfig{Confirmations: 1, PollInterval: time.Second})

	recipient := common.HexToAddress(anvilAccount1)

	_, err = sender.Submit(ctx, evm.SendRequest{
		To:       &recipient,
		Value:    big.NewInt(1),
		GasLimit: 1, // below the 21000 intrinsic gas of a transfer
		Label:    "intrinsic gas too low",
	})
	assert.Error(t, err)
	t.Logf("broadcast rejected as expected: %v", err)

	// the nonce was never consumed, so the next transaction still uses 0
	nonce, err := sender.Nonces().Next(ctx)
	require.NoError(t, err)
	assert.Equal(t, uint64(0), nonce)
}
