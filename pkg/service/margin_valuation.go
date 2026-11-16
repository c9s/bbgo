package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"

	"github.com/c9s/bbgo/pkg/cache"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

// valuationQuote is the currency margin interests are valued in
const valuationQuote = "USDT"

// MarginInterestValuationService values the margin_interests rows in USDT at the
// time they were charged and stores the result in margin_interest_valuations.
//
// The price of a charge at t is the close of the 1h kline of <asset>USDT that
// ends at the top of t's hour, i.e. the price at that hour. Without a USDT
// market the inverse USDT<asset> is used; without either the row stays
// unpriced. The asset priced is the one the interest was paid in (BNB when
// Binance converts it).
//
// The table is derived from margin_interests and the kline tables, so it can be
// emptied and recomputed at any time.
type MarginInterestValuationService struct {
	DB *sqlx.DB
}

// spotKLineExchange hides the margin and futures settings of an exchange, so
// the kline sync stores and reads the spot klines of the symbol it is given.
// BacktestService replaces the symbol with the isolated symbol on an isolated
// margin session, and uses the futures table on a futures session.
type spotKLineExchange struct {
	types.Exchange
}

// Sync syncs the 1h klines the unpriced rows of session since since need, then
// values every row that has no valuation yet or is still unpriced. A row whose
// kline cannot be synced is stored unpriced and retried by the next Sync.
func (s *MarginInterestValuationService) Sync(ctx context.Context, ex types.Exchange, session string, since time.Time) error {
	pending, err := s.selectPending(ctx, ex.Name(), session, since)
	if err != nil {
		return err
	}

	if len(pending) == 0 {
		return nil
	}

	markets, err := cache.LoadExchangeMarketsWithCache(ctx, ex)
	if err != nil {
		return fmt.Errorf("unable to load %s markets to value margin interests: %w", ex.Name(), err)
	}

	pairs := make(map[string]pricePair)
	oldest := make(map[string]time.Time)
	for _, p := range pending {
		pair, ok := pairs[p.Asset]
		if !ok {
			pair = findPricePair(p.Asset, markets)
			pairs[p.Asset] = pair
		}

		if pair.symbol == "" {
			continue
		}

		start := priceKLineStart(p.Time.Time())
		if t, ok := oldest[pair.symbol]; !ok || start.Before(t) {
			oldest[pair.symbol] = start
		}
	}

	klines := &BacktestService{DB: s.DB}
	spot := spotKLineExchange{ex}
	now := time.Now()

	var errs []error
	closes := make(map[string]map[int64]fixedpoint.Value, len(oldest))
	for symbol, start := range oldest {
		if err := klines.Sync(ctx, spot, symbol, []types.Interval{types.Interval1h}, start, now); err != nil {
			errs = append(errs, fmt.Errorf("unable to sync %s 1h klines: %w", symbol, err))
		}

		// the klines stored before a failed sync still price their rows
		c, err := s.queryCloses(ctx, spot, symbol, start, now)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		closes[symbol] = c
	}

	for _, p := range pending {
		pair := pairs[p.Asset]
		v := valueInterest(p.MarginInterest, pair, closes[pair.symbol], now)
		if err := s.write(ctx, v, p.ValuationGID.Valid); err != nil {
			return err
		}
	}

	if err := errors.Join(errs...); err != nil {
		util.GetLoggerFromCtx(ctx).WithError(err).Warnf("some %s margin interests of session %q are not priced yet", ex.Name(), session)
		return err
	}

	return nil
}

// pendingInterest is a margin_interests row without a priced valuation
type pendingInterest struct {
	types.MarginInterest

	// ValuationGID is set when the row has an unpriced valuation
	ValuationGID sql.NullInt64 `db:"valuation_gid"`
}

// selectPending selects the rows of session since since with no valuation or an unpriced one
func (s *MarginInterestValuationService) selectPending(
	ctx context.Context, ex types.ExchangeName, session string, since time.Time,
) ([]pendingInterest, error) {
	columns := make([]string, 0, len(marginInterestColumns)+2)
	columns = append(columns, "mi.gid")
	for _, c := range marginInterestColumns {
		columns = append(columns, "mi."+c)
	}
	columns = append(columns, "v.gid AS valuation_gid")

	query, args, err := sq.Select(columns...).
		From("margin_interests mi").
		LeftJoin("margin_interest_valuations v ON v.interest_gid = mi.gid").
		Where(sq.Eq{"mi.exchange": ex, "mi.session": session}).
		Where(sq.GtOrEq{"mi.time": since.UTC()}).
		Where(sq.Or{sq.Eq{"v.gid": nil}, sq.Eq{"v.price_in_usdt": nil}}).
		OrderBy("mi.time ASC").
		ToSql()
	if err != nil {
		return nil, err
	}

	var rows []pendingInterest
	if err := s.DB.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, err
	}
	return rows, nil
}

// queryCloses returns the close of each stored 1h kline of symbol in [start, until], by start time
func (s *MarginInterestValuationService) queryCloses(
	ctx context.Context, ex types.Exchange, symbol string, start, until time.Time,
) (map[int64]fixedpoint.Value, error) {
	query, args, err := sq.Select("start_time", "close").
		From(targetKlineTable(ex)).
		Where(sq.Eq{"symbol": symbol, "`interval`": types.Interval1h.String()}).
		Where(sq.Expr("start_time BETWEEN ? AND ?", start, until)).
		ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := s.DB.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("unable to query %s 1h klines: %w", symbol, err)
	}
	defer rows.Close()

	closes := make(map[int64]fixedpoint.Value)
	for rows.Next() {
		var startTime types.Time
		var closePrice fixedpoint.Value
		if err := rows.Scan(&startTime, &closePrice); err != nil {
			return nil, err
		}
		closes[startTime.Time().UnixMilli()] = closePrice
	}

	return closes, rows.Err()
}

// valuationInsertSQL inserts every column: insertType leaves out a nil pointer
// field and caches its SQL per type, see marginInterestInsertSQL
var valuationInsertSQL = "INSERT INTO margin_interest_valuations (" + strings.Join(valuationColumns, ", ") +
	") VALUES (:" + strings.Join(valuationColumns, ", :") + ")"

// valuationColumns are the columns of margin_interest_valuations but gid
var valuationColumns = fieldsNamesOf(types.MarginInterestValuation{
	BorrowedInterest: new(fixedpoint.Value),
	PriceInUSDT:      new(fixedpoint.Value),
	InterestUSDT:     new(fixedpoint.Value),
})

// valuationPriceUpdateSQL prices an existing, unpriced valuation
const valuationPriceUpdateSQL = "UPDATE margin_interest_valuations SET " +
	"price_symbol = :price_symbol, price_kline_start = :price_kline_start, " +
	"price_in_usdt = :price_in_usdt, interest_usdt = :interest_usdt, updated_at = :updated_at " +
	"WHERE interest_gid = :interest_gid"

func (s *MarginInterestValuationService) write(ctx context.Context, v types.MarginInterestValuation, exists bool) error {
	query := valuationInsertSQL
	if exists {
		query = valuationPriceUpdateSQL
	}

	_, err := s.DB.NamedExecContext(ctx, query, v)
	return err
}

// pricePair is the market an asset is priced from
type pricePair struct {
	// symbol is '' for USDT itself and for an asset without a USDT market
	symbol string

	// inverse: the market is USDT<asset>, so the price is 1 / close
	inverse bool
}

func findPricePair(asset string, markets types.MarketMap) pricePair {
	if asset == valuationQuote {
		return pricePair{}
	}

	if _, ok := markets[asset+valuationQuote]; ok {
		return pricePair{symbol: asset + valuationQuote}
	}

	if _, ok := markets[valuationQuote+asset]; ok {
		return pricePair{symbol: valuationQuote + asset, inverse: true}
	}

	return pricePair{}
}

// priceKLineStart is the start of the 1h kline whose close prices a charge at t:
// the hour before t's hour, which closes at the top of t's hour
func priceKLineStart(t time.Time) time.Time {
	return t.UTC().Truncate(time.Hour).Add(-time.Hour)
}

// isConvertedInterest reports whether Binance charged the interest in another
// asset (BNB) than the borrowed one: PERIODIC_CONVERTED or ON_BORROW_CONVERTED
func isConvertedInterest(interestType string) bool {
	return strings.HasSuffix(interestType, "_CONVERTED")
}

// valueInterest values m from the 1h kline closes of its price pair
func valueInterest(m types.MarginInterest, pair pricePair, closes map[int64]fixedpoint.Value, now time.Time) types.MarginInterestValuation {
	v := types.MarginInterestValuation{
		InterestGID:    m.GID,
		Exchange:       m.Exchange,
		Session:        m.Session,
		Time:           types.Time(m.Time.Time().UTC()),
		IsolatedSymbol: m.IsolatedSymbol,
		PaidAsset:      m.Asset,
		Converted:      isConvertedInterest(m.InterestType),
		Interest:       m.Interest,
		PriceSymbol:    pair.symbol,
		UpdatedTime:    types.Time(now.UTC()),
	}

	switch {
	case m.RawAsset != "":
		v.BorrowedAsset = m.RawAsset
	case !v.Converted:
		v.BorrowedAsset = m.Asset
	}

	switch {
	case v.BorrowedAsset == "":
		// a converted row without rawAsset: the borrowed asset is unknown
	case v.BorrowedAsset == m.Asset:
		interest := m.Interest
		v.BorrowedInterest = &interest
	default:
		// interestRate is a daily rate, and each row is one hour of interest
		estimate := m.Principal.Mul(m.InterestRate).Div(fixedpoint.NewFromInt(24))
		v.BorrowedInterest = &estimate
	}

	if m.Asset == valuationQuote {
		one, interest := fixedpoint.One, m.Interest
		v.PriceInUSDT, v.InterestUSDT = &one, &interest
		return v
	}

	if pair.symbol == "" {
		return v
	}

	start := priceKLineStart(m.Time.Time())
	closePrice, ok := closes[start.UnixMilli()]
	if !ok || closePrice.Sign() <= 0 {
		return v
	}

	price := closePrice
	if pair.inverse {
		price = fixedpoint.One.Div(closePrice)
	}

	interestUSDT := m.Interest.Mul(price)
	v.PriceKLineStart = types.Time(start)
	v.PriceInUSDT, v.InterestUSDT = &price, &interestUSDT
	return v
}

// QueryMarginInterestValuationOptions selects the valuations of one session in
// the hours [Since, Until). Both bounds must be at the top of a UTC hour.
type QueryMarginInterestValuationOptions struct {
	Exchange types.ExchangeName
	Session  string

	// Since is inclusive, at the top of a UTC hour
	Since time.Time

	// Until is exclusive, at the top of a UTC hour
	Until time.Time
}

// Query returns the valuations of a session in the hours [Since, Until), oldest first.
func (s *MarginInterestValuationService) Query(
	ctx context.Context, opts QueryMarginInterestValuationOptions,
) ([]types.MarginInterestValuation, error) {
	if !isTopOfHour(opts.Since) || !isTopOfHour(opts.Until) {
		return nil, fmt.Errorf("margin interest valuation query range [%s, %s) is not on UTC hour boundaries",
			opts.Since.UTC().Format(time.RFC3339Nano), opts.Until.UTC().Format(time.RFC3339Nano))
	}

	query, args, err := sq.Select(append([]string{"gid"}, valuationColumns...)...).
		From("margin_interest_valuations").
		Where(sq.Eq{"exchange": opts.Exchange, "session": opts.Session}).
		Where(sq.GtOrEq{"time": opts.Since.UTC()}).
		Where(sq.Lt{"time": opts.Until.UTC()}).
		OrderBy("time ASC", "interest_gid ASC").
		ToSql()
	if err != nil {
		return nil, err
	}

	var valuations []types.MarginInterestValuation
	if err := s.DB.SelectContext(ctx, &valuations, query, args...); err != nil {
		return nil, err
	}
	return valuations, nil
}
