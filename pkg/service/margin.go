package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	sq "github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"

	"github.com/c9s/bbgo/pkg/exchange/batch"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

// MaxSessionNameLength is the size of the session column of margin_interests,
// VARCHAR(64), which counts characters, not bytes
const MaxSessionNameLength = 64

// ValidateMarginSessionName returns an error if session does not fit the session column
func ValidateMarginSessionName(session string) error {
	if utf8.RuneCountInString(session) > MaxSessionNameLength {
		return fmt.Errorf("session name %q is longer than %d characters", session, MaxSessionNameLength)
	}
	return nil
}

// marginHistoryAPI returns the margin history service of ex, or nil if ex is not a margin exchange
func marginHistoryAPI(ex types.Exchange) types.MarginHistoryService {
	api, ok := ex.(types.MarginHistoryService)
	if !ok {
		return nil
	}

	marginExchange, ok := ex.(types.MarginExchange)
	if !ok {
		return nil
	}

	if !marginExchange.GetMarginSettings().IsMargin {
		return nil
	}

	return api
}

// MarginService syncs and queries the margin history (loans, repays, interests, liquidations)
type MarginService struct {
	DB *sqlx.DB
}

// Sync syncs the loans, repays, interests and liquidations of asset, starting
// from the last synced record, or from startTime. Interests are synced per
// session: they resume from the last record of session and are stored under it.
//
// A session name that does not fit the session column is skipped with a
// warning, so it does not stop the startup sync of the other sessions.
func (s *MarginService) Sync(ctx context.Context, ex types.Exchange, session, asset string, startTime time.Time) error {
	if err := ValidateMarginSessionName(session); err != nil {
		util.GetLoggerFromCtx(ctx).WithError(err).Warnf("margin history of session %q is not synced", session)
		return nil
	}

	api := marginHistoryAPI(ex)
	if api == nil {
		return nil
	}

	tasks := []SyncTask{
		{
			Select: SelectLastMarginLoans(ex.Name(), asset, 100),
			Type:   types.MarginLoan{},
			BatchQuery: func(ctx context.Context, startTime, endTime time.Time) (interface{}, chan error) {
				query := &batch.MarginLoanBatchQuery{
					MarginHistoryService: api,
				}
				return query.Query(ctx, asset, startTime, endTime)
			},
			Time: func(obj interface{}) time.Time {
				return obj.(types.MarginLoan).Time.Time()
			},
			ID: func(obj interface{}) string {
				return strconv.FormatUint(obj.(types.MarginLoan).TransactionID, 10)
			},
			LogInsert: true,
		},
		{
			Select: SelectLastMarginRepays(ex.Name(), asset, 100),
			Type:   types.MarginRepay{},
			BatchQuery: func(ctx context.Context, startTime, endTime time.Time) (interface{}, chan error) {
				query := &batch.MarginRepayBatchQuery{
					MarginHistoryService: api,
				}
				return query.Query(ctx, asset, startTime, endTime)
			},
			Time: func(obj interface{}) time.Time {
				return obj.(types.MarginRepay).Time.Time()
			},
			ID: func(obj interface{}) string {
				return strconv.FormatUint(obj.(types.MarginRepay).TransactionID, 10)
			},
			LogInsert: true,
		},
		s.interestSyncTask(api, ex.Name(), session, asset),
		{
			Select: SelectLastMarginLiquidations(ex.Name(), 100),
			Type:   types.MarginLiquidation{},
			BatchQuery: func(ctx context.Context, startTime, endTime time.Time) (interface{}, chan error) {
				query := &batch.MarginLiquidationBatchQuery{
					MarginHistoryService: api,
				}
				return query.Query(ctx, startTime, endTime)
			},
			Time: func(obj interface{}) time.Time {
				return obj.(types.MarginLiquidation).UpdatedTime.Time()
			},
			ID: func(obj interface{}) string {
				m := obj.(types.MarginLiquidation)
				return strconv.FormatUint(m.OrderID, 10)
			},
			LogInsert: true,
		},
	}

	for _, sel := range tasks {
		if err := sel.execute(ctx, s.DB, startTime); err != nil {
			return err
		}
	}

	return nil
}

// SyncInterests syncs only the interests of asset for session. Unlike Sync, it
// starts from max(last synced record, startTime), so a sparse or old asset is not
// scanned again from months back on every call.
func (s *MarginService) SyncInterests(ctx context.Context, ex types.Exchange, session, asset string, startTime time.Time) error {
	if err := ValidateMarginSessionName(session); err != nil {
		return err
	}

	api := marginHistoryAPI(ex)
	if api == nil {
		return nil
	}

	task := s.interestSyncTask(api, ex.Name(), session, asset)
	task.ClampStartTime = true
	return task.execute(ctx, s.DB, startTime)
}

// interestResumeOverlap is how far before the newest stored row an interest sync starts
const interestResumeOverlap = time.Hour

// interestSelectLimit is the number of newest rows loaded to resume and dedup the interest sync
const interestSelectLimit = 100

// marginInterestInsertSQL inserts every column of margin_interests. insertType
// leaves out a nil pointer field and caches its SQL per type, so a first row
// without a TxID would leave tx_id out of every later insert.
var marginInterestInsertSQL = "INSERT INTO margin_interests (" + strings.Join(marginInterestColumns, ", ") +
	") VALUES (:" + strings.Join(marginInterestColumns, ", :") + ")"

// marginInterestColumns are the columns of margin_interests but gid
var marginInterestColumns = fieldsNamesOf(types.MarginInterest{TxID: new(uint64)})

// insertMarginInterest inserts m, with tx_id NULL when m.TxID is nil
func insertMarginInterest(db *sqlx.DB, m types.MarginInterest) error {
	_, err := db.NamedExec(marginInterestInsertSQL, m)
	return err
}

// interestSyncTask builds the interest sync task. Each row is stored as the
// exchange returns it, stamped with session and normalized to UTC.
func (s *MarginService) interestSyncTask(api types.MarginHistoryService, ex types.ExchangeName, session, asset string) SyncTask {
	return SyncTask{
		Select:      SelectLastMarginInterests(ex, session, asset, interestSelectLimit),
		SelectLimit: interestSelectLimit,
		Type:        types.MarginInterest{},
		BatchQuery: func(ctx context.Context, startTime, endTime time.Time) (interface{}, chan error) {
			query := &batch.MarginInterestBatchQuery{
				MarginHistoryService: api,
			}
			return query.Query(ctx, asset, startTime, endTime)
		},
		Time: func(obj interface{}) time.Time {
			return obj.(types.MarginInterest).Time.Time()
		},
		ID: func(obj interface{}) string {
			return obj.(types.MarginInterest).Key()
		},
		Insert: func(obj interface{}) error {
			m := obj.(types.MarginInterest)
			m.Session = session
			m.Time = types.Time(m.Time.Time().UTC())
			return insertMarginInterest(s.DB, m)
		},
		// re-fetch the hour before the newest row: a borrow-time row stamped mid-hour
		// may be synced before the hourly charge of the same hour is published.
		// SelectLimit keeps the overlap within the loaded rows, so no row is stored twice.
		ResumeOverlap: interestResumeOverlap,
		LogInsert:     true,
	}
}

// QueryMarginInterestOptions selects the interest rows of one session in the
// hours [Since, Until). Both bounds must be at the top of a UTC hour, e.g. a
// margininterest.HourRange.
type QueryMarginInterestOptions struct {
	Exchange types.ExchangeName
	Session  string

	// Assets filters the assets, empty means all
	Assets []string

	// Since is inclusive, at the top of a UTC hour
	Since time.Time

	// Until is exclusive, at the top of a UTC hour
	Until time.Time
}

func isTopOfHour(t time.Time) bool {
	return t.Equal(t.Truncate(time.Hour))
}

// QueryInterests returns the interest rows of a session in the hours [Since, Until), oldest first.
func (s *MarginService) QueryInterests(ctx context.Context, opts QueryMarginInterestOptions) ([]types.MarginInterest, error) {
	if !isTopOfHour(opts.Since) || !isTopOfHour(opts.Until) {
		return nil, fmt.Errorf("margin interest query range [%s, %s) is not on UTC hour boundaries",
			opts.Since.UTC().Format(time.RFC3339Nano), opts.Until.UTC().Format(time.RFC3339Nano))
	}

	builder := sq.Select(marginInterestColumns...).
		From("margin_interests").
		Where(sq.Eq{"exchange": opts.Exchange, "session": opts.Session}).
		Where(sq.GtOrEq{"time": opts.Since.UTC()}).
		Where(sq.Lt{"time": opts.Until.UTC()}).
		OrderBy("time ASC")

	if len(opts.Assets) > 0 {
		builder = builder.Where(sq.Eq{"asset": opts.Assets})
	}

	sql, args, err := builder.ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := s.DB.QueryxContext(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var interests []types.MarginInterest
	for rows.Next() {
		var m types.MarginInterest
		if err := rows.StructScan(&m); err != nil {
			return nil, err
		}
		interests = append(interests, m)
	}

	return interests, rows.Err()
}

func SelectLastMarginLoans(ex types.ExchangeName, asset string, limit uint64) sq.SelectBuilder {
	return sq.Select("*").
		From("margin_loans").
		Where(sq.Eq{"exchange": ex, "asset": asset}).
		OrderBy("time DESC").
		Limit(limit)
}

func SelectLastMarginRepays(ex types.ExchangeName, asset string, limit uint64) sq.SelectBuilder {
	return sq.Select("*").
		From("margin_repays").
		Where(sq.Eq{"exchange": ex, "asset": asset}).
		OrderBy("time DESC").
		Limit(limit)
}

// SelectLastMarginInterests selects the newest margin_interests rows of a session
func SelectLastMarginInterests(ex types.ExchangeName, session, asset string, limit uint64) sq.SelectBuilder {
	return sq.Select("*").
		From("margin_interests").
		Where(sq.Eq{"exchange": ex, "session": session, "asset": asset}).
		OrderBy("time DESC").
		Limit(limit)
}

func SelectLastMarginLiquidations(ex types.ExchangeName, limit uint64) sq.SelectBuilder {
	return sq.Select("*").
		From("margin_liquidations").
		Where(sq.Eq{"exchange": ex}).
		OrderBy("time DESC").
		Limit(limit)
}
