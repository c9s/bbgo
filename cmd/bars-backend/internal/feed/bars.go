package feed

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

// tradeHistoryQuerier is implemented by exchanges that expose a public/
// market-data historical trades endpoint (e.g. Binance's GET /api/v3/historicalTrades,
// which still requires an API key for rate-limit tracking even though it needs
// no trading permission). types.Exchange itself does not include this method:
// QueryTrades on that interface is the authenticated "my trades" account
// history call, not a venue-wide public trade feed.
type tradeHistoryQuerier interface {
	QueryHistoricalTrades(ctx context.Context, symbol string, limit uint64) ([]types.Trade, error)
}

// BarType selects which indicator/v2 bar aggregation to use.
type BarType string

const (
	BarTypeTick   BarType = "tick"
	BarTypeVolume BarType = "volume"
)

// BarKey identifies one (symbol, bar type, parameter) tick/volume bar series.
// Param is the raw string form of numTicks (tick bars) or barVolume (volume bars),
// kept as a string so it can double as a map key without re-formatting fixedpoint values.
type BarKey struct {
	Symbol string
	Type   BarType
	Param  string
}

// ParseBarKey validates symbol/barType/param and builds a BarKey.
func ParseBarKey(symbol string, barType BarType, param string) (BarKey, error) {
	switch barType {
	case BarTypeTick:
		n, err := strconv.Atoi(param)
		if err != nil || n <= 0 {
			return BarKey{}, fmt.Errorf("invalid tick count param %q", param)
		}
	case BarTypeVolume:
		v, err := fixedpoint.NewFromString(param)
		if err != nil || v.Sign() <= 0 {
			return BarKey{}, fmt.Errorf("invalid volume threshold param %q", param)
		}
	default:
		return BarKey{}, fmt.Errorf("unknown bar type %q, must be %q or %q", barType, BarTypeTick, BarTypeVolume)
	}

	return BarKey{Symbol: symbol, Type: barType, Param: param}, nil
}

// barStream is the subset of TickBarStream/VolumeBarStream's API that BarManager needs.
type barStream interface {
	Tail(n int) types.KLineWindow
	OnUpdate(cb func(k types.KLine))
}

// barEntry owns one live bar aggregation plus its own fan-out subscriber set.
//
// It registers exactly one OnUpdate callback on the underlying bar stream (in
// newBarEntry) and fans updates out to a dynamic set of subscriber channels;
// the generated OnUpdate/EmitUpdate callbacks on TickBarStream/VolumeBarStream
// have no "off", so subscribing/unsubscribing directly against them would leak
// one callback per WS connection for the lifetime of the process.
type barEntry struct {
	stream barStream
	local  *types.StandardStream

	mu   sync.RWMutex
	subs map[chan types.KLine]struct{}
}

func newBarEntry(stream barStream, local *types.StandardStream) *barEntry {
	e := &barEntry{
		stream: stream,
		local:  local,
		subs:   make(map[chan types.KLine]struct{}),
	}
	stream.OnUpdate(e.broadcast)
	return e
}

func (e *barEntry) broadcast(k types.KLine) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for ch := range e.subs {
		select {
		case ch <- k:
		default:
		}
	}
}

func (e *barEntry) subscribe() (chan types.KLine, func()) {
	ch := make(chan types.KLine, 64)

	e.mu.Lock()
	e.subs[ch] = struct{}{}
	e.mu.Unlock()

	cancel := func() {
		e.mu.Lock()
		delete(e.subs, ch)
		e.mu.Unlock()
	}

	return ch, cancel
}

// BarManager lazily builds a TickBarStream or VolumeBarStream per (symbol, type,
// param) key, backfills it from recent REST trades, and keeps it live by
// re-emitting market trades from the main exchange stream into it.
type BarManager struct {
	tradeHistory tradeHistoryQuerier // nil if the exchange has no credentials configured
	limit        int

	mu      sync.Mutex
	entries map[BarKey]*barEntry
}

// NewBarManager creates a BarManager that backfills/keeps at most limit bars per
// series. If ex does not implement QueryHistoricalTrades (e.g. an anonymous
// public-only exchange instance with no API key configured), every bar series
// will fail to backfill with a descriptive error, but KLineManager is unaffected.
func NewBarManager(ex types.Exchange, limit int) *BarManager {
	tradeHistory, _ := ex.(tradeHistoryQuerier)
	return &BarManager{
		tradeHistory: tradeHistory,
		limit:        limit,
		entries:      make(map[BarKey]*barEntry),
	}
}

func (m *BarManager) getOrCreate(ctx context.Context, key BarKey) (*barEntry, error) {
	m.mu.Lock()
	if e, ok := m.entries[key]; ok {
		m.mu.Unlock()
		return e, nil
	}
	m.mu.Unlock()

	local := types.NewStandardStream()
	localPtr := &local

	var stream barStream
	switch key.Type {
	case BarTypeTick:
		n, err := strconv.Atoi(key.Param)
		if err != nil {
			return nil, fmt.Errorf("invalid tick count param %q: %w", key.Param, err)
		}
		stream = indicatorv2.TickBars(localPtr, key.Symbol, n)
	case BarTypeVolume:
		v, err := fixedpoint.NewFromString(key.Param)
		if err != nil {
			return nil, fmt.Errorf("invalid volume threshold param %q: %w", key.Param, err)
		}
		stream = indicatorv2.VolumeBars(localPtr, key.Symbol, v)
	default:
		return nil, fmt.Errorf("unknown bar type %q", key.Type)
	}

	entry := newBarEntry(stream, localPtr)

	if m.tradeHistory == nil {
		return nil, fmt.Errorf(
			"exchange does not support historical trade backfill (QueryHistoricalTrades); " +
				"tick/volume bars need it to seed history, set the exchange's API key/secret env vars (even a key without trading permission is enough)")
	}

	// QueryHistoricalTrades already returns trades sorted ascending by time.
	trades, err := m.tradeHistory.QueryHistoricalTrades(ctx, key.Symbol, uint64(m.limit))
	if err != nil {
		return nil, fmt.Errorf("backfill trades for %s failed: %w", key.Symbol, err)
	}

	for _, t := range trades {
		localPtr.EmitMarketTrade(t)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.entries[key]; ok {
		// another request raced us and created the entry first; keep the winner.
		return existing, nil
	}
	m.entries[key] = entry
	return entry, nil
}

// Snapshot ensures the series exists (creating + backfilling it on first use)
// and returns its current in-memory bars.
func (m *BarManager) Snapshot(ctx context.Context, key BarKey) ([]types.KLine, error) {
	entry, err := m.getOrCreate(ctx, key)
	if err != nil {
		return nil, err
	}
	return entry.stream.Tail(m.limit), nil
}

// Subscribe ensures the series exists and registers a channel for live updates.
// The returned cancel function must be called to stop receiving updates.
func (m *BarManager) Subscribe(ctx context.Context, key BarKey) (<-chan types.KLine, func(), error) {
	entry, err := m.getOrCreate(ctx, key)
	if err != nil {
		return nil, nil, err
	}

	ch, cancel := entry.subscribe()
	return ch, cancel, nil
}

// handleTrade is registered as the main stream's OnMarketTrade callback: it
// re-emits every trade into the local bridge stream of every bar series that
// tracks the trade's symbol, driving their tick/volume aggregation forward.
func (m *BarManager) handleTrade(trade types.Trade) {
	m.mu.Lock()
	var matches []*barEntry
	for key, e := range m.entries {
		if key.Symbol == trade.Symbol {
			matches = append(matches, e)
		}
	}
	m.mu.Unlock()

	for _, e := range matches {
		e.local.EmitMarketTrade(trade)
	}
}
