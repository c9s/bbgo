package marketdata

import (
	"context"
	"sync"
)

// OverflowPolicy decides what a ChanCursor does when its buffer is full.
type OverflowPolicy uint8

const (
	// OverflowBlock blocks the producer until the consumer catches up. This is
	// the correct policy for replay: for a gRPC stream it propagates through
	// HTTP/2 flow control back to the server, and for a file reader it simply
	// throttles decoding.
	OverflowBlock OverflowPolicy = iota

	// OverflowFail terminates the cursor with ErrOverflow instead of blocking.
	// Only appropriate for a genuinely live source that must not stall a shared
	// transport.
	//
	// There is deliberately no "drop oldest" policy: silently discarding market
	// data is how a backtest produces confidently wrong results.
	OverflowFail
)

// DefaultChanBufferSize is the buffer used when NewChanCursor is given a
// non-positive size.
const DefaultChanBufferSize = 1024

// ChanCursor adapts a push-style producer to the pull Cursor interface.
//
// The producer runs in its own goroutine and calls Push, then exactly one of
// Finish or Fail. Push reports false once the consumer has closed the cursor or
// the context is done, which is the producer's signal to return.
type ChanCursor struct {
	ch   chan *Event
	done chan struct{}

	ctx      context.Context
	cancel   context.CancelFunc
	overflow OverflowPolicy

	cur *Event

	mu       sync.Mutex
	err      error
	finished bool

	closeOnce  sync.Once
	finishOnce sync.Once
}

// ChanOption configures a ChanCursor.
type ChanOption func(*ChanCursor)

// WithOverflowPolicy sets the policy used when the buffer is full.
func WithOverflowPolicy(p OverflowPolicy) ChanOption {
	return func(c *ChanCursor) { c.overflow = p }
}

// NewChanCursor returns a cursor fed by a producer goroutine. The returned
// cursor is bound to ctx.
func NewChanCursor(ctx context.Context, bufSize int, opts ...ChanOption) *ChanCursor {
	if bufSize <= 0 {
		bufSize = DefaultChanBufferSize
	}

	ctx, cancel := context.WithCancel(ctx)
	c := &ChanCursor{
		ch:     make(chan *Event, bufSize),
		done:   make(chan struct{}),
		ctx:    ctx,
		cancel: cancel,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Context returns the cursor's context. A producer should use it so that
// closing the cursor cancels whatever the producer is blocked on.
func (c *ChanCursor) Context() context.Context { return c.ctx }

// Push delivers an event to the consumer. It returns false once the cursor is
// closed or its context is done, which is the producer's signal to stop.
func (c *ChanCursor) Push(ev *Event) bool {
	// Termination takes precedence over buffer space. Without this check a
	// select with a free buffer slot would pick the send at random even after
	// the consumer closed the cursor, so a producer could keep running long
	// after nobody was reading.
	select {
	case <-c.done:
		return false
	case <-c.ctx.Done():
		return false
	default:
	}

	if c.overflow == OverflowFail {
		select {
		case c.ch <- ev:
			return true
		case <-c.done:
			return false
		case <-c.ctx.Done():
			return false
		default:
			c.Fail(ErrOverflow)
			return false
		}
	}

	select {
	case c.ch <- ev:
		return true
	case <-c.done:
		return false
	case <-c.ctx.Done():
		return false
	}
}

// Fail terminates the cursor with err. Subsequent Push calls return false.
// Only the first Fail or Finish takes effect.
func (c *ChanCursor) Fail(err error) {
	c.mu.Lock()
	if c.err == nil && !c.finished {
		c.err = err
	}
	c.mu.Unlock()

	c.finishOnce.Do(func() { close(c.ch) })
}

// Finish signals clean exhaustion.
func (c *ChanCursor) Finish() {
	c.mu.Lock()
	c.finished = true
	c.mu.Unlock()

	c.finishOnce.Do(func() { close(c.ch) })
}

func (c *ChanCursor) Next() bool {
	c.mu.Lock()
	err := c.err
	c.mu.Unlock()
	if err != nil {
		c.cur = nil
		return false
	}

	select {
	case ev, ok := <-c.ch:
		if !ok {
			c.cur = nil
			return false
		}
		c.cur = ev
		return true

	case <-c.done:
		c.cur = nil
		return false

	case <-c.ctx.Done():
		c.mu.Lock()
		if c.err == nil {
			c.err = c.ctx.Err()
		}
		c.mu.Unlock()
		c.cur = nil
		return false
	}
}

func (c *ChanCursor) Event() *Event { return c.cur }

func (c *ChanCursor) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

// Close releases the cursor and unblocks a producer waiting in Push.
func (c *ChanCursor) Close() error {
	c.closeOnce.Do(func() {
		close(c.done)
		c.cancel()
	})
	c.cur = nil
	return nil
}
