package marketdata

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/types"
)

// SequenceMode selects how strictly BookState validates update continuity.
type SequenceMode uint8

const (
	// SequenceNone performs no validation. For sources that carry no sequence
	// numbers at all.
	SequenceNone SequenceMode = iota

	// SequenceMonotonic requires that the sequence never goes backwards. Gaps
	// are reported through OnGap but tolerated. This is the default, because
	// exploratory work should not hard-fail.
	SequenceMonotonic

	// SequenceContiguous requires each update to chain onto the previous one.
	// A gap is an error. The backtest engine should use this: a silently
	// corrupted book produces plausible-looking wrong fills.
	SequenceContiguous
)

// BookState applies snapshot and update events to an order book and validates
// sequence continuity.
//
// It is the consumer side of the L2 contract: the source layer never holds book
// state, never reorders, and never repairs a gap. This type is what a strategy,
// a test, or eventually the backtest exchange uses to turn the event stream
// back into a book.
//
// It deliberately does not reuse pkg/depth.Buffer, which is built for the live
// case: that type owns a SnapshotFetcher issuing REST calls, buffers for a
// wall-clock period, and resets on a time.Timer. All three are meaningless
// under simulated time. Its one transferable idea — checkPreviousID — is what
// SequenceContiguous generalizes.
type BookState struct {
	// Book receives the applied events. If nil, NewBookState creates one.
	Book *types.MutexOrderBook

	// Mode selects sequence validation strictness.
	Mode SequenceMode

	// OnGap, when set, is called just before a contiguity violation is
	// returned, with the sequence the book is at and the one the update claimed
	// to follow. It is a hook for metrics; Apply still returns the error.
	OnGap func(have, claimed uint64)

	// CheckCrossed makes Apply verify the book is still valid after each event.
	// It costs a scan of both sides, so it is off by default.
	CheckCrossed bool

	lastSeq    uint64
	ready      bool
	unverified int64
}

// NewBookState returns a BookState for symbol with the default mode.
func NewBookState(symbol string, exchange types.ExchangeName) *BookState {
	return &BookState{
		Book: types.NewMutexOrderBook(symbol, exchange),
		Mode: SequenceMonotonic,
	}
}

// Ready reports whether a snapshot has initialized the book.
func (s *BookState) Ready() bool { return s.ready }

// LastSequence returns the sequence of the last applied event, or zero.
func (s *BookState) LastSequence() uint64 { return s.lastSeq }

// Unverified returns how many updates were accepted under SequenceContiguous
// without their contiguity being provable, because the source did not supply
// Event.PrevSeq. A non-zero count means the book may have holes that no check
// can see, so it is worth surfacing rather than assuming success.
func (s *BookState) Unverified() int64 { return s.unverified }

// Reset drops the book contents and returns to the not-ready state.
func (s *BookState) Reset() {
	if s.Book != nil {
		s.Book.Reset()
	}
	s.lastSeq = 0
	s.ready = false
	s.unverified = 0
}

// Apply applies a book event.
//
// It returns ErrBookNotReady for an update that arrives before any snapshot,
// ErrBookGap for a sequence violation under SequenceContiguous, and
// ErrBookCrossed when CheckCrossed is set and the result is invalid. Events
// that are not book events are ignored and return nil, so a consumer can feed
// it the whole merged stream.
func (s *BookState) Apply(ev *Event) error {
	switch ev.Type {
	case EventTypeBookSnapshot:
		if ev.Book == nil {
			return fmt.Errorf("marketdata: %s event has no book payload", ev.Type)
		}
		s.Book.Load(*ev.Book)
		s.lastSeq = ev.Key.Seq
		s.ready = true

	case EventTypeBookUpdate:
		if ev.Book == nil {
			return fmt.Errorf("marketdata: %s event has no book payload", ev.Type)
		}
		if !s.ready {
			return fmt.Errorf("%w: %s update at %s", ErrBookNotReady, ev.Symbol, ev.Time())
		}
		if err := s.checkSequence(ev); err != nil {
			return err
		}
		s.Book.Update(*ev.Book)
		if ev.Key.Seq != 0 {
			s.lastSeq = ev.Key.Seq
		}

	default:
		return nil
	}

	if s.CheckCrossed {
		if ok, err := s.Book.IsValid(); !ok {
			return fmt.Errorf("%w: %s at %s: %v", ErrBookCrossed, ev.Symbol, ev.Time(), err)
		}
	}

	return nil
}

func (s *BookState) checkSequence(ev *Event) error {
	if s.Mode == SequenceNone || ev.Key.Seq == 0 || s.lastSeq == 0 {
		return nil
	}

	// Going backwards is a gap under every mode that checks anything: the
	// stream has been reordered or events have been replayed.
	if ev.Key.Seq < s.lastSeq {
		return fmt.Errorf("%w: %s sequence went backwards, have %d got %d",
			ErrBookGap, ev.Symbol, s.lastSeq, ev.Key.Seq)
	}

	if s.Mode != SequenceContiguous {
		return nil
	}

	if ev.PrevSeq == 0 {
		// The venue did not say what this event follows, so nothing stronger
		// than monotonicity can be proven. Count it instead of inventing a
		// rule: requiring Seq == lastSeq+1 would report a gap on every Binance
		// diff, because "u" counts individual updates rather than events.
		s.unverified++
		return nil
	}

	if ev.PrevSeq != s.lastSeq {
		if s.OnGap != nil {
			s.OnGap(s.lastSeq, ev.PrevSeq)
		}
		return fmt.Errorf("%w: %s update claims to follow %d but the book is at %d",
			ErrBookGap, ev.Symbol, ev.PrevSeq, s.lastSeq)
	}

	return nil
}
