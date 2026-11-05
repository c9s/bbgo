package bfxfunding

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/exchange/bitfinex/bfxapi"
)

// maxTradePageLimit is the max number of trades returned by the public trades endpoint per request
const maxTradePageLimit = 10000

// the public trades endpoint allows about 15 requests per minute per IP,
// so the limiter is shared by all the strategy instances in the process.
var defaultTradeSyncLimiter = rate.NewLimiter(rate.Every(4*time.Second), 1)

// TradeSyncer fetches the public funding trades of a funding symbol page by page in ascending order.
type TradeSyncer struct {
	client  *bfxapi.Client
	symbol  string
	limiter *rate.Limiter
	logger  logrus.FieldLogger

	pageLimit int

	// cursor is the time of the latest fetched trade; the endpoint's start parameter is inclusive,
	// so the IDs of the trades at the cursor time are kept to filter out the duplicates.
	cursor    time.Time
	cursorIDs map[int64]struct{}
}

func NewTradeSyncer(client *bfxapi.Client, symbol string, logger logrus.FieldLogger) *TradeSyncer {
	return &TradeSyncer{
		client:    client,
		symbol:    symbol,
		limiter:   defaultTradeSyncLimiter,
		logger:    logger,
		pageLimit: maxTradePageLimit,
		cursorIDs: make(map[int64]struct{}),
	}
}

// SetCursor sets the position to resume from: the time of the latest known trade and the IDs of the trades at that time.
func (s *TradeSyncer) SetCursor(t time.Time, ids []int64) {
	s.cursor = t
	s.cursorIDs = make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		s.cursorIDs[id] = struct{}{}
	}
}

// Sync fetches the trades from the cursor (or since, whichever is later) until now.
// onPage is called with the new trades of each page so that the progress can be persisted.
func (s *TradeSyncer) Sync(ctx context.Context, since time.Time, onPage func(trades []FundingTrade) error) (int, error) {
	if s.cursor.Before(since) {
		s.SetCursor(since, nil)
	}

	total := 0
	for {
		if err := s.limiter.Wait(ctx); err != nil {
			return total, err
		}

		start := s.cursor
		resp, err := s.client.NewGetPublicTradeHistoryBySymbolRequest().
			Symbol(s.symbol).
			Start(start).
			Limit(s.pageLimit).
			Sort(1).
			Do(ctx)
		if err != nil {
			return total, fmt.Errorf("unable to query public funding trades of %s since %s: %w", s.symbol, start, err)
		}

		page := resp.FundingTrades
		trades := make([]FundingTrade, 0, len(page))
		for _, pt := range page {
			t := toFundingTrade(pt)
			if t.Time.Before(s.cursor) {
				continue
			}

			if t.Time.Equal(s.cursor) {
				if _, seen := s.cursorIDs[t.ID]; seen {
					continue
				}
			}

			trades = append(trades, t)
		}

		if len(trades) > 0 {
			s.advance(trades)

			if onPage != nil {
				if err := onPage(trades); err != nil {
					return total, err
				}
			}

			total += len(trades)
			s.logger.Debugf("synced %d %s funding trades, cursor: %s", len(trades), s.symbol, s.cursor)
		}

		if len(page) < s.pageLimit {
			return total, nil
		}

		// a full page of trades sharing the same timestamp would never move the cursor forward
		if !s.cursor.After(start) {
			s.SetCursor(start.Add(time.Millisecond), nil)
		}
	}
}

func (s *TradeSyncer) advance(trades []FundingTrade) {
	for _, t := range trades {
		if t.Time.After(s.cursor) {
			s.cursor = t.Time
			s.cursorIDs = make(map[int64]struct{})
		}

		if t.Time.Equal(s.cursor) {
			s.cursorIDs[t.ID] = struct{}{}
		}
	}
}

func toFundingTrade(pt bfxapi.PublicFundingTrade) FundingTrade {
	return FundingTrade{
		ID:     pt.ID,
		Time:   pt.CreatedAt.Time(),
		Amount: pt.Amount.Abs().Float64(),
		Rate:   pt.Rate.Float64(),
		Period: pt.Period,
	}
}
