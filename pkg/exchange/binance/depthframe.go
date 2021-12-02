package binance

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/adshao/go-binance/v2"
)

//go:generate callbackgen -type DepthFrame
type DepthFrame struct {
	Symbol string

	client *binance.Client

	context context.Context

	snapshotMutex sync.Mutex
	snapshotDepth *DepthEvent

	bufMutex  sync.Mutex
	bufEvents []DepthEvent

	resetC chan struct{}
	once   sync.Once

	readyCallbacks []func(snapshotDepth DepthEvent, bufEvents []DepthEvent)
	pushCallbacks  []func(e DepthEvent)
}

func (f *DepthFrame) reset() {
	if debugBinanceDepth {
		log.Infof("resetting %s depth frame", f.Symbol)
	}

	f.bufMutex.Lock()
	f.bufEvents = nil
	f.bufMutex.Unlock()

	f.snapshotMutex.Lock()
	f.snapshotDepth = nil
	f.once = sync.Once{}
	f.snapshotMutex.Unlock()
}

func (f *DepthFrame) emitReset() {
	select {
	case f.resetC <- struct{}{}:
	default:
	}
}

func (f *DepthFrame) bufferEvent(e DepthEvent) {
	if debugBinanceDepth {
		log.Infof("buffering %s depth event FirstUpdateID = %d, FinalUpdateID = %d", f.Symbol, e.FirstUpdateID, e.FinalUpdateID)
	}

	f.bufMutex.Lock()
	f.bufEvents = append(f.bufEvents, e)
	f.bufMutex.Unlock()
}

func (f *DepthFrame) loadDepthSnapshot() error {
	if debugBinanceDepth {
		log.Infof("buffering %s depth events...", f.Symbol)
	}

	time.Sleep(3 * time.Second)

	depth, err := f.fetch(f.context)
	if err != nil {
		return err
	}

	if len(depth.Asks) == 0 {
		return fmt.Errorf("%s depth response error: empty asks", f.Symbol)
	}

	if len(depth.Bids) == 0 {
		return fmt.Errorf("%s depth response error: empty bids", f.Symbol)
	}

	if debugBinanceDepth {
		log.Infof("fetched %s depth, last update ID = %d", f.Symbol, depth.FinalUpdateID)
	}

	// filter the events by the event IDs
	f.bufMutex.Lock()
	bufEvents := f.bufEvents
	f.bufEvents = nil
	f.bufMutex.Unlock()

	var events []DepthEvent
	for _, e := range bufEvents {
		if e.FinalUpdateID < depth.FinalUpdateID {
			if debugBinanceDepth {
				log.Infof("DROP %s depth event (final update id is %d older than the last update), updateID %d ~ %d (len %d)",
					f.Symbol,
					depth.FinalUpdateID-e.FinalUpdateID,
					e.FirstUpdateID, e.FinalUpdateID, e.FinalUpdateID-e.FirstUpdateID)
			}

			continue
		}

		if debugBinanceDepth {
			log.Infof("KEEP %s depth event, updateID %d ~ %d (len %d)",
				f.Symbol,
				e.FirstUpdateID, e.FinalUpdateID, e.FinalUpdateID-e.FirstUpdateID)
		}

		events = append(events, e)
	}

	// since we're buffering the update events, ideally the some of the head events
	// should be older than the received depth snapshot.
	// if the head event is newer than the depth we got,
	// then there are something missed, we need to restart the process.
	if len(events) > 0 {
		// The first processed event should have U (final update ID) <= lastUpdateId+1 AND (first update id) >= lastUpdateId+1.
		firstEvent := events[0]

		// valid
		nextID := depth.FinalUpdateID + 1
		if firstEvent.FirstUpdateID > nextID || firstEvent.FinalUpdateID < nextID {
			return fmt.Errorf("mismatch %s final update id for order book, resetting depth", f.Symbol)
		}

		if debugBinanceDepth {
			log.Infof("VALID first %s depth event, updateID %d ~ %d (len %d)",
				f.Symbol,
				firstEvent.FirstUpdateID, firstEvent.FinalUpdateID, firstEvent.FinalUpdateID-firstEvent.FirstUpdateID)
		}
	}

	if debugBinanceDepth {
		log.Infof("READY %s depth, %d bufferred events", f.Symbol, len(events))
	}

	f.snapshotMutex.Lock()
	f.snapshotDepth = depth
	f.snapshotMutex.Unlock()

	f.EmitReady(*depth, events)
	return nil
}

func (f *DepthFrame) PushEvent(e DepthEvent) {
	select {
	case <-f.resetC:
		f.reset()
	default:
	}

	f.snapshotMutex.Lock()
	snapshot := f.snapshotDepth
	f.snapshotMutex.Unlock()

	// before the snapshot is loaded, we need to buffer the events until we loaded the snapshot.
	if snapshot == nil {
		// buffer the events until we loaded the snapshot
		f.bufferEvent(e)

		go f.once.Do(func() {
			if err := f.loadDepthSnapshot(); err != nil {
				log.WithError(err).Errorf("%s depth snapshot load failed, resetting..", f.Symbol)
				f.emitReset()
			}
		})
		return
	}

	// drop old events
	if e.FinalUpdateID <= snapshot.FinalUpdateID {
		log.Infof("DROP %s depth update event, updateID %d ~ %d (len %d)",
			f.Symbol,
			e.FirstUpdateID, e.FinalUpdateID, e.FinalUpdateID-e.FirstUpdateID)
		return
	}

	if e.FirstUpdateID > snapshot.FinalUpdateID+1 {
		log.Infof("MISSING %s depth update event, resetting, updateID %d ~ %d (len %d)",
			f.Symbol,
			e.FirstUpdateID, e.FinalUpdateID, e.FinalUpdateID-e.FirstUpdateID)

		f.emitReset()
		return
	}

	f.snapshotMutex.Lock()
	f.snapshotDepth.FinalUpdateID = e.FinalUpdateID
	f.snapshotMutex.Unlock()
	f.EmitPush(e)
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
		Symbol:        f.Symbol,
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