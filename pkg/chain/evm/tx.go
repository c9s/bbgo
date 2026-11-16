package evm

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	log "github.com/sirupsen/logrus"
)

var (
	// ErrTxReverted is returned when a mined transaction has status 0. The
	// TxResult is still returned so the caller can account for the gas spent.
	ErrTxReverted = errors.New("the transaction reverted on chain")

	// ErrPrivateRelayNotConfigured is returned for a request asking for private
	// submission on a sender that has no relay configured. Private submission
	// itself is not implemented yet.
	ErrPrivateRelayNotConfigured = errors.New("private transaction submission is not configured")

	// ErrPrivateRelayNotImplemented marks the MEV protection hook as reserved.
	ErrPrivateRelayNotImplemented = errors.New("private transaction submission is not implemented yet")
)

const (
	DefaultGasLimitMultiplier = 1.2
	DefaultBumpInterval       = 45 * time.Second
	DefaultSendTimeout        = 10 * time.Minute
)

// minPollInterval is the floor for the receipt polling interval. It is a
// variable so the lifecycle tests can drive the state machine without waiting
// on real block times.
var minPollInterval = time.Second

// SenderConfig configures the transaction lifecycle.
type SenderConfig struct {
	Fee FeeConfig `json:"fee,omitempty" yaml:"fee,omitempty"`

	// Confirmations overrides Chain.DefaultConfirmations, which is only used
	// when the finalized block tag is unavailable.
	Confirmations int `json:"confirmations,omitempty" yaml:"confirmations,omitempty"`

	// GasLimitMultiplier pads the estimated gas limit.
	GasLimitMultiplier float64 `json:"gasLimitMultiplier,omitempty" yaml:"gasLimitMultiplier,omitempty"`

	// BumpInterval is how long a transaction may stay pending before it is
	// replaced at a higher fee.
	BumpInterval time.Duration `json:"bumpInterval,omitempty" yaml:"bumpInterval,omitempty"`

	// PollInterval is the receipt polling interval. It defaults to the chain
	// block time.
	PollInterval time.Duration `json:"pollInterval,omitempty" yaml:"pollInterval,omitempty"`

	// Timeout bounds the whole lifecycle of one transaction, including the
	// confirmation wait.
	Timeout time.Duration `json:"timeout,omitempty" yaml:"timeout,omitempty"`

	// PrivateRelay reserves the MEV protection hook. Setting it is rejected
	// until private submission is implemented.
	PrivateRelay string `json:"privateRelay,omitempty" yaml:"privateRelay,omitempty"`
}

func (c SenderConfig) withDefaults(chain *Chain) SenderConfig {
	c.Fee = c.Fee.WithDefaults()

	if c.Confirmations <= 0 {
		c.Confirmations = chain.DefaultConfirmations
	}

	if c.GasLimitMultiplier < 1 {
		c.GasLimitMultiplier = DefaultGasLimitMultiplier
	}

	if c.BumpInterval <= 0 {
		c.BumpInterval = DefaultBumpInterval
	}

	if c.PollInterval <= 0 {
		c.PollInterval = chain.BlockTime
	}

	if c.PollInterval < minPollInterval {
		c.PollInterval = minPollInterval
	}

	if c.Timeout <= 0 {
		c.Timeout = DefaultSendTimeout
	}

	return c
}

// TxClient is the subset of Client that the sender needs. It exists so the
// lifecycle can be tested without a node.
type TxClient interface {
	NonceReader
	FeeEstimator

	Chain() *Chain
	EstimateGas(ctx context.Context, call ethereum.CallMsg) (uint64, error)
	CallContract(ctx context.Context, call ethereum.CallMsg, blockNumber *big.Int) ([]byte, error)
	SendTransaction(ctx context.Context, tx *ethtypes.Transaction) error
	TransactionReceipt(ctx context.Context, hash common.Hash) (*ethtypes.Receipt, error)
	BlockNumber(ctx context.Context) (uint64, error)
}

// SendRequest describes one transaction to send.
type SendRequest struct {
	// To is the recipient. A nil To means contract creation, which this layer
	// does not support.
	To *common.Address

	// Value is the native amount in wei.
	Value *big.Int

	Data []byte

	// GasLimit skips the gas estimation when non-zero.
	GasLimit uint64

	// Fee skips the fee estimation when non-nil.
	Fee *Fee

	// SkipSimulation disables the eth_call dry run. The gas estimation still
	// runs, so a revert is still usually caught.
	SkipSimulation bool

	// Private asks for submission through a private relay (MEV protection).
	Private bool

	// Label appears in the logs, for example "approve" or "swap".
	Label string
}

func (r SendRequest) callMsg(from common.Address, fee *Fee, gasLimit uint64) ethereum.CallMsg {
	msg := ethereum.CallMsg{
		From:  from,
		To:    r.To,
		Value: r.Value,
		Data:  r.Data,
		Gas:   gasLimit,
	}

	if fee != nil {
		if fee.Legacy {
			msg.GasPrice = fee.GasPrice
		} else {
			msg.GasFeeCap = fee.MaxFeePerGas
			msg.GasTipCap = fee.MaxPriorityFeePerGas
		}
	}

	return msg
}

// TxResult is the outcome of a confirmed transaction. All amounts are raw
// values: GasCost is in wei and is not converted to any quote currency, because
// this layer has no price source.
type TxResult struct {
	Hash  common.Hash
	Nonce uint64

	BlockNumber uint64
	BlockHash   common.Hash

	Status  uint64
	GasUsed uint64

	EffectiveGasPrice *big.Int

	// GasCost is GasUsed * EffectiveGasPrice, in wei of the native currency.
	GasCost *big.Int

	Receipt *ethtypes.Receipt

	// ReplacedHashes lists the earlier hashes of this nonce, from the original
	// broadcast through every fee bump.
	ReplacedHashes []common.Hash

	// FinalizedByTag reports whether finality came from the finalized block tag
	// rather than from counting confirmations.
	FinalizedByTag bool
}

func (r *TxResult) Succeeded() bool {
	return r != nil && r.Status == ethtypes.ReceiptStatusSuccessful
}

func (r *TxResult) String() string {
	if r == nil {
		return "<nil>"
	}

	return fmt.Sprintf("tx %s nonce=%d block=%d status=%d gasUsed=%d gasCost=%s wei",
		r.Hash.Hex(), r.Nonce, r.BlockNumber, r.Status, r.GasUsed, r.GasCost)
}

// PendingTx is the handle returned by Submit. The transaction is watched in the
// background: bumping and confirmation continue even if the caller stops
// waiting.
type PendingTx struct {
	nonce uint64
	label string

	mu     sync.Mutex
	hashes []common.Hash
	result *TxResult
	err    error

	done chan struct{}
}

func newPendingTx(nonce uint64, label string, hash common.Hash) *PendingTx {
	return &PendingTx{
		nonce:  nonce,
		label:  label,
		hashes: []common.Hash{hash},
		done:   make(chan struct{}),
	}
}

func (p *PendingTx) Nonce() uint64 {
	return p.nonce
}

func (p *PendingTx) Label() string {
	return p.label
}

// Hash returns the hash of the most recent broadcast of this nonce.
func (p *PendingTx) Hash() common.Hash {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.hashes[len(p.hashes)-1]
}

// Hashes returns every hash broadcast for this nonce, oldest first.
func (p *PendingTx) Hashes() []common.Hash {
	p.mu.Lock()
	defer p.mu.Unlock()

	return append([]common.Hash(nil), p.hashes...)
}

func (p *PendingTx) addHash(hash common.Hash) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.hashes = append(p.hashes, hash)
}

// Done is closed once the transaction reaches a terminal state.
func (p *PendingTx) Done() <-chan struct{} {
	return p.done
}

func (p *PendingTx) finish(result *TxResult, err error) {
	p.mu.Lock()
	p.result = result
	p.err = err
	p.mu.Unlock()

	close(p.done)
}

// Wait blocks until the transaction reaches a terminal state or the given
// context is done. Abandoning the wait does not stop the background watch.
//
// A reverted transaction returns both a result, for gas accounting, and an
// error wrapping ErrTxReverted.
func (p *PendingTx) Wait(ctx context.Context) (*TxResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()

	case <-p.done:
		p.mu.Lock()
		defer p.mu.Unlock()

		return p.result, p.err
	}
}

// Sender signs, broadcasts and confirms transactions for one address.
//
// Transactions are serialized: one transaction is in flight at a time, so
// Submit blocks until the previous transaction of this sender reaches a
// terminal state. That keeps the nonce sequence unambiguous and makes a failure
// impossible to leave a gap behind.
type Sender struct {
	client TxClient
	signer Signer
	chain  *Chain
	nonces *NonceManager
	config SenderConfig

	queue sync.Mutex
}

func NewSender(client TxClient, signer Signer, config SenderConfig) *Sender {
	chain := client.Chain()

	return &Sender{
		client: client,
		signer: signer,
		chain:  chain,
		nonces: NewNonceManager(client, signer.Address()),
		config: config.withDefaults(chain),
	}
}

func (s *Sender) Address() common.Address {
	return s.signer.Address()
}

func (s *Sender) Nonces() *NonceManager {
	return s.nonces
}

func (s *Sender) Config() SenderConfig {
	return s.config
}

// Simulate runs the request as an eth_call and returns the call output. It
// changes nothing on chain and takes no nonce.
func (s *Sender) Simulate(ctx context.Context, req SendRequest) ([]byte, error) {
	if req.To == nil {
		return nil, errors.New("contract creation is not supported")
	}

	out, err := s.client.CallContract(ctx, req.callMsg(s.Address(), nil, 0), nil)
	if err != nil {
		return nil, WrapRevert("simulation", err)
	}

	return out, nil
}

// EstimateGas returns the padded gas limit for the request.
func (s *Sender) EstimateGas(ctx context.Context, req SendRequest) (uint64, error) {
	estimated, err := s.client.EstimateGas(ctx, req.callMsg(s.Address(), nil, 0))
	if err != nil {
		return 0, WrapRevert("gas estimation", err)
	}

	padded := uint64(float64(estimated) * s.config.GasLimitMultiplier)
	if padded < estimated {
		padded = estimated
	}

	return padded, nil
}

// EstimateFee prices a transaction with this sender's fee configuration.
func (s *Sender) EstimateFee(ctx context.Context) (*Fee, error) {
	return EstimateFee(ctx, s.client, s.chain, s.config.Fee)
}

// SendAndWait submits the transaction and waits for it to reach a terminal
// state. It is a blocking wrapper around Submit.
func (s *Sender) SendAndWait(ctx context.Context, req SendRequest) (*TxResult, error) {
	pending, err := s.Submit(ctx, req)
	if err != nil {
		return nil, err
	}

	return pending.Wait(ctx)
}

// Submit prepares, signs and broadcasts the transaction, then watches it in the
// background. It returns once the transaction is broadcast, so the handle
// always carries a real hash.
//
// The background watch does not inherit cancellation from ctx, since the whole
// point of the handle is to outlive the caller; it is bounded by
// SenderConfig.Timeout instead.
func (s *Sender) Submit(ctx context.Context, req SendRequest) (*PendingTx, error) {
	if req.To == nil {
		return nil, errors.New("contract creation is not supported")
	}

	if req.Private {
		if s.config.PrivateRelay == "" {
			return nil, ErrPrivateRelayNotConfigured
		}

		return nil, ErrPrivateRelayNotImplemented
	}

	// serialize: one transaction in flight per sender
	s.queue.Lock()

	pending, params, err := s.prepareAndBroadcast(ctx, req)
	if err != nil {
		s.queue.Unlock()
		return nil, err
	}

	go func() {
		defer s.queue.Unlock()

		watchCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.config.Timeout)
		defer cancel()

		result, err := s.watch(watchCtx, params, pending)
		pending.finish(result, err)
	}()

	return pending, nil
}

// txParams is everything needed to re-sign the same transaction at a higher fee.
type txParams struct {
	nonce    uint64
	to       common.Address
	value    *big.Int
	data     []byte
	gasLimit uint64
	fee      *Fee
	label    string
	logger   log.FieldLogger
}

func (s *Sender) prepareAndBroadcast(ctx context.Context, req SendRequest) (*PendingTx, *txParams, error) {
	nonce, err := s.nonces.Next(ctx)
	if err != nil {
		return nil, nil, err
	}

	release := true
	defer func() {
		if release {
			s.nonces.Release(nonce)
		}
	}()

	if !req.SkipSimulation {
		if _, err := s.Simulate(ctx, req); err != nil {
			return nil, nil, err
		}
	}

	gasLimit := req.GasLimit
	if gasLimit == 0 {
		gasLimit, err = s.EstimateGas(ctx, req)
		if err != nil {
			return nil, nil, err
		}
	}

	fee := req.Fee
	if fee == nil {
		fee, err = s.EstimateFee(ctx)
		if err != nil {
			return nil, nil, err
		}
	}

	label := req.Label
	if label == "" {
		label = "tx"
	}

	params := &txParams{
		nonce:    nonce,
		to:       *req.To,
		value:    req.Value,
		data:     req.Data,
		gasLimit: gasLimit,
		fee:      fee,
		label:    label,
		logger: log.WithFields(log.Fields{
			"chain": s.chain.Name,
			"from":  s.Address().Hex(),
			"nonce": nonce,
			"label": label,
		}),
	}

	hash, err := s.broadcast(ctx, params)
	if err != nil {
		if isNonceError(err) {
			// the local count disagrees with the node, resynchronize instead of
			// handing the nonce back
			release = false
			s.nonces.Reset()
		}

		return nil, nil, err
	}

	release = false

	params.logger.Infof("broadcast %s, gasLimit=%d %s, %s",
		hash.Hex(), gasLimit, params.fee, s.chain.TxURL(hash.Hex()))

	return newPendingTx(nonce, label, hash), params, nil
}

func (s *Sender) buildTx(params *txParams) *ethtypes.Transaction {
	value := params.value
	if value == nil {
		value = new(big.Int)
	}

	if params.fee.Legacy {
		return ethtypes.NewTx(&ethtypes.LegacyTx{
			Nonce:    params.nonce,
			To:       &params.to,
			Value:    value,
			Data:     params.data,
			Gas:      params.gasLimit,
			GasPrice: params.fee.GasPrice,
		})
	}

	return ethtypes.NewTx(&ethtypes.DynamicFeeTx{
		ChainID:   s.chain.ID.Big(),
		Nonce:     params.nonce,
		To:        &params.to,
		Value:     value,
		Data:      params.data,
		Gas:       params.gasLimit,
		GasFeeCap: params.fee.MaxFeePerGas,
		GasTipCap: params.fee.MaxPriorityFeePerGas,
	})
}

func (s *Sender) broadcast(ctx context.Context, params *txParams) (common.Hash, error) {
	signed, err := s.signer.SignTx(s.buildTx(params), s.chain.ID.Big())
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to sign the transaction: %w", err)
	}

	if err := s.client.SendTransaction(ctx, signed); err != nil {
		// the same signed transaction already sitting in the pool is the
		// outcome we wanted
		if isAlreadyKnown(err) {
			return signed.Hash(), nil
		}

		return common.Hash{}, fmt.Errorf("failed to broadcast the transaction: %w", err)
	}

	return signed.Hash(), nil
}

func (s *Sender) watch(ctx context.Context, params *txParams, pending *PendingTx) (*TxResult, error) {
	ticker := time.NewTicker(s.config.PollInterval)
	defer ticker.Stop()

	bumps := 0
	lastBroadcast := time.Now()

	for {
		receipt := s.findReceipt(ctx, pending.Hashes(), params.logger)
		if receipt != nil {
			return s.confirm(ctx, params, pending, receipt)
		}

		if bumps < s.config.Fee.MaxBumpAttempts && time.Since(lastBroadcast) >= s.config.BumpInterval {
			bumped, err := params.fee.Bump(s.config.Fee.BumpPercent, s.config.Fee.MaxFeePerGasWei())
			switch {
			case errors.Is(err, ErrFeeCapExceeded):
				// stop bumping, keep waiting: the transaction may still be
				// included once the network calms down
				params.logger.Warnf("not bumping the fee any further: %v", err)
				bumps = s.config.Fee.MaxBumpAttempts

			case err != nil:
				params.logger.WithError(err).Warnf("failed to compute the bumped fee")

			default:
				previous := params.fee
				params.fee = bumped

				hash, broadcastErr := s.broadcast(ctx, params)
				if broadcastErr != nil {
					params.fee = previous

					if isNonceError(broadcastErr) {
						// an earlier broadcast of this nonce was mined; keep
						// polling the hashes we already know
						params.logger.Infof("the nonce was already used while bumping, waiting for the receipt")
						bumps = s.config.Fee.MaxBumpAttempts
					} else {
						params.logger.WithError(broadcastErr).Warnf("failed to broadcast the replacement transaction")
					}
				} else {
					bumps++
					lastBroadcast = time.Now()
					pending.addHash(hash)
					params.logger.Infof("replaced with %s at a higher fee (%s), attempt %d/%d",
						hash.Hex(), params.fee, bumps, s.config.Fee.MaxBumpAttempts)
				}
			}
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("gave up waiting for %s: %w", pending.Hash().Hex(), ctx.Err())

		case <-ticker.C:
		}
	}
}

// findReceipt looks for a receipt of any hash broadcast for this nonce, since a
// bumped transaction may be mined under an earlier hash.
func (s *Sender) findReceipt(
	ctx context.Context, hashes []common.Hash, logger log.FieldLogger,
) *ethtypes.Receipt {
	for _, hash := range hashes {
		receipt, err := s.client.TransactionReceipt(ctx, hash)
		if err == nil && receipt != nil {
			return receipt
		}

		if err != nil && !errors.Is(err, ethereum.NotFound) {
			logger.WithError(err).Debugf("failed to read the receipt of %s", hash.Hex())
		}
	}

	return nil
}

// confirm waits for the receipt to become final and re-reads it afterwards, so
// a transaction that was reorged out is detected instead of reported as done.
func (s *Sender) confirm(
	ctx context.Context, params *txParams, pending *PendingTx, receipt *ethtypes.Receipt,
) (*TxResult, error) {
	ticker := time.NewTicker(s.config.PollInterval)
	defer ticker.Stop()

	for {
		final, byTag, err := s.isFinal(ctx, receipt.BlockNumber.Uint64())
		if err != nil {
			params.logger.WithError(err).Debugf("failed to check finality")
		}

		if final {
			// re-read the receipt: a reorg would have moved or dropped it
			current, err := s.client.TransactionReceipt(ctx, receipt.TxHash)
			if err != nil {
				if errors.Is(err, ethereum.NotFound) {
					params.logger.Warnf("the receipt of %s disappeared, the transaction was reorged out",
						receipt.TxHash.Hex())

					return s.watch(ctx, params, pending)
				}

				params.logger.WithError(err).Debugf("failed to re-read the receipt")
			} else if current != nil {
				receipt = current
			}

			return s.result(params, pending, receipt, byTag)
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("gave up waiting for the confirmation of %s: %w",
				receipt.TxHash.Hex(), ctx.Err())

		case <-ticker.C:
		}
	}
}

// isFinal prefers the finalized block tag and falls back to counting
// confirmations when the tag is unavailable.
func (s *Sender) isFinal(ctx context.Context, blockNumber uint64) (final bool, byTag bool, err error) {
	if s.chain.SupportsFinalizedTag {
		header, tagErr := s.client.HeaderByNumber(ctx, big.NewInt(int64(rpc.FinalizedBlockNumber)))
		if tagErr == nil && header != nil && header.Number != nil {
			return header.Number.Uint64() >= blockNumber, true, nil
		}

		err = tagErr
	}

	head, headErr := s.client.BlockNumber(ctx)
	if headErr != nil {
		return false, false, errors.Join(err, headErr)
	}

	if head < blockNumber {
		return false, false, err
	}

	return head-blockNumber+1 >= uint64(s.config.Confirmations), false, err
}

func (s *Sender) result(
	params *txParams, pending *PendingTx, receipt *ethtypes.Receipt, finalizedByTag bool,
) (*TxResult, error) {
	effectiveGasPrice := receipt.EffectiveGasPrice
	if effectiveGasPrice == nil || effectiveGasPrice.Sign() == 0 {
		// some chains omit it; the fee we signed with is the best answer
		effectiveGasPrice = params.fee.Cap()
	}

	gasCost := new(big.Int)
	if effectiveGasPrice != nil {
		gasCost.Mul(new(big.Int).SetUint64(receipt.GasUsed), effectiveGasPrice)
	}

	hashes := pending.Hashes()
	replaced := make([]common.Hash, 0, len(hashes))
	for _, hash := range hashes {
		if hash != receipt.TxHash {
			replaced = append(replaced, hash)
		}
	}

	result := &TxResult{
		Hash:              receipt.TxHash,
		Nonce:             params.nonce,
		BlockNumber:       receipt.BlockNumber.Uint64(),
		BlockHash:         receipt.BlockHash,
		Status:            receipt.Status,
		GasUsed:           receipt.GasUsed,
		EffectiveGasPrice: effectiveGasPrice,
		GasCost:           gasCost,
		Receipt:           receipt,
		ReplacedHashes:    replaced,
		FinalizedByTag:    finalizedByTag,
	}

	if !result.Succeeded() {
		params.logger.Errorf("%s reverted on chain, gasUsed=%d gasCost=%s wei",
			receipt.TxHash.Hex(), receipt.GasUsed, gasCost)

		return result, fmt.Errorf("transaction %s: %w", receipt.TxHash.Hex(), ErrTxReverted)
	}

	params.logger.Infof("confirmed %s in block %d, gasUsed=%d gasCost=%s wei, finalizedByTag=%v",
		receipt.TxHash.Hex(), result.BlockNumber, receipt.GasUsed, gasCost, finalizedByTag)

	return result, nil
}

func isAlreadyKnown(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())

	return strings.Contains(msg, "already known") || strings.Contains(msg, "already in the pool")
}

func isNonceError(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())

	return strings.Contains(msg, "nonce too low") || strings.Contains(msg, "nonce too high")
}
