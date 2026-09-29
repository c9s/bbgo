package bfxfunding

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/c9s/bbgo/pkg/types"
)

// insertBatchSize keeps the number of bound variables per statement (6 per row)
// under the sqlite3 default limit of 999.
const insertBatchSize = 150

// FundingTrade is a compact representation of a public funding trade used for the analysis.
// Amount is always the absolute volume of the trade and Rate is the daily rate.
type FundingTrade struct {
	ID     int64
	Time   time.Time
	Amount float64
	Rate   float64
	Period int
}

// fundingTradeRow scans the time with types.Time since sqlite3 returns DATETIME(3) columns as strings
type fundingTradeRow struct {
	ID     int64      `db:"trade_id"`
	Time   types.Time `db:"time"`
	Amount float64    `db:"amount"`
	Rate   float64    `db:"rate"`
	Period int        `db:"period"`
}

// TradeStore persists the public funding trades of Bitfinex so that the history
// only needs to be backfilled once.
type TradeStore struct {
	DB *sqlx.DB
}

func NewTradeStore(db *sqlx.DB) *TradeStore {
	return &TradeStore{DB: db}
}

// LastTrade returns the time of the latest stored trade and the IDs of the trades at that time.
// It returns a zero time when there is no trade stored for the symbol.
func (s *TradeStore) LastTrade(ctx context.Context, symbol string) (time.Time, []int64, error) {
	var lastTime types.Time
	row := s.DB.QueryRowContext(ctx,
		s.DB.Rebind("SELECT `time` FROM bfxfunding_public_trades WHERE symbol = ? ORDER BY `time` DESC LIMIT 1"),
		symbol)
	if err := row.Scan(&lastTime); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return time.Time{}, nil, nil
		}

		return time.Time{}, nil, err
	}

	last := lastTime.Time()
	var ids []int64
	err := s.DB.SelectContext(ctx, &ids,
		s.DB.Rebind("SELECT trade_id FROM bfxfunding_public_trades WHERE symbol = ? AND `time` = ?"),
		symbol, last)
	if err != nil {
		return time.Time{}, nil, err
	}

	return last, ids, nil
}

// Insert stores the trades and ignores the ones that are already stored.
func (s *TradeStore) Insert(ctx context.Context, symbol string, trades []FundingTrade) error {
	if len(trades) == 0 {
		return nil
	}

	verb := "INSERT OR IGNORE"
	if s.DB.DriverName() == "mysql" {
		verb = "INSERT IGNORE"
	}

	tx, err := s.DB.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}

	for start := 0; start < len(trades); start += insertBatchSize {
		end := min(start+insertBatchSize, len(trades))
		batch := trades[start:end]

		placeholders := make([]string, 0, len(batch))
		args := make([]any, 0, len(batch)*6)
		for _, t := range batch {
			placeholders = append(placeholders, "(?, ?, ?, ?, ?, ?)")
			args = append(args, symbol, t.ID, t.Amount, t.Rate, t.Period, t.Time.UTC())
		}

		query := verb + " INTO bfxfunding_public_trades (symbol, trade_id, amount, rate, period, `time`) VALUES " +
			strings.Join(placeholders, ", ")
		if _, err := tx.ExecContext(ctx, tx.Rebind(query), args...); err != nil {
			_ = tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}

// Query returns the trades of the symbol since the given time, ordered by time.
func (s *TradeStore) Query(ctx context.Context, symbol string, since time.Time) ([]FundingTrade, error) {
	rows, err := s.DB.QueryxContext(ctx,
		s.DB.Rebind("SELECT trade_id, amount, rate, period, `time` FROM bfxfunding_public_trades "+
			"WHERE symbol = ? AND `time` >= ? ORDER BY `time` ASC, trade_id ASC"),
		symbol, since.UTC())
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var trades []FundingTrade
	for rows.Next() {
		var r fundingTradeRow
		if err := rows.StructScan(&r); err != nil {
			return nil, err
		}

		trades = append(trades, FundingTrade{ID: r.ID, Time: r.Time.Time(), Amount: r.Amount, Rate: r.Rate, Period: r.Period})
	}

	return trades, rows.Err()
}
