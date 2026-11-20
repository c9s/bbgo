// Package stream follows an EVM chain block by block.
//
// The stream is driven by block heads, not by a log subscription. A WebSocket
// newHeads subscription only tells the stream that the chain moved; every block
// is then read over the HTTP endpoint pool, together with its logs queried by
// block hash. That gives one code path for three situations that a plain
// eth_subscribe("logs") handles badly:
//
//   - a dropped WebSocket loses nothing: the stream reads every block from its
//     cursor up to the head, so the blocks missed while disconnected are
//     backfilled from the cursor;
//   - no WebSocket endpoint at all: a poll of eth_blockNumber drives it instead;
//   - reorgs: every block's parent hash is checked against the previous block,
//     and a mismatch walks back to the common ancestor and reports what was
//     removed.
//
// Callbacks run synchronously on the Run goroutine in chain order, so a slow
// callback slows the stream down instead of losing blocks.
package stream

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/chain/evm"
)

// ErrReorgTooDeep is returned by Run when the node's chain no longer contains
// any block of the stream's recent window. Either the reorg is deeper than
// Config.MaxReorgDepth, or the endpoints disagree about which chain they
// follow. The stream cannot tell which blocks to roll back, so it stops.
var ErrReorgTooDeep = errors.New("evm stream: reorg deeper than the tracked window")

const (
	minPollInterval     = 250 * time.Millisecond
	minWSPollInterval   = 30 * time.Second
	minStallTimeout     = 30 * time.Second
	defaultMaxReorgDeep = 64

	maxRetryBackoff = 30 * time.Second
)

// Reader is the subset of evm.Client the stream reads blocks with.
type Reader interface {
	Chain() *evm.Chain
	BlockNumber(ctx context.Context) (uint64, error)
	BlockByNumber(ctx context.Context, number *big.Int, fullTx bool) (*evm.Block, error)
	FilterLogs(ctx context.Context, query ethereum.FilterQuery) ([]ethtypes.Log, error)
}

var _ Reader = (*evm.Client)(nil)

// LogFilter selects the logs fetched for every block. It has the semantics of
// eth_getLogs: Addresses is an OR list, Topics is positional with an OR list
// per position, and an empty position matches anything.
type LogFilter struct {
	Addresses []common.Address
	Topics    [][]common.Hash
}

type Config struct {
	// WSEndpoint is the WebSocket endpoint the head subscription uses. Without
	// it the stream polls eth_blockNumber over the HTTP endpoints.
	WSEndpoint string

	// FromBlock is the first block to deliver. Zero starts at the current
	// head, minus Confirmations. Set it to the last delivered block + 1 to
	// resume after a restart.
	FromBlock uint64

	// Confirmations is how many blocks the stream stays behind the head. Zero
	// delivers each block as soon as it is seen, at the cost of more reorgs
	// to handle; a few blocks behind, reorgs become rare.
	Confirmations int

	// FullTransactions fetches the transactions of every block, not only
	// their hashes.
	FullTransactions bool

	// Logs, when set, fetches the matching logs of every block.
	Logs *LogFilter

	// PollInterval is how often eth_blockNumber is polled. It drives the
	// stream when there is no WebSocket endpoint and defaults to the block
	// time. With a WebSocket endpoint it is only a safety net, so the
	// subscription going quiet without an error does not stall the stream; it
	// defaults to ten block times, at least 30s.
	PollInterval time.Duration

	// StallTimeout reconnects the WebSocket when no head arrives for this
	// long. Some providers drop a subscription without closing the socket. It
	// defaults to ten block times, at least 30s.
	StallTimeout time.Duration

	// MaxReorgDepth is how many delivered blocks are kept to find the common
	// ancestor on a reorg. It defaults to twice the chain's default
	// confirmations, at least 64.
	MaxReorgDepth int
}

func (c Config) withDefaults(chain *evm.Chain) Config {
	blockTime := chain.BlockTime
	if blockTime <= 0 {
		blockTime = time.Second
	}

	if c.PollInterval <= 0 {
		if c.WSEndpoint == "" {
			c.PollInterval = blockTime
		} else {
			c.PollInterval = max(10*blockTime, minWSPollInterval)
		}
	}

	c.PollInterval = max(c.PollInterval, minPollInterval)

	if c.StallTimeout <= 0 {
		c.StallTimeout = max(10*blockTime, minStallTimeout)
	}

	if c.MaxReorgDepth <= 0 {
		c.MaxReorgDepth = max(2*chain.DefaultConfirmations, defaultMaxReorgDeep)
	}

	if c.Confirmations < 0 {
		c.Confirmations = 0
	}

	return c
}

// BlockRef identifies one block.
type BlockRef struct {
	Number uint64
	Hash   common.Hash
}

func (r BlockRef) String() string {
	return fmt.Sprintf("#%d (%s)", r.Number, r.Hash.Hex())
}

// Block is one delivered block with the logs matching Config.Logs.
type Block struct {
	*evm.Block

	Logs []ethtypes.Log
}

func (b *Block) Ref() BlockRef {
	return BlockRef{Number: b.Number(), Hash: b.Hash}
}

// Reorg reports blocks that were delivered and are no longer on the canonical
// chain. Removed is ordered newest first. Every delivered log carries its
// BlockHash, so a consumer drops what it derived from the removed hashes. The
// stream continues with the block after Ancestor.
type Reorg struct {
	Ancestor BlockRef
	Removed  []BlockRef
}

// Stream delivers the blocks of one chain in order.
//
//go:generate callbackgen -type Stream
type Stream struct {
	reader Reader
	chain  *evm.Chain
	config Config

	// wake is signalled when a new head is observed
	wake chan struct{}

	mu        sync.Mutex
	head      uint64
	cursor    BlockRef
	hasCursor bool
	connected bool
	running   bool

	// recent holds the delivered blocks used to find the common ancestor on
	// a reorg, oldest first. Only the Run goroutine touches it.
	recent []BlockRef

	blockCallbacks      []func(block *Block)
	logCallbacks        []func(log ethtypes.Log)
	reorgCallbacks      []func(reorg Reorg)
	connectCallbacks    []func()
	disconnectCallbacks []func(err error)
}

func New(reader Reader, config Config) *Stream {
	chain := reader.Chain()
	return &Stream{
		reader: reader,
		chain:  chain,
		config: config.withDefaults(chain),
		wake:   make(chan struct{}, 1),
	}
}

func (s *Stream) Config() Config {
	return s.config
}

// Cursor returns the last delivered block. Persist Cursor().Number + 1 as
// Config.FromBlock to resume.
func (s *Stream) Cursor() (BlockRef, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cursor, s.hasCursor
}

// Head returns the highest head number observed.
func (s *Stream) Head() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.head
}

// Connected reports whether the WebSocket head subscription is live. It is
// always false without a WebSocket endpoint.
func (s *Stream) Connected() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.connected
}

func (s *Stream) setConnected(connected bool) {
	s.mu.Lock()
	s.connected = connected
	s.mu.Unlock()
}

// observeHead records a head number and wakes the Run loop. Heads only move
// forward: a lower number, from an endpoint lagging behind, is ignored.
func (s *Stream) observeHead(number uint64) {
	s.mu.Lock()
	if number > s.head {
		s.head = number
	}
	s.mu.Unlock()

	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *Stream) setCursor(ref BlockRef) {
	s.mu.Lock()
	s.cursor = ref
	s.hasCursor = true
	s.mu.Unlock()
}

// Run follows the chain until ctx is done. It returns ctx.Err() on
// cancellation, ErrReorgTooDeep, or a configuration error such as a WebSocket
// endpoint serving another chain. Transient RPC errors are retried.
func (s *Stream) Run(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return errors.New("evm stream: already running")
	}
	s.running = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	fatal := make(chan error, 1)
	if s.config.WSEndpoint != "" {
		go s.subscribeHeads(ctx, fatal)
	}

	if err := s.retry(ctx, s.init); err != nil {
		return err
	}

	ticker := time.NewTicker(s.config.PollInterval)
	defer ticker.Stop()

	backoff := time.Duration(0)
	for {
		err := s.catchUp(ctx)
		switch {
		case err == nil:
			backoff = 0

		case errors.Is(err, ErrReorgTooDeep):
			return err

		case ctx.Err() != nil:
			return ctx.Err()

		default:
			backoff = nextBackoff(backoff)
			log.WithError(err).Warnf("evm stream %s: failed to read blocks, retrying in %s", s.chain.Name, backoff)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case err := <-fatal:
				return err
			case <-time.After(backoff):
			}

			continue
		}

		select {
		case <-ctx.Done():
			return ctx.Err()

		case err := <-fatal:
			return err

		case <-s.wake:

		case <-ticker.C:
			s.pollHead(ctx)
		}
	}
}

func (s *Stream) pollHead(ctx context.Context) {
	number, err := s.reader.BlockNumber(ctx)
	if err != nil {
		if ctx.Err() == nil {
			log.WithError(err).Warnf("evm stream %s: failed to poll the head", s.chain.Name)
		}

		return
	}

	s.observeHead(number)
}

func (s *Stream) retry(ctx context.Context, fn func(ctx context.Context) error) error {
	backoff := time.Duration(0)
	for {
		err := fn(ctx)
		if err == nil {
			return nil
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}

		backoff = nextBackoff(backoff)
		log.WithError(err).Warnf("evm stream %s: retrying in %s", s.chain.Name, backoff)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
	}
}

// init reads the head and places the cursor right before the first block to
// deliver, so that block is parent-checked like every later one.
func (s *Stream) init(ctx context.Context) error {
	head, err := s.reader.BlockNumber(ctx)
	if err != nil {
		return fmt.Errorf("failed to read the head: %w", err)
	}

	s.observeHead(head)

	start := s.config.FromBlock
	if start == 0 {
		start = safeHead(head, s.config.Confirmations)
	}

	if start == 0 {
		// starting from genesis, which has no parent to check
		return nil
	}

	parent, err := s.reader.BlockByNumber(ctx, new(big.Int).SetUint64(start-1), false)
	if err != nil {
		return fmt.Errorf("failed to read block %d: %w", start-1, err)
	}

	ref := BlockRef{Number: parent.Number(), Hash: parent.Hash}
	s.recent = append(s.recent[:0], ref)
	s.setCursor(ref)

	log.Infof("evm stream %s: starting at block %d, head %d, confirmations %d",
		s.chain.Name, start, head, s.config.Confirmations)

	return nil
}

func safeHead(head uint64, confirmations int) uint64 {
	if uint64(confirmations) >= head {
		return 0
	}

	return head - uint64(confirmations)
}

// catchUp delivers every block from the cursor up to the head minus the
// confirmations.
func (s *Stream) catchUp(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		target := safeHead(s.Head(), s.config.Confirmations)
		cursor, hasCursor := s.Cursor()

		next := uint64(0)
		if hasCursor {
			next = cursor.Number + 1
		}

		if next > target {
			return nil
		}

		block, err := s.reader.BlockByNumber(ctx, new(big.Int).SetUint64(next), s.config.FullTransactions)
		if errors.Is(err, ethereum.NotFound) {
			// the endpoint answering is behind the head another one reported
			return nil
		} else if err != nil {
			return fmt.Errorf("failed to read block %d: %w", next, err)
		}

		if hasCursor && block.ParentHash() != cursor.Hash {
			if err := s.rollback(ctx); err != nil {
				return err
			}

			continue
		}

		out := &Block{Block: block}
		if s.config.Logs != nil {
			logs, err := s.readLogs(ctx, block)
			if err != nil {
				return fmt.Errorf("failed to read the logs of block %d: %w", next, err)
			}

			out.Logs = logs
		}

		s.deliver(out)
	}
}

// readLogs reads the matching logs of exactly one block.
//
// The logs are queried by block hash, so they cannot belong to another fork of
// the same height. But a load-balanced provider may route the query to a
// backend that has not indexed the block yet, and some answer an unknown block
// hash with an empty result rather than an error. An empty result is therefore
// only trusted when the block's logs bloom rules a match out. Otherwise it is
// checked again by block number: a lagging backend fails that query
// explicitly ("beyond current head"), which is retried, and an empty answer
// then means the bloom matched by chance.
func (s *Stream) readLogs(ctx context.Context, block *evm.Block) ([]ethtypes.Log, error) {
	byHash, err := s.reader.FilterLogs(ctx, ethereum.FilterQuery{
		BlockHash: &block.Hash,
		Addresses: s.config.Logs.Addresses,
		Topics:    s.config.Logs.Topics,
	})
	if err != nil {
		return nil, err
	}

	logs := logsOfBlock(byHash, block.Hash)
	if len(logs) > 0 || !bloomMayMatch(block.Header.Bloom, s.config.Logs) {
		return logs, nil
	}

	number := new(big.Int).SetUint64(block.Number())
	byNumber, err := s.reader.FilterLogs(ctx, ethereum.FilterQuery{
		FromBlock: number,
		ToBlock:   number,
		Addresses: s.config.Logs.Addresses,
		Topics:    s.config.Logs.Topics,
	})
	if err != nil {
		return nil, err
	}

	logs = logsOfBlock(byNumber, block.Hash)
	if len(logs) == 0 && len(byNumber) > 0 {
		return nil, fmt.Errorf("the endpoint returned the logs of another block %d", block.Number())
	}

	return logs, nil
}

// logsOfBlock keeps the logs of the given block, dropping anything a provider
// returned for another block or flagged as removed.
func logsOfBlock(logs []ethtypes.Log, blockHash common.Hash) []ethtypes.Log {
	out := logs[:0]
	for _, l := range logs {
		if l.BlockHash == blockHash && !l.Removed {
			out = append(out, l)
		}
	}

	return out
}

// bloomMayMatch reports whether a block with this logs bloom can contain a log
// matching the filter. A false answer is certain; a true one may be a false
// positive.
func bloomMayMatch(bloom ethtypes.Bloom, filter *LogFilter) bool {
	if bloom == (ethtypes.Bloom{}) {
		return false
	}

	if len(filter.Addresses) > 0 {
		found := false
		for _, address := range filter.Addresses {
			if ethtypes.BloomLookup(bloom, address) {
				found = true
				break
			}
		}

		if !found {
			return false
		}
	}

	for _, position := range filter.Topics {
		if len(position) == 0 {
			continue
		}

		found := false
		for _, topic := range position {
			if ethtypes.BloomLookup(bloom, topic) {
				found = true
				break
			}
		}

		if !found {
			return false
		}
	}

	return true
}

func (s *Stream) deliver(block *Block) {
	ref := block.Ref()

	s.recent = append(s.recent, ref)
	if over := len(s.recent) - s.config.MaxReorgDepth; over > 0 {
		s.recent = append(s.recent[:0], s.recent[over:]...)
	}

	s.setCursor(ref)

	s.EmitBlock(block)
	for _, l := range block.Logs {
		s.EmitLog(l)
	}
}

// rollback walks the recent window back until the node's block at the same
// height has the same hash, then moves the cursor to that common ancestor.
func (s *Stream) rollback(ctx context.Context) error {
	var removed []BlockRef

	for i := len(s.recent) - 1; i >= 0; i-- {
		ref := s.recent[i]

		block, err := s.reader.BlockByNumber(ctx, new(big.Int).SetUint64(ref.Number), false)
		if err != nil {
			return fmt.Errorf("failed to read block %d while looking for the reorg ancestor: %w", ref.Number, err)
		}

		if block.Hash != ref.Hash {
			removed = append(removed, ref)
			continue
		}

		s.recent = s.recent[:i+1]
		s.setCursor(ref)

		if len(removed) == 0 {
			// the parent check failed but nothing changed below the cursor:
			// the endpoint that answered was briefly on another fork. Fail
			// so the read is retried after a backoff instead of spinning.
			return fmt.Errorf("block %d does not extend the delivered block %s on this endpoint", ref.Number+1, ref)
		}

		log.Warnf("evm stream %s: reorg of %d block(s), ancestor %s", s.chain.Name, len(removed), ref)

		s.EmitReorg(Reorg{Ancestor: ref, Removed: removed})
		return nil
	}

	return fmt.Errorf("%w: none of the last %d delivered blocks is on the node's chain",
		ErrReorgTooDeep, len(s.recent))
}

func nextBackoff(d time.Duration) time.Duration {
	if d <= 0 {
		return time.Second
	}

	return min(2*d, maxRetryBackoff)
}
