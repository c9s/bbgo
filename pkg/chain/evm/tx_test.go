package evm

import (
	"context"
	"errors"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// the well-known Hardhat / Anvil test account #0, never used for real funds
const testSignerKey = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"

// fakeTxClient is an in-memory stand-in for the RPC layer, enough to drive the
// whole transaction lifecycle without a node.
type fakeTxClient struct {
	chain *Chain

	mu sync.Mutex

	nonce    uint64
	gas      uint64
	gasPrice *big.Int
	baseFee  *big.Int
	rewards  [][]*big.Int

	head      uint64
	finalized uint64

	callErr      error
	gasErr       error
	finalizedErr error

	sent     []*ethtypes.Transaction
	receipts map[common.Hash]*ethtypes.Receipt

	// sendHook runs instead of the default broadcast behaviour, so a test can
	// mine, stall or reject a transaction.
	sendHook func(f *fakeTxClient, tx *ethtypes.Transaction) error
}

func newFakeTxClient(chainName string) *fakeTxClient {
	chain, _ := LookupChain(chainName)

	return &fakeTxClient{
		chain:     chain,
		gas:       21000,
		gasPrice:  gwei(3),
		baseFee:   gwei(10),
		rewards:   rewardsOf(2),
		head:      100,
		finalized: 100,
		receipts:  map[common.Hash]*ethtypes.Receipt{},
	}
}

func (f *fakeTxClient) Chain() *Chain {
	return f.chain
}

func (f *fakeTxClient) PendingNonceAt(_ context.Context, _ common.Address) (uint64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.nonce, nil
}

func (f *fakeTxClient) HeaderByNumber(_ context.Context, number *big.Int) (*ethtypes.Header, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if number != nil && number.Sign() < 0 {
		if f.finalizedErr != nil {
			return nil, f.finalizedErr
		}

		return &ethtypes.Header{Number: new(big.Int).SetUint64(f.finalized)}, nil
	}

	return &ethtypes.Header{Number: new(big.Int).SetUint64(f.head), BaseFee: f.baseFee}, nil
}

func (f *fakeTxClient) FeeHistory(
	_ context.Context, _ uint64, _ *big.Int, _ []float64,
) (*ethereum.FeeHistory, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return &ethereum.FeeHistory{Reward: f.rewards}, nil
}

func (f *fakeTxClient) SuggestGasPrice(_ context.Context) (*big.Int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.gasPrice, nil
}

func (f *fakeTxClient) EstimateGas(_ context.Context, _ ethereum.CallMsg) (uint64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.gasErr != nil {
		return 0, f.gasErr
	}

	return f.gas, nil
}

func (f *fakeTxClient) CallContract(_ context.Context, _ ethereum.CallMsg, _ *big.Int) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.callErr != nil {
		return nil, f.callErr
	}

	return nil, nil
}

func (f *fakeTxClient) SendTransaction(_ context.Context, tx *ethtypes.Transaction) error {
	f.mu.Lock()
	f.sent = append(f.sent, tx)
	hook := f.sendHook
	f.mu.Unlock()

	if hook != nil {
		return hook(f, tx)
	}

	f.mine(tx, ethtypes.ReceiptStatusSuccessful)

	return nil
}

func (f *fakeTxClient) TransactionReceipt(_ context.Context, hash common.Hash) (*ethtypes.Receipt, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if receipt, ok := f.receipts[hash]; ok {
		return receipt, nil
	}

	return nil, ethereum.NotFound
}

func (f *fakeTxClient) BlockNumber(_ context.Context) (uint64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.head, nil
}

// mine records a receipt for tx in the current head block.
func (f *fakeTxClient) mine(tx *ethtypes.Transaction, status uint64) {
	f.mu.Lock()
	defer f.mu.Unlock()

	price := tx.GasPrice()
	if tx.Type() == ethtypes.DynamicFeeTxType {
		price = new(big.Int).Add(f.baseFee, tx.GasTipCap())
	}

	f.receipts[tx.Hash()] = &ethtypes.Receipt{
		TxHash:            tx.Hash(),
		BlockNumber:       new(big.Int).SetUint64(f.head),
		BlockHash:         common.HexToHash("0xb10c"),
		Status:            status,
		GasUsed:           21000,
		EffectiveGasPrice: price,
	}
}

func (f *fakeTxClient) sentCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return len(f.sent)
}

func (f *fakeTxClient) sentTx(i int) *ethtypes.Transaction {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.sent[i]
}

// fastPolling shrinks the lifecycle intervals so the state machine can be
// driven in milliseconds.
func fastPolling(t *testing.T) {
	t.Helper()

	previous := minPollInterval
	minPollInterval = time.Millisecond
	t.Cleanup(func() { minPollInterval = previous })
}

func testSenderConfig() SenderConfig {
	return SenderConfig{
		PollInterval: 2 * time.Millisecond,
		BumpInterval: 20 * time.Millisecond,
		Timeout:      5 * time.Second,
	}
}

func newTestSender(t *testing.T, client *fakeTxClient, config SenderConfig) *Sender {
	t.Helper()

	signer, err := NewPrivateKeySigner(testSignerKey)
	require.NoError(t, err)

	return NewSender(client, signer, config)
}

var testRecipient = common.HexToAddress("0x1111111111111111111111111111111111111111")

func testRequest() SendRequest {
	to := testRecipient

	return SendRequest{To: &to, Value: big.NewInt(1000), Label: "test"}
}

func TestSenderSendAndWait(t *testing.T) {
	fastPolling(t)

	ctx := context.Background()
	client := newFakeTxClient("bsc")
	sender := newTestSender(t, client, testSenderConfig())

	result, err := sender.SendAndWait(ctx, testRequest())
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.True(t, result.Succeeded())
	assert.Equal(t, uint64(0), result.Nonce)
	assert.Equal(t, uint64(100), result.BlockNumber)
	assert.Equal(t, uint64(21000), result.GasUsed)
	assert.Equal(t, new(big.Int).Mul(big.NewInt(21000), gwei(3)), result.GasCost)
	assert.True(t, result.FinalizedByTag, "bsc supports the finalized tag")
	assert.Empty(t, result.ReplacedHashes)

	// BSC is priced as a legacy chain, and the gas limit is padded
	require.Equal(t, 1, client.sentCount())
	sent := client.sentTx(0)
	assert.Equal(t, ethtypes.LegacyTxType, int(sent.Type()))
	assert.Equal(t, gwei(3), sent.GasPrice())
	assert.Equal(t, uint64(25200), sent.Gas())
	assert.Equal(t, uint64(0), sent.Nonce())
	assert.Equal(t, result.Hash, sent.Hash())
}

func TestSenderSendsDynamicFeeTxOnEIP1559Chain(t *testing.T) {
	fastPolling(t)

	client := newFakeTxClient("ethereum")
	sender := newTestSender(t, client, testSenderConfig())

	result, err := sender.SendAndWait(context.Background(), testRequest())
	require.NoError(t, err)
	assert.True(t, result.Succeeded())

	sent := client.sentTx(0)
	assert.Equal(t, ethtypes.DynamicFeeTxType, int(sent.Type()))
	assert.Equal(t, gwei(2), sent.GasTipCap())
	assert.Equal(t, gwei(22), sent.GasFeeCap())
}

func TestSenderSimulationRevertReleasesTheNonce(t *testing.T) {
	fastPolling(t)

	ctx := context.Background()
	client := newFakeTxClient("bsc")
	client.callErr = errors.New("execution reverted: STF")
	client.nonce = 4

	sender := newTestSender(t, client, testSenderConfig())

	_, err := sender.Submit(ctx, testRequest())
	assert.ErrorIs(t, err, ErrReverted)
	assert.ErrorContains(t, err, "STF")
	assert.Zero(t, client.sentCount(), "nothing should be broadcast")

	// the allocated nonce is handed back, so the next transaction reuses it
	nonce, err := sender.Nonces().Next(ctx)
	assert.NoError(t, err)
	assert.Equal(t, uint64(4), nonce)
}

func TestSenderGasEstimationRevert(t *testing.T) {
	fastPolling(t)

	client := newFakeTxClient("bsc")
	client.gasErr = errors.New("execution reverted: TRANSFER_FROM_FAILED")

	sender := newTestSender(t, client, testSenderConfig())

	_, err := sender.Submit(context.Background(), testRequest())
	assert.ErrorIs(t, err, ErrReverted)
	assert.ErrorContains(t, err, "TRANSFER_FROM_FAILED")
}

func TestSenderRevertedOnChain(t *testing.T) {
	fastPolling(t)

	client := newFakeTxClient("bsc")
	client.sendHook = func(f *fakeTxClient, tx *ethtypes.Transaction) error {
		f.mine(tx, ethtypes.ReceiptStatusFailed)
		return nil
	}

	sender := newTestSender(t, client, testSenderConfig())

	result, err := sender.SendAndWait(context.Background(), testRequest())
	assert.ErrorIs(t, err, ErrTxReverted)

	// the result is still returned, so the caller can account for the gas spent
	require.NotNil(t, result)
	assert.False(t, result.Succeeded())
	assert.Equal(t, uint64(21000), result.GasUsed)
	assert.Positive(t, result.GasCost.Sign())
}

func TestSenderBumpsStuckTransaction(t *testing.T) {
	fastPolling(t)

	client := newFakeTxClient("bsc")
	client.sendHook = func(f *fakeTxClient, tx *ethtypes.Transaction) error {
		// the first broadcast stays pending; the replacement is mined
		if f.sentCount() < 2 {
			return nil
		}

		f.mine(tx, ethtypes.ReceiptStatusSuccessful)

		return nil
	}

	sender := newTestSender(t, client, testSenderConfig())

	result, err := sender.SendAndWait(context.Background(), testRequest())
	require.NoError(t, err)

	require.Equal(t, 2, client.sentCount())
	first, second := client.sentTx(0), client.sentTx(1)

	// same nonce, higher fee
	assert.Equal(t, first.Nonce(), second.Nonce())
	assert.Positive(t, second.GasPrice().Cmp(first.GasPrice()))

	assert.Equal(t, second.Hash(), result.Hash)
	assert.Equal(t, []common.Hash{first.Hash()}, result.ReplacedHashes)
}

func TestSenderStopsBumpingAtTheFeeCap(t *testing.T) {
	fastPolling(t)

	client := newFakeTxClient("bsc")
	client.sendHook = func(f *fakeTxClient, tx *ethtypes.Transaction) error {
		// never mined until the test mines it explicitly
		return nil
	}

	config := testSenderConfig()
	// the cap allows the initial 3 gwei but no bump at all
	config.Fee.MaxFeePerGasGwei = 3.2

	sender := newTestSender(t, client, config)

	pending, err := sender.Submit(context.Background(), testRequest())
	require.NoError(t, err)

	// give the watcher time to try bumping and give up
	time.Sleep(80 * time.Millisecond)
	assert.Equal(t, 1, client.sentCount(), "the bump must be refused by the cap")

	// once it is finally mined the original transaction still confirms
	client.mine(client.sentTx(0), ethtypes.ReceiptStatusSuccessful)

	result, err := pending.Wait(context.Background())
	require.NoError(t, err)
	assert.Equal(t, client.sentTx(0).Hash(), result.Hash)
}

func TestSenderSerializesTransactions(t *testing.T) {
	fastPolling(t)

	ctx := context.Background()
	client := newFakeTxClient("bsc")

	release := make(chan struct{})
	client.sendHook = func(f *fakeTxClient, tx *ethtypes.Transaction) error {
		if tx.Nonce() == 0 {
			go func() {
				<-release
				f.mine(tx, ethtypes.ReceiptStatusSuccessful)
			}()

			return nil
		}

		f.mine(tx, ethtypes.ReceiptStatusSuccessful)

		return nil
	}

	sender := newTestSender(t, client, testSenderConfig())

	first, err := sender.Submit(ctx, testRequest())
	require.NoError(t, err)
	assert.Equal(t, uint64(0), first.Nonce())

	submitted := make(chan *PendingTx, 1)
	go func() {
		second, err := sender.Submit(ctx, testRequest())
		if err != nil {
			close(submitted)
			return
		}

		submitted <- second
	}()

	// the second submission must wait for the first to reach a terminal state
	select {
	case <-submitted:
		t.Fatal("the second transaction was submitted while the first was still in flight")
	case <-time.After(30 * time.Millisecond):
	}

	close(release)

	firstResult, err := first.Wait(ctx)
	require.NoError(t, err)
	assert.True(t, firstResult.Succeeded())

	second := <-submitted
	require.NotNil(t, second)
	assert.Equal(t, uint64(1), second.Nonce())

	secondResult, err := second.Wait(ctx)
	require.NoError(t, err)
	assert.True(t, secondResult.Succeeded())
}

func TestSenderFallsBackToConfirmationCount(t *testing.T) {
	fastPolling(t)

	client := newFakeTxClient("bsc")
	client.finalizedErr = errors.New("the finalized tag is not supported")

	config := testSenderConfig()
	config.Confirmations = 3

	sender := newTestSender(t, client, config)

	pending, err := sender.Submit(context.Background(), testRequest())
	require.NoError(t, err)

	// mined at head 100, so 3 confirmations means head must reach 102
	time.Sleep(20 * time.Millisecond)

	select {
	case <-pending.Done():
		t.Fatal("the transaction was confirmed before enough blocks were produced")
	default:
	}

	client.mu.Lock()
	client.head = 102
	client.mu.Unlock()

	result, err := pending.Wait(context.Background())
	require.NoError(t, err)
	assert.False(t, result.FinalizedByTag)
	assert.True(t, result.Succeeded())
}

func TestSenderDetectsReorg(t *testing.T) {
	fastPolling(t)

	client := newFakeTxClient("bsc")
	sender := newTestSender(t, client, testSenderConfig())

	var reorged bool
	client.sendHook = func(f *fakeTxClient, tx *ethtypes.Transaction) error {
		f.mine(tx, ethtypes.ReceiptStatusSuccessful)

		if !reorged {
			reorged = true

			// drop the receipt again right after it appears, as a reorg would
			go func() {
				f.mu.Lock()
				delete(f.receipts, tx.Hash())
				f.mu.Unlock()

				time.Sleep(20 * time.Millisecond)
				f.mine(tx, ethtypes.ReceiptStatusSuccessful)
			}()
		}

		return nil
	}

	result, err := sender.SendAndWait(context.Background(), testRequest())
	require.NoError(t, err)
	assert.True(t, result.Succeeded())
}

func TestSenderAlreadyKnownIsNotAnError(t *testing.T) {
	fastPolling(t)

	client := newFakeTxClient("bsc")
	client.sendHook = func(f *fakeTxClient, tx *ethtypes.Transaction) error {
		f.mine(tx, ethtypes.ReceiptStatusSuccessful)

		return errors.New("already known")
	}

	sender := newTestSender(t, client, testSenderConfig())

	result, err := sender.SendAndWait(context.Background(), testRequest())
	require.NoError(t, err)
	assert.True(t, result.Succeeded())
}

func TestSenderNonceTooLowResynchronizes(t *testing.T) {
	fastPolling(t)

	ctx := context.Background()
	client := newFakeTxClient("bsc")
	client.sendHook = func(_ *fakeTxClient, _ *ethtypes.Transaction) error {
		return errors.New("nonce too low")
	}

	sender := newTestSender(t, client, testSenderConfig())

	_, err := sender.Submit(ctx, testRequest())
	assert.ErrorContains(t, err, "nonce too low")

	// the node moved on, and the manager must pick up its view instead of
	// reusing the stale local count
	client.mu.Lock()
	client.nonce = 9
	client.mu.Unlock()

	nonce, err := sender.Nonces().Next(ctx)
	assert.NoError(t, err)
	assert.Equal(t, uint64(9), nonce)
}

func TestSenderRejectsUnsupportedRequests(t *testing.T) {
	ctx := context.Background()
	client := newFakeTxClient("bsc")
	sender := newTestSender(t, client, testSenderConfig())

	_, err := sender.Submit(ctx, SendRequest{})
	assert.ErrorContains(t, err, "contract creation is not supported")

	req := testRequest()
	req.Private = true

	_, err = sender.Submit(ctx, req)
	assert.ErrorIs(t, err, ErrPrivateRelayNotConfigured)

	config := testSenderConfig()
	config.PrivateRelay = "https://relay.example"

	_, err = newTestSender(t, client, config).Submit(ctx, req)
	assert.ErrorIs(t, err, ErrPrivateRelayNotImplemented)
}

func TestSenderWaitRespectsTheCallerContext(t *testing.T) {
	fastPolling(t)

	client := newFakeTxClient("bsc")
	client.sendHook = func(_ *fakeTxClient, _ *ethtypes.Transaction) error { return nil }

	sender := newTestSender(t, client, testSenderConfig())

	pending, err := sender.Submit(context.Background(), testRequest())
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err = pending.Wait(ctx)
	assert.ErrorIs(t, err, context.DeadlineExceeded)

	// the background watch is still running, so the transaction can still
	// confirm after the caller stopped waiting
	client.mine(client.sentTx(0), ethtypes.ReceiptStatusSuccessful)

	result, err := pending.Wait(context.Background())
	require.NoError(t, err)
	assert.True(t, result.Succeeded())
}

func TestSenderConfigDefaults(t *testing.T) {
	chain, _ := LookupChain("polygon")
	config := SenderConfig{}.withDefaults(chain)

	assert.Equal(t, chain.DefaultConfirmations, config.Confirmations)
	assert.Equal(t, DefaultGasLimitMultiplier, config.GasLimitMultiplier)
	assert.Equal(t, DefaultBumpInterval, config.BumpInterval)
	assert.Equal(t, DefaultSendTimeout, config.Timeout)
	assert.Equal(t, chain.BlockTime, config.PollInterval)
}
