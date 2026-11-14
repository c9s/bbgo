// Package stream follows a Bitcoin chain block by block.
//
// It is the Bitcoin counterpart of pkg/chain/evm/stream and has the same
// shape: something signals that the chain moved, and every block from the
// cursor up to the head is then read and parent-checked. Here the signal is a
// poll of getbestblockhash, which is cheap next to a ten-minute block time.
//
//   - Nothing is lost while the node is unreachable: the stream reads every
//     block from its cursor up to the head, so missed blocks are backfilled.
//   - Reorgs: every block's previousblockhash is checked against the last
//     delivered block, and a mismatch walks back to the common ancestor and
//     reports what was removed.
//   - A reorg that replaces the tip with another block at the same height
//     does not move the height, and the next block is ten minutes away. A
//     change of the best block hash therefore also re-checks that the
//     delivered cursor is still on the node's chain.
//
// Callbacks run synchronously on the Run goroutine in chain order, so a slow
// callback slows the stream down instead of losing blocks.
package stream

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/chain/btc"
)

// ErrReorgTooDeep is returned by Run when the node's chain no longer contains
// any block of the stream's recent window. Either the reorg is deeper than
// Config.MaxReorgDepth, or the node follows another chain. The stream cannot
// tell which blocks to roll back, so it stops.
var ErrReorgTooDeep = errors.New("btc stream: reorg deeper than the tracked window")

const (
	defaultPollInterval  = 5 * time.Second
	minPollInterval      = 250 * time.Millisecond
	defaultMaxReorgDepth = 64

	maxRetryBackoff = 30 * time.Second
)

// Reader is the subset of btc.Client the stream reads the chain with.
type Reader interface {
	GetBlockchainInfo(ctx context.Context) (*btc.BlockchainInfo, error)
	GetBestBlockHash(ctx context.Context) (btc.Hash, error)
	GetBlockHeader(ctx context.Context, hash btc.Hash) (*btc.BlockHeader, error)
	GetBlockHash(ctx context.Context, height int64) (btc.Hash, error)
	GetBlock(ctx context.Context, hash btc.Hash) (*btc.Block, error)
	GetBlockTxIDs(ctx context.Context, hash btc.Hash) (*btc.BlockHeader, []btc.Hash, error)
}

var _ Reader = (*btc.Client)(nil)

type Config struct {
	// Chain, when set, is the chain the node must follow: "main", "test",
	// "testnet4", "signet" or "regtest". Run fails on another one.
	Chain string

	// FromHeight is the first block to deliver. Zero starts at the current
	// head, minus Confirmations. Set it to the last delivered height + 1 to
	// resume after a restart.
	FromHeight int64

	// Confirmations is how many blocks the stream stays behind the head. Zero
	// delivers each block as soon as the node has it; one or two make reorgs
	// rare, six is the customary settlement depth.
	Confirmations int64

	// FullTransactions reads every block with getblock verbosity=3: decoded
	// transactions, each input with its prevout, each transaction with its
	// fee. A full block is around 15 MB of JSON. Without it only the header
	// and the transaction ids are read.
	FullTransactions bool

	// PollInterval is how often getbestblockhash is polled. It defaults to 5s.
	PollInterval time.Duration

	// MaxReorgDepth is how many delivered blocks are kept to find the common
	// ancestor on a reorg. It defaults to 64.
	MaxReorgDepth int
}

func (c Config) withDefaults() Config {
	if c.PollInterval <= 0 {
		c.PollInterval = defaultPollInterval
	}

	c.PollInterval = max(c.PollInterval, minPollInterval)

	if c.MaxReorgDepth <= 0 {
		c.MaxReorgDepth = defaultMaxReorgDepth
	}

	if c.Confirmations < 0 {
		c.Confirmations = 0
	}

	return c
}

// BlockRef identifies one block.
type BlockRef struct {
	Height int64
	Hash   btc.Hash
}

func (r BlockRef) String() string {
	return fmt.Sprintf("#%d (%s)", r.Height, r.Hash)
}

// Block is one delivered block.
type Block struct {
	*btc.Block

	// TxIDs lists the transaction ids in block order, the coinbase first. It
	// is set in both modes; Block.Tx is only set with Config.FullTransactions.
	TxIDs []btc.Hash
}

func (b *Block) Ref() BlockRef {
	return BlockRef{Height: b.Height, Hash: b.Hash}
}

// Reorg reports blocks that were delivered and are no longer on the node's
// chain. Removed is ordered newest first. The transactions of a removed block
// are back in the mempool or replaced, and the outputs they created no longer
// exist. The stream continues with the block after Ancestor.
type Reorg struct {
	Ancestor BlockRef
	Removed  []BlockRef
}

// Stream delivers the blocks of one chain in order.
//
//go:generate callbackgen -type Stream
type Stream struct {
	reader Reader
	config Config

	// wake is signalled when a new best block is observed
	wake chan struct{}

	mu        sync.Mutex
	head      BlockRef
	cursor    BlockRef
	hasCursor bool
	running   bool

	// headChanged is set when the best block hash changes, so the cursor is
	// checked to still be on the node's chain.
	headChanged bool

	// recent holds the delivered blocks used to find the common ancestor on
	// a reorg, oldest first. Only the Run goroutine touches it.
	recent []BlockRef

	blockCallbacks []func(block *Block)
	reorgCallbacks []func(reorg Reorg)
}

func New(reader Reader, config Config) *Stream {
	return &Stream{
		reader: reader,
		config: config.withDefaults(),
		wake:   make(chan struct{}, 1),
	}
}

func (s *Stream) Config() Config {
	return s.config
}

// Cursor returns the last delivered block. Persist Cursor().Height + 1 as
// Config.FromHeight to resume.
func (s *Stream) Cursor() (BlockRef, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cursor, s.hasCursor
}

// Head returns the node's best block as last observed.
func (s *Stream) Head() BlockRef {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.head
}

// observeHead records the node's best block and wakes the Run loop when it
// changed. Unlike an EVM head number, a lower height is not ignored: the node
// may have switched to a chain with more work and fewer blocks.
func (s *Stream) observeHead(head BlockRef) {
	s.mu.Lock()
	changed := head != s.head
	if changed {
		s.head = head
		s.headChanged = true
	}
	s.mu.Unlock()

	if changed {
		select {
		case s.wake <- struct{}{}:
		default:
		}
	}
}

func (s *Stream) setCursor(ref BlockRef) {
	s.mu.Lock()
	s.cursor = ref
	s.hasCursor = true
	s.mu.Unlock()
}

// Run follows the chain until ctx is done. It returns ctx.Err() on
// cancellation, ErrReorgTooDeep, or an error when the node follows another
// chain than Config.Chain. Transient RPC errors are retried.
func (s *Stream) Run(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return errors.New("btc stream: already running")
	}
	s.running = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

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
			log.WithError(err).Warnf("btc stream: failed to read blocks, retrying in %s", backoff)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}

			continue
		}

		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-s.wake:

		case <-ticker.C:
			s.pollHead(ctx)
		}
	}
}

// pollHead reads the best block hash, and its height only when it changed.
func (s *Stream) pollHead(ctx context.Context) {
	hash, err := s.reader.GetBestBlockHash(ctx)
	if err != nil {
		if ctx.Err() == nil {
			log.WithError(err).Warn("btc stream: failed to poll the best block")
		}

		return
	}

	if hash == s.Head().Hash {
		return
	}

	header, err := s.reader.GetBlockHeader(ctx, hash)
	if err != nil {
		if ctx.Err() == nil {
			log.WithError(err).Warnf("btc stream: failed to read the header of the best block %s", hash)
		}

		return
	}

	s.observeHead(BlockRef{Height: header.Height, Hash: header.Hash})
}

func (s *Stream) retry(ctx context.Context, fn func(ctx context.Context) error) error {
	backoff := time.Duration(0)
	for {
		err := fn(ctx)
		if err == nil {
			return nil
		}

		var fatal *fatalError
		if errors.As(err, &fatal) {
			return fatal.err
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}

		backoff = nextBackoff(backoff)
		log.WithError(err).Warnf("btc stream: retrying in %s", backoff)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
	}
}

// fatalError stops retry: retrying cannot fix it.
type fatalError struct{ err error }

func (e *fatalError) Error() string { return e.err.Error() }

// init reads the best block and places the cursor right before the first
// block to deliver, so that block is parent-checked like every later one.
func (s *Stream) init(ctx context.Context) error {
	info, err := s.reader.GetBlockchainInfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to read the chain info: %w", err)
	}

	if s.config.Chain != "" && info.Chain != s.config.Chain {
		return &fatalError{fmt.Errorf("btc stream: the node follows chain %q, want %q", info.Chain, s.config.Chain)}
	}

	s.observeHead(BlockRef{Height: info.Blocks, Hash: info.BestBlockHash})

	start := s.config.FromHeight
	if start <= 0 {
		start = max(info.Blocks-s.config.Confirmations, 0)
	}

	if start == 0 {
		// starting from genesis, which has no parent to check
		return nil
	}

	parent, err := s.reader.GetBlockHash(ctx, start-1)
	if err != nil {
		return fmt.Errorf("failed to read the hash of block %d: %w", start-1, err)
	}

	ref := BlockRef{Height: start - 1, Hash: parent}
	s.recent = append(s.recent[:0], ref)
	s.setCursor(ref)

	log.Infof("btc stream: chain %s, starting at block %d, head %d, confirmations %d",
		info.Chain, start, info.Blocks, s.config.Confirmations)

	return nil
}

// isBehind reports whether err means the node answering does not have the
// block yet: a load-balanced provider may route a call to a backend that is
// behind the one which reported the head.
func isBehind(err error) bool {
	var rpcErr *btc.RPCError
	return btc.IsNotFound(err) ||
		(errors.As(err, &rpcErr) && rpcErr.Code == btc.ErrCodeInvalidParameter)
}

// catchUp delivers every block from the cursor up to the head minus the
// confirmations.
func (s *Stream) catchUp(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		cursor, hasCursor := s.Cursor()

		if hasCursor && s.takeHeadChanged() {
			onChain, err := s.isOnChain(ctx, cursor)
			if err != nil {
				s.markHeadChanged()
				if isBehind(err) {
					return nil
				}

				return err
			}

			if !onChain {
				if err := s.rollback(ctx); err != nil {
					return err
				}

				continue
			}
		}

		next := int64(0)
		if hasCursor {
			next = cursor.Height + 1
		}

		if next > s.Head().Height-s.config.Confirmations {
			return nil
		}

		hash, err := s.reader.GetBlockHash(ctx, next)
		if isBehind(err) {
			return nil
		} else if err != nil {
			return fmt.Errorf("failed to read the hash of block %d: %w", next, err)
		}

		block, err := s.readBlock(ctx, hash)
		if isBehind(err) {
			return nil
		} else if err != nil {
			return fmt.Errorf("failed to read block %d %s: %w", next, hash, err)
		}

		if block.Hash != hash || block.Height != next {
			return fmt.Errorf("asked for block %d %s, got %s", next, hash, block.Ref())
		}

		if hasCursor && (block.PreviousBlockHash == nil || *block.PreviousBlockHash != cursor.Hash) {
			if err := s.rollback(ctx); err != nil {
				return err
			}

			continue
		}

		s.deliver(block)
	}
}

func (s *Stream) takeHeadChanged() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := s.headChanged
	s.headChanged = false
	return changed
}

func (s *Stream) markHeadChanged() {
	s.mu.Lock()
	s.headChanged = true
	s.mu.Unlock()
}

// isOnChain reports whether the node's block at ref's height is ref.
func (s *Stream) isOnChain(ctx context.Context, ref BlockRef) (bool, error) {
	hash, err := s.reader.GetBlockHash(ctx, ref.Height)
	if err != nil {
		return false, err
	}

	return hash == ref.Hash, nil
}

func (s *Stream) readBlock(ctx context.Context, hash btc.Hash) (*Block, error) {
	if s.config.FullTransactions {
		block, err := s.reader.GetBlock(ctx, hash)
		if err != nil {
			return nil, err
		}

		txids := make([]btc.Hash, len(block.Tx))
		for i := range block.Tx {
			txids[i] = block.Tx[i].TxID
		}

		return &Block{Block: block, TxIDs: txids}, nil
	}

	header, txids, err := s.reader.GetBlockTxIDs(ctx, hash)
	if err != nil {
		return nil, err
	}

	return &Block{Block: &btc.Block{BlockHeader: *header}, TxIDs: txids}, nil
}

func (s *Stream) deliver(block *Block) {
	ref := block.Ref()

	s.recent = append(s.recent, ref)
	if over := len(s.recent) - s.config.MaxReorgDepth; over > 0 {
		s.recent = append(s.recent[:0], s.recent[over:]...)
	}

	s.setCursor(ref)
	s.EmitBlock(block)
}

// rollback walks the recent window back until the node's block at the same
// height has the same hash, then moves the cursor to that common ancestor.
func (s *Stream) rollback(ctx context.Context) error {
	var removed []BlockRef

	for i := len(s.recent) - 1; i >= 0; i-- {
		ref := s.recent[i]

		onChain, err := s.isOnChain(ctx, ref)
		if err != nil {
			// a height out of range is a lagging backend, or a new chain
			// with fewer blocks; either way, retry rather than guess
			return fmt.Errorf("failed to read block %d while looking for the reorg ancestor: %w", ref.Height, err)
		}

		if !onChain {
			removed = append(removed, ref)
			continue
		}

		s.recent = s.recent[:i+1]
		s.setCursor(ref)

		if len(removed) == 0 {
			// the parent check failed but nothing changed below the cursor:
			// the backend that answered was briefly on another chain. Fail
			// so the read is retried after a backoff instead of spinning.
			return fmt.Errorf("block %d does not extend the delivered block %s on this node", ref.Height+1, ref)
		}

		log.Warnf("btc stream: reorg of %d block(s), ancestor %s", len(removed), ref)

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
