package depth

import (
	"fmt"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
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

	fetchC chan struct{}
	mu     sync.Mutex
	once   util.Reonce

	// updateTimeout the timeout duration when not receiving update messages
	updateTimeout time.Duration

	// bufferingPeriod is used to buffer the update message before we get the full depth
	bufferingPeriod time.Duration
}

func NewBuffer(fetcher SnapshotFetcher, bufferingPeriod time.Duration) *Buffer {
	return &Buffer{
		fetcher:         fetcher,
		fetchC:          make(chan struct{}, 1),
		bufferingPeriod: bufferingPeriod,
	}
}

func (b *Buffer) SetUpdateTimeout(d time.Duration) {
	b.updateTimeout = d
}

func (b *Buffer) resetSnapshot() {
	b.snapshot = nil
	b.finalUpdateID = 0
}

// emitFetch emits the fetch signal, and in the next call of AddUpdate, the buffer will try to fetch the snapshot
// if the fetch signal is already emitted, it will be ignored
func (b *Buffer) emitFetch() {
	select {
	case b.fetchC <- struct{}{}:
	default:
	}
}

func (b *Buffer) Reset() {
	b.mu.Lock()
	b.resetSnapshot()
	b.emitFetch()
	b.mu.Unlock()

	b.EmitReset()
}

func (b *Buffer) SetSnapshot(snapshot types.SliceOrderBook, firstUpdateID int64, finalArgs ...int64) error {
	finalUpdateID := firstUpdateID
	if len(finalArgs) > 0 {
		finalUpdateID = finalArgs[0]
	}

	b.mu.Lock()

	if b.finalUpdateID >= finalUpdateID {
		b.mu.Unlock()
		return nil
	}

	// set the final update ID so that we will know if there is an update missing
	b.finalUpdateID = finalUpdateID

	// set the snapshot
	b.snapshot = &snapshot

	b.mu.Unlock()

	// should unlock first then call ready
	b.EmitReady(snapshot, nil)
	return nil
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

	// if the snapshot is set to nil, we need to buffer the message
	b.mu.Lock()

	select {
	case <-b.fetchC:
		b.buffer = append(b.buffer, u)
		b.resetSnapshot()
		b.once.Reset()
		b.once.Do(func() {
			go b.tryFetch()
		})
		b.mu.Unlock()
		return nil

	default:

	}

	// snapshot is nil means we haven't fetched the snapshot yet
	// we need to buffer the message
	if b.snapshot == nil {
		b.buffer = append(b.buffer, u)
		b.once.Do(func() {
			go b.tryFetch()
		})
		b.mu.Unlock()
		return nil
	}

	// if it's ready, then we have the snapshot, we can push the update

	// skip older events
	if u.FinalUpdateID <= b.finalUpdateID {
		log.Infof("the final update id %d of event is less than equal to the final update id %d of the snapshot, skip", u.FinalUpdateID, b.finalUpdateID)
		b.mu.Unlock()
		return nil
	}

	// if there is a missing update, we should reset the snapshot and re-fetch the snapshot
	if u.FirstUpdateID > b.finalUpdateID+1 {
		// drop the prior updates in the buffer since it's corrupted
		b.buffer = []Update{u}
		b.resetSnapshot()
		b.once.Reset()
		b.once.Do(func() {
			go b.tryFetch()
		})

		b.mu.Unlock()
		b.EmitReset()

		return fmt.Errorf("found missing update between finalUpdateID %d and firstUpdateID %d, diff: %d",
			b.finalUpdateID+1,
			u.FirstUpdateID,
			u.FirstUpdateID-b.finalUpdateID)
	}

	log.Debugf("depth update id %d -> %d", b.finalUpdateID, u.FinalUpdateID)
	b.finalUpdateID = u.FinalUpdateID
	b.mu.Unlock()

	b.EmitPush(u)

	return nil
}

// tryFetch tries to fetch the snapshot and push the updates
func (b *Buffer) tryFetch() {
	for {
		<-time.After(b.bufferingPeriod)

		err := b.fetchAndPush()
		if err != nil {
			log.WithError(err).Errorf("snapshot fetch failed, retry in %s", b.bufferingPeriod)
			continue
		}

		break
	}
}

func (b *Buffer) fetchAndPush() error {
	book, finalUpdateID, err := b.fetcher()
	if err != nil {
		return err
	}

	b.mu.Lock()
	log.Debugf("fetched depth snapshot, final update id %d", finalUpdateID)

	if len(b.buffer) > 0 {
		// the snapshot is too early, we should re-fetch the snapshot
		if finalUpdateID < b.buffer[0].FirstUpdateID-1 {
			b.mu.Unlock()
			return fmt.Errorf("depth snapshot is too early, final update %d is < the first update id %d", finalUpdateID, b.buffer[0].FirstUpdateID)
		}
	}

	var pushUpdates []Update
	for idx, u := range b.buffer {
		// skip old events
		if u.FinalUpdateID <= finalUpdateID {
			continue
		}

		if u.FirstUpdateID > finalUpdateID+1 {
			// drop prior updates in the buffer since it's corrupted
			b.buffer = b.buffer[idx:]
			b.mu.Unlock()
			return fmt.Errorf("there is a missing depth update, the update id %d > final update id %d + 1", u.FirstUpdateID, finalUpdateID)
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

	// should unlock first then call ready
	b.EmitReady(book, pushUpdates)
	return nil
}
