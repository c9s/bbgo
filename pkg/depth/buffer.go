package depth

import (
	"fmt"
	"sync"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
	log "github.com/sirupsen/logrus"
)

type SnapshotFetcher func() (snapshot types.SliceOrderBook, finalUpdateID int64, err error)

type Update struct {
	FirstUpdateID, FinalUpdateID int64

	// Object is the update object
	Object types.SliceOrderBook
}

//go:generate callbackgen -type Buffer
type Buffer struct {
	buffer []Update

	finalUpdateID int64
	fetcher       SnapshotFetcher
	snapshot      *types.SliceOrderBook

	resetCallbacks []func()
	readyCallbacks []func(snapshot types.SliceOrderBook, updates []Update)
	pushCallbacks  []func(update Update)

	resetC chan struct{}
	mu     sync.Mutex
	once   util.Reonce
}

func NewBuffer(fetcher SnapshotFetcher) *Buffer {
	return &Buffer{
		fetcher: fetcher,
		resetC:  make(chan struct{}, 1),
	}
}

func (b *Buffer) resetSnapshot() {
	b.snapshot = nil
	b.finalUpdateID = 0
	b.EmitReset()
}

func (b *Buffer) emitReset() {
	select {
	case b.resetC <- struct{}{}:
	default:
	}
}

func (b *Buffer) Reset() {
	b.mu.Lock()
	b.resetSnapshot()
	b.emitReset()
	b.mu.Unlock()
}

// AddUpdate adds the update to the buffer or push the update to the subscriber
func (b *Buffer) AddUpdate(o types.SliceOrderBook, firstUpdateID int64, finalArgs ...int64) error {
	finalUpdateID := firstUpdateID
	if len(finalArgs) > 0 {
		finalUpdateID = finalArgs[0]
	}

	u := Update{
		FirstUpdateID: firstUpdateID,
		FinalUpdateID: finalUpdateID,
		Object:        o,
	}

	// we lock here because there might be 2+ calls to the AddUpdate method
	// we don't want to reset sync.Once 2 times here
	b.mu.Lock()
	select {
	case <-b.resetC:
		log.Warnf("received depth reset signal, resetting...")

		// if the once goroutine is still running, overwriting this once might cause "unlock of unlocked mutex" panic.
		b.once.Reset()
	default:
	}

	// if the snapshot is set to nil, we need to buffer the message
	if b.snapshot == nil {
		b.buffer = append(b.buffer, u)
		b.once.Do(func() {
			go b.tryFetch()
		})
		b.mu.Unlock()
		return nil
	}

	// if there is a missing update, we should reset the snapshot and re-fetch the snapshot
	if u.FirstUpdateID > b.finalUpdateID+1 {
		// emitReset will reset the once outside the mutex lock section
		b.buffer = []Update{u}
		b.resetSnapshot()
		b.emitReset()
		b.mu.Unlock()
		return fmt.Errorf("there is a missing update between %d and %d", u.FirstUpdateID, b.finalUpdateID+1)
	}

	b.finalUpdateID = u.FinalUpdateID
	b.EmitPush(u)
	b.mu.Unlock()
	return nil
}

func (b *Buffer) fetchAndPush() error {
	log.Info("fetching depth snapshot...")
	book, finalUpdateID, err := b.fetcher()
	if err != nil {
		return err
	}

	log.Infof("fetched depth snapshot, final update id %d", finalUpdateID)

	b.mu.Lock()
	if len(b.buffer) > 0 {
		// the snapshot is too early
		if finalUpdateID < b.buffer[0].FirstUpdateID {
			b.resetSnapshot()
			b.emitReset()
			b.mu.Unlock()
			return fmt.Errorf("depth final update %d is < the first update id %d", finalUpdateID, b.buffer[0].FirstUpdateID)
		}
	}

	var pushUpdates []Update
	for _, u := range b.buffer {
		if u.FirstUpdateID > finalUpdateID+1 {
			b.resetSnapshot()
			b.emitReset()
			b.mu.Unlock()
			return fmt.Errorf("the update id %d > final update id %d + 1", u.FirstUpdateID, finalUpdateID)
		}

		if u.FirstUpdateID < finalUpdateID+1 {
			continue
		}

		pushUpdates = append(pushUpdates, u)

		// update the final update id to the correct final update id
		finalUpdateID = u.FinalUpdateID
	}

	// clean the buffer since we have filtered out the buffer we want
	b.buffer = nil

	// set the final update ID so that we will know if there is an update missing
	b.finalUpdateID = finalUpdateID

	// set the snapshot
	b.snapshot = &book

	b.mu.Unlock()
	b.EmitReady(book, pushUpdates)
	return nil
}

func (b *Buffer) tryFetch() {
	for {
		err := b.fetchAndPush()
		if err != nil {
			log.WithError(err).Errorf("snapshot fetch failed")
			continue
		}
		break
	}
}
