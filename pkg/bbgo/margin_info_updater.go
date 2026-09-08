package bbgo

import (
	"context"
	"fmt"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util/timejitter"
)

type MaxBorrowableCallback func(asset string, amount fixedpoint.Value)

type MarginInfoUpdaterConfig struct {
	Interval          types.Duration `json:"marginInfoUpdaterInterval" yaml:"marginInfoUpdaterInterval"`
	BatchSize         int            `json:"marginInfoUpdaterBatchSize" yaml:"marginInfoUpdaterBatchSize"`
	CoolDown          types.Duration `json:"marginInfoUpdaterCooldown" yaml:"marginInfoUpdaterCooldown"`
	TradesBufferCount int            `json:"marginInfoUpdaterTradesBufferCount" yaml:"marginInfoUpdaterTradesBufferCount"`
	BindSession       bool           `json:"marginInfoUpdaterBindSession" yaml:"marginInfoUpdaterBindSession"`
}

func (c *MarginInfoUpdaterConfig) Defaults() {
	if c.Interval == 0 {
		c.Interval = defaultMarginInfoUpdaterInterval
	}
	if c.BatchSize == 0 {
		c.BatchSize = defaultMarginInfoUpdateBatchSize
	}
	if c.CoolDown == 0 {
		c.CoolDown = defaultMarginInfoUpdaterCooldown
	}
	if c.TradesBufferCount == 0 {
		c.TradesBufferCount = 30
	}
}

//go:generate callbackgen -type MarginInfoUpdater
type MarginInfoUpdater struct {
	service types.MarginBorrowRepayService
	config  MarginInfoUpdaterConfig

	assets map[string]fixedpoint.Value

	doneAssets        map[string]struct{}
	lastResetDoneTime time.Time

	boundSession    *ExchangeSession
	firstTradeTimes map[string]time.Time
	tradeCounts     map[string]int
	dirtyAssets     map[string]struct{}

	maxBorrowableCallbacks []MaxBorrowableCallback

	mu sync.Mutex
}

func NewMarginInfoUpdater(
	service types.MarginBorrowRepayService,
	config MarginInfoUpdaterConfig,
) *MarginInfoUpdater {
	return &MarginInfoUpdater{
		assets:     make(map[string]fixedpoint.Value),
		doneAssets: make(map[string]struct{}),
		service:    service,
		config:     config,
	}
}

// Run starts the update workers for the given interval.
func (m *MarginInfoUpdater) Run(ctx context.Context) {
	ticker := time.NewTicker(timejitter.Milliseconds(m.config.Interval.Duration(), 500))
	defer ticker.Stop()

	m.Update(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			if !m.lastResetDoneTime.IsZero() && now.Sub(m.lastResetDoneTime) <= m.config.CoolDown.Duration() {
				// still in cooldown period, skip this update cycle
				continue
			}
			m.Update(ctx)
		}
	}
}

// AddBorrowableAssets adds the assets to the updater.
func (m *MarginInfoUpdater) AddBorrowableAssets(
	assets ...string,
) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, asset := range assets {
		m.assets[asset] = fixedpoint.Zero
	}
}

func (m *MarginInfoUpdater) GetMaxBorrowable(asset string) (fixedpoint.Value, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	amount, ok := m.assets[asset]
	return amount, ok
}

// Update queries the max borrowable amount for each asset and emit update events.
func (m *MarginInfoUpdater) Update(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pending := m.getPendingAssets()
	if len(pending) == 0 {
		return
	}

	batchSize := m.config.BatchSize
	if batchSize > len(pending) {
		batchSize = len(pending)
	}

	for _, asset := range pending[:batchSize] {
		maxBorrowable, err := m.service.QueryMarginAssetMaxBorrowable(
			ctx,
			asset,
		)

		if err != nil {
			log.WithError(err).Errorf("query margin asset max borrowable error: %s", asset)
			continue
		}

		m.assets[asset] = maxBorrowable
		m.doneAssets[asset] = struct{}{}
		if m.dirtyAssets != nil {
			delete(m.dirtyAssets, asset)
		}

		m.EmitMaxBorrowable(asset, maxBorrowable)
	}

	// once every asset has been refreshed, reset to begin a new cycle.
	if len(m.doneAssets) >= len(m.assets) {
		m.doneAssets = make(map[string]struct{})
		m.lastResetDoneTime = time.Now()
	}
}

func (m *MarginInfoUpdater) Bind(session *ExchangeSession) error {
	if session.PublicOnly {
		return fmt.Errorf("cannot bind margin info updater to public only session: %s", session.Name)
	}
	if m.boundSession != nil {
		return fmt.Errorf("margin info updater is already bound to session: %s", m.boundSession.Name)
	}
	// initialize the required fields for tracking trade updates and dirty assets
	m.boundSession = session
	m.firstTradeTimes = make(map[string]time.Time)
	m.tradeCounts = make(map[string]int)
	m.dirtyAssets = make(map[string]struct{})

	// mark all assets as dirty so that they will be updated on the first run of the updater
	for asset := range m.assets {
		m.dirtyAssets[asset] = struct{}{}
	}

	session.UserDataStream.OnTradeUpdate(func(trade types.Trade) {
		market, found := session.Market(trade.Symbol)
		if !found {
			return
		}
		_, baseFound := m.assets[market.BaseCurrency]
		_, quoteFound := m.assets[market.QuoteCurrency]
		if !baseFound && !quoteFound {
			return
		}
		m.mu.Lock()
		defer m.mu.Unlock()

		// first trade for this symbol
		if _, found := m.firstTradeTimes[trade.Symbol]; !found {
			m.firstTradeTimes[trade.Symbol] = trade.Time.Time()
			m.tradeCounts[trade.Symbol] = 1
			return
		}

		m.tradeCounts[trade.Symbol]++
		numTrades := m.tradeCounts[trade.Symbol]
		firstTradeTime := m.firstTradeTimes[trade.Symbol]
		shouldMarkDirty := numTrades >= m.config.TradesBufferCount || trade.Time.Time().Sub(firstTradeTime) >= m.config.CoolDown.Duration()
		if shouldMarkDirty {
			if baseFound {
				m.dirtyAssets[market.BaseCurrency] = struct{}{}
			}
			if quoteFound {
				m.dirtyAssets[market.QuoteCurrency] = struct{}{}
			}
			m.firstTradeTimes[trade.Symbol] = trade.Time.Time()
			m.tradeCounts[trade.Symbol] = 1
		}
	})

	return nil
}

func (m *MarginInfoUpdater) getPendingAssets() []string {
	if m.boundSession != nil {
		return m.getPendingAssetsBoundSession()
	}
	return m.getPendingAssetsDefault()
}

func (m *MarginInfoUpdater) getPendingAssetsBoundSession() []string {
	pending := make([]string, 0, len(m.assets))
	for asset := range m.assets {
		_, done := m.doneAssets[asset]
		_, dirty := m.dirtyAssets[asset]
		if !done && dirty {
			pending = append(pending, asset)
		}
	}
	return pending
}

func (m *MarginInfoUpdater) getPendingAssetsDefault() []string {
	pending := make([]string, 0, len(m.assets))
	for asset := range m.assets {
		if _, done := m.doneAssets[asset]; !done {
			pending = append(pending, asset)
		}
	}
	return pending
}
