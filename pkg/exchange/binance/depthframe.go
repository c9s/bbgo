package binance

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/adshao/go-binance/v2"
)

//go:generate callbackgen -type DepthFrame
type DepthFrame struct {
	client  *binance.Client
	context context.Context

	mu            sync.Mutex
	once          sync.Once
	SnapshotDepth *DepthEvent
	Symbol        string
	BufEvents     []DepthEvent

	readyCallbacks []func(snapshotDepth DepthEvent, bufEvents []DepthEvent)
	pushCallbacks  []func(e DepthEvent)
}

func (f *DepthFrame) reset() {
	f.mu.Lock()
	f.SnapshotDepth = nil
	f.BufEvents = nil
	f.mu.Unlock()
}

func (f *DepthFrame) loadDepthSnapshot() {
	f.mu.Lock()

	if debugBinanceDepth {
		log.Infof("loading %s depth from the restful api", f.Symbol)
	}

	depth, err := f.fetch(f.context)
	if err != nil {
		log.WithError(err).Errorf("depth api error")
		f.mu.Unlock()
		return
	}

	if len(depth.Asks) == 0 {
		log.Errorf("depth response error: empty asks")
		f.mu.Unlock()
		return
	}

	if len(depth.Bids) == 0 {
		log.Errorf("depth response error: empty bids")
		f.mu.Unlock()
		return
	}

	// filter the events by the event IDs
	var events []DepthEvent
	for _, e := range f.BufEvents {
		if e.FirstUpdateID <= depth.FinalUpdateID || e.FinalUpdateID <= depth.FinalUpdateID {
			continue
		}

		events = append(events, e)
	}

	// since we're buffering the update events, ideally the some of the head events
	// should be older than the received depth snapshot.
	// if the head event is newer than the depth we got,
	// then there are something missed, we need to restart the process.
	if len(events) > 0 {
		firstEvent := events[0]
		if firstEvent.FirstUpdateID > depth.FinalUpdateID+1 {
			log.Warn("miss matched final update id for order book, resetting depth...")
			f.SnapshotDepth = nil
			f.BufEvents = nil
			f.mu.Unlock()
			return
		}
	}

	f.SnapshotDepth = depth
	f.BufEvents = nil
	f.mu.Unlock()

	f.EmitReady(*depth, events)
}

func (f *DepthFrame) PushEvent(e DepthEvent) {
	f.mu.Lock()

	// before the snapshot is loaded, we need to buffer the events until we loaded the snapshot.
	if f.SnapshotDepth == nil {
		// buffer the events until we loaded the snapshot
		f.BufEvents = append(f.BufEvents, e)
		f.mu.Unlock()

		f.loadDepthSnapshot()

		// start a worker to update the snapshot periodically.
		go f.once.Do(func() {
			if debugBinanceDepth {
				log.Infof("starting depth snapshot updater for %s market", f.Symbol)
			}

			ticker := time.NewTicker(30*time.Minute + time.Duration(rand.Intn(10))*time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-f.context.Done():
					return

				case <-ticker.C:
					f.loadDepthSnapshot()
				}
			}
		})
	} else {
		// if we have the snapshot, we could use that final update ID filter the events

		// too old: drop any update ID < the final update ID
		if e.FinalUpdateID < f.SnapshotDepth.FinalUpdateID {
			if debugBinanceDepth {
				log.Warnf("event final update id %d < depth final update id %d, skip", e.FinalUpdateID, f.SnapshotDepth.FinalUpdateID)
			}

			f.mu.Unlock()
			return
		}

		// too new: if the first update ID > final update ID + 1, it means something is missing, we need to reload.
		if e.FirstUpdateID > f.SnapshotDepth.FinalUpdateID+1 {
			if debugBinanceDepth {
				log.Warnf("event first update id %d > final update id + 1 (%d), resetting snapshot", e.FirstUpdateID, f.SnapshotDepth.FirstUpdateID+1)
			}

			f.SnapshotDepth = nil

			// save the new event for later
			f.BufEvents = append(f.BufEvents, e)
			f.mu.Unlock()
			return
		}

		// update the final update ID, so that we can check the next event
		f.SnapshotDepth.FinalUpdateID = e.FinalUpdateID
		f.mu.Unlock()

		f.EmitPush(e)
	}
}

// fetch fetches the depth and convert to the depth event so that we can reuse the event structure to convert it to the global orderbook type
func (f *DepthFrame) fetch(ctx context.Context) (*DepthEvent, error) {
	if debugBinanceDepth {
		log.Infof("fetching %s depth snapshot", f.Symbol)
	}

	response, err := f.client.NewDepthService().Symbol(f.Symbol).Do(ctx)
	if err != nil {
		return nil, err
	}

	event := DepthEvent{
		FirstUpdateID: 0,
		FinalUpdateID: response.LastUpdateID,
	}

	for _, entry := range response.Bids {
		event.Bids = append(event.Bids, DepthEntry{PriceLevel: entry.Price, Quantity: entry.Quantity})
	}

	for _, entry := range response.Asks {
		event.Asks = append(event.Asks, DepthEntry{PriceLevel: entry.Price, Quantity: entry.Quantity})
	}

	return &event, nil
}
