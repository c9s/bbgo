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
// unpriced. Both the asset the interest was paid in (BNB when Binance converts
// it) and the borrowed asset are priced, at the same kline. PriceStatus tells
// which values are set; only the rows missing a kline are valued again.
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

// Sync syncs the 1h klines the pending rows of session since since need, then
// values every row that has no valuation yet or is missing a kline. A row whose
// kline cannot be synced is stored as kline_missing and valued again by the next Sync.
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

	pricer := interestPricer{
		pairs:  make(map[string]pricePair),
		closes: make(map[string]map[int64]fixedpoint.Value),
	}

	// oldest is the oldest kline start each price pair needs
	oldest := make(map[string]time.Time)
	need := func(asset string, start time.Time) {
		if asset == "" || asset == valuationQuote {
			return
		}

		pair, ok := pricer.pairs[asset]
		if !ok {
			pair = findPricePair(asset, markets)
			pricer.pairs[asset] = pair
		}

		if pair.symbol == "" {
			return
		}

		if t, ok := oldest[pair.symbol]; !ok || start.Before(t) {
			oldest[pair.symbol] = start
		}
	}

	for _, p := range pending {
		start := priceKLineStart(p.Time.Time())
		need(p.Asset, start)
		need(borrowedAssetOf(p.MarginInterest), start)
	}

	klines := &BacktestService{DB: s.DB}
	spot := spotKLineExchange{ex}
	now := time.Now()

	var errs []error
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
		pricer.closes[symbol] = c
	}

	for _, p := range pending {
		v := valueInterest(p.MarginInterest, pricer, now)
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

// pendingInterest is a margin_interests row without a valuation, or with one missing a kline
type pendingInterest struct {
	types.MarginInterest

	// ValuationGID is set when the row has a valuation, missing a kline
	ValuationGID sql.NullInt64 `db:"valuation_gid"`
}

// selectPending selects the rows of session since since with no valuation or one missing a kline
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
		Where(sq.Or{sq.Eq{"v.gid": nil}, sq.Eq{"v.price_status": string(types.MarginInterestKLineMissing)}}).
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
	BorrowedInterest:     new(fixedpoint.Value),
	PriceInUSDT:          new(fixedpoint.Value),
	InterestUSDT:         new(fixedpoint.Value),
	BorrowedPriceInUSDT:  new(fixedpoint.Value),
	BorrowedInterestUSDT: new(fixedpoint.Value),
	PrincipalUSDT:        new(fixedpoint.Value),
})

// valuationUpdateSQL values an existing valuation again, every column but the keys
var valuationUpdateSQL = func() string {
	sets := make([]string, 0, len(valuationColumns))
	for _, c := range valuationColumns {
		if c != "interest_gid" {
			sets = append(sets, c+" = :"+c)
		}
	}
	return "UPDATE margin_interest_valuations SET " + strings.Join(sets, ", ") + " WHERE interest_gid = :interest_gid"
}()

func (s *MarginInterestValuationService) write(ctx context.Context, v types.MarginInterestValuation, exists bool) error {
	query := valuationInsertSQL
	if exists {
		query = valuationUpdateSQL
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

// borrowedAssetOf is the asset m was borrowed in: rawAsset, else the paid asset
// when the interest was not converted, else "" (unknown)
func borrowedAssetOf(m types.MarginInterest) string {
	switch {
	case m.RawAsset != "":
		return m.RawAsset
	case !isConvertedInterest(m.InterestType):
		return m.Asset
	}
	return ""
}

// interestPricer prices assets from the 1h kline closes a Sync has loaded
type interestPricer struct {
	// pairs is the price pair of each asset; an asset not in it has no market
	pairs map[string]pricePair

	// closes is the close of each stored 1h kline, by symbol, then start time
	closes map[string]map[int64]fixedpoint.Value
}

// assetPrice is the USDT price of an asset at a charge
type assetPrice struct {
	symbol string

	// price is nil when the asset is not priced
	price  *fixedpoint.Value
	status types.MarginInterestPriceStatus

	// fromKLine: price is the close of a kline, not the 1 of USDT
	fromKLine bool
}

// priceAt prices asset from the close of the 1h kline starting at start
func (p interestPricer) priceAt(asset string, start time.Time) assetPrice {
	if asset == valuationQuote {
		one := fixedpoint.One
		return assetPrice{price: &one, status: types.MarginInterestPriced}
	}

	pair := p.pairs[asset]
	if pair.symbol == "" {
		return assetPrice{status: types.MarginInterestNoMarket}
	}

	closePrice, ok := p.closes[pair.symbol][start.UnixMilli()]
	if !ok || closePrice.Sign() <= 0 {
		return assetPrice{symbol: pair.symbol, status: types.MarginInterestKLineMissing}
	}

	price := closePrice
	if pair.inverse {
		price = fixedpoint.One.Div(closePrice)
	}

	return assetPrice{symbol: pair.symbol, price: &price, status: types.MarginInterestPriced, fromKLine: true}
}

// priceStatusRank orders the statuses: when several apply, the lowest wins
var priceStatusRank = map[types.MarginInterestPriceStatus]int{
	types.MarginInterestKLineMissing:    0,
	types.MarginInterestNoMarket:        1,
	types.MarginInterestUnknownBorrowed: 2,
	types.MarginInterestPriced:          3,
}

func firstPriceStatus(a, b types.MarginInterestPriceStatus) types.MarginInterestPriceStatus {
	if priceStatusRank[b] < priceStatusRank[a] {
		return b
	}
	return a
}

// valueInterest values m in its paid and its borrowed asset
func valueInterest(m types.MarginInterest, pricer interestPricer, now time.Time) types.MarginInterestValuation {
	v := types.MarginInterestValuation{
		InterestGID:    m.GID,
		Exchange:       m.Exchange,
		Session:        m.Session,
		Time:           types.Time(m.Time.Time().UTC()),
		IsolatedSymbol: m.IsolatedSymbol,
		PaidAsset:      m.Asset,
		BorrowedAsset:  borrowedAssetOf(m),
		InterestType:   m.InterestType,
		Converted:      isConvertedInterest(m.InterestType),
		Principal:      m.Principal,
		InterestRate:   m.InterestRate,
		Interest:       m.Interest,
		UpdatedTime:    types.Time(now.UTC()),
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

	start := priceKLineStart(m.Time.Time())
	paid := pricer.priceAt(m.Asset, start)
	v.PriceSymbol, v.PriceStatus = paid.symbol, paid.status
	if paid.price != nil {
		interestUSDT := m.Interest.Mul(*paid.price)
		v.PriceInUSDT, v.InterestUSDT = paid.price, &interestUSDT
	}

	borrowed := paid
	switch v.BorrowedAsset {
	case "":
		v.PriceStatus = firstPriceStatus(v.PriceStatus, types.MarginInterestUnknownBorrowed)
		borrowed = assetPrice{}
	case m.Asset:
	default:
		borrowed = pricer.priceAt(v.BorrowedAsset, start)
		v.PriceStatus = firstPriceStatus(v.PriceStatus, borrowed.status)
	}

	v.BorrowedPriceSymbol = borrowed.symbol
	if borrowed.price != nil {
		borrowedPrice := *borrowed.price
		interestUSDT := v.BorrowedInterest.Mul(borrowedPrice)
		principalUSDT := m.Principal.Mul(borrowedPrice)
		v.BorrowedPriceInUSDT = &borrowedPrice
		v.BorrowedInterestUSDT, v.PrincipalUSDT = &interestUSDT, &principalUSDT
	}

	if paid.fromKLine || borrowed.fromKLine {
		v.PriceKLineStart = types.Time(start)
	}

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
