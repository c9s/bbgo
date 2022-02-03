package bbgo

import (
	"sync"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type Quota struct {
	mu        sync.Mutex
	Available fixedpoint.Value
	Locked    fixedpoint.Value
}

func (q *Quota) Add(fund fixedpoint.Value) {
	q.mu.Lock()
	q.Available = q.Available.Add(fund)
	q.mu.Unlock()
}

func (q *Quota) Lock(fund fixedpoint.Value) bool {
	if fund.Compare(q.Available) > 0 {
		return false
	}

	q.mu.Lock()
	q.Available = q.Available.Sub(fund)
	q.Locked = q.Locked.Add(fund)
	q.mu.Unlock()

	return true
}

func (q *Quota) Commit() {
	q.mu.Lock()
	q.Locked = fixedpoint.Zero
	q.mu.Unlock()
}

func (q *Quota) Rollback() {
	q.mu.Lock()
	q.Available = q.Available.Add(q.Locked)
	q.Locked = fixedpoint.Zero
	q.mu.Unlock()
}

type QuotaTransaction struct {
	mu         sync.Mutex
	BaseAsset  Quota
	QuoteAsset Quota
}

func (m *QuotaTransaction) Commit() bool {
	m.mu.Lock()
	m.BaseAsset.Commit()
	m.QuoteAsset.Commit()
	m.mu.Unlock()
	return true
}

func (m *QuotaTransaction) Rollback() bool {
	m.mu.Lock()
	m.BaseAsset.Rollback()
	m.QuoteAsset.Rollback()
	m.mu.Unlock()
	return true
}
