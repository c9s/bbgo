package xfundingv2

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// ClosedRoundRecord is the summary of a closed arbitrage round persisted to the DB.
// The `db` tags name the xfundingv2_closed_rounds columns and are the single source
// of truth for the insert column list and values (see dbColumnsOf / dbValuesOf).
type ClosedRoundRecord struct {
	// ID is the round's own DB-independent identifier (UUID generated at creation)
	ID string `db:"id"`

	InstanceID string `db:"strategy_instance_id"`

	SpotSymbol    string `db:"spot_symbol"`
	FuturesSymbol string `db:"futures_symbol"`

	SpotExchange    string `db:"spot_exchange"`
	FuturesExchange string `db:"futures_exchange"`

	Direction       string `db:"direction"`
	CollateralAsset string `db:"collateral_asset"`

	Leverage             fixedpoint.Value `db:"leverage"`
	TriggeredFundingRate fixedpoint.Value `db:"triggered_funding_rate"`
	AnnualizedRate       fixedpoint.Value `db:"annualized_rate"`
	FundingIncome        fixedpoint.Value `db:"funding_income"`

	SpotPnL       fixedpoint.Value `db:"spot_pnl"`
	SpotNetPnL    fixedpoint.Value `db:"spot_net_pnl"`
	FuturesPnL    fixedpoint.Value `db:"futures_pnl"`
	FuturesNetPnL fixedpoint.Value `db:"futures_net_pnl"`
	NetPnL        fixedpoint.Value `db:"net_pnl"`

	NumHoldingIntervals int `db:"num_holding_intervals"`

	StartAt   time.Time  `db:"started_at"`
	ReadyAt   *time.Time `db:"ready_at"`
	ClosingAt time.Time  `db:"closing_at"`
	ClosedAt  time.Time  `db:"closed_at"`
}

// RoundInsertService persists round records into the DB
type RoundInsertService struct {
	C chan *ArbitrageRound

	db         *sqlx.DB
	ctx        context.Context
	instanceID string
	logger     logrus.FieldLogger
}

func NewRoundInsertService(ctx context.Context, db *sqlx.DB, instanceID string) *RoundInsertService {
	return &RoundInsertService{
		C: make(chan *ArbitrageRound, 100),

		db:         db,
		ctx:        ctx,
		instanceID: instanceID,
	}
}

func (s *RoundInsertService) SetLogger(logger logrus.FieldLogger) {
	s.logger = logger.WithField("component", "roundInsertService")
}

func (s *RoundInsertService) Start() {
	if s.logger == nil {
		s.logger = logrus.New()
	}
	go s.run()
}

func (s *RoundInsertService) Stop() {
	close(s.C)
}

func (s *RoundInsertService) run() {
	defer s.logger.Info("round insert service stopped")

	for {
		select {
		case <-s.ctx.Done():
			return
		case round, ok := <-s.C:
			if !ok {
				return
			}
			switch round.State() {
			case RoundClosed:
				if err := s.InsertClosedRound(round); err != nil {
					s.logger.WithError(err).Errorf("failed to insert closed round record: %s", round)
				}
			}
		}
	}
}

// newClosedRoundRecord builds the summary record and funding-fee records from a closed round
func (s *RoundInsertService) newClosedRoundRecord(round *ArbitrageRound) (ClosedRoundRecord, []FundingFee) {
	pnl := round.RealizedPnL()

	var readyTime *time.Time
	if ts := round.ReadyAt(); !ts.IsZero() {
		readyTime = &ts
	}
	record := ClosedRoundRecord{
		ID:         round.ID(),
		InstanceID: s.instanceID,

		SpotSymbol:    round.SpotSymbol(),
		FuturesSymbol: round.FuturesSymbol(),

		SpotExchange:    round.syncState.SpotExchangeName.String(),
		FuturesExchange: round.syncState.FuturesExchangeName.String(),

		Direction:       string(round.syncState.DirectionPolicy.Direction),
		CollateralAsset: round.CollateralAsset(),

		Leverage:             round.syncState.Leverage,
		TriggeredFundingRate: round.TriggeredFundingRate(),
		AnnualizedRate:       round.AnnualizedRate(),
		FundingIncome:        pnl.FundingIncome,

		SpotPnL:       pnl.SpotProfitStats.AccumulatedPnL,
		SpotNetPnL:    pnl.SpotProfitStats.AccumulatedNetProfit,
		FuturesPnL:    pnl.FuturesProfitStats.AccumulatedPnL,
		FuturesNetPnL: pnl.FuturesProfitStats.AccumulatedNetProfit,
		NetPnL:        pnl.NetPnL(),

		NumHoldingIntervals: round.NumHoldingIntervals(round.ClosedAt()),

		StartAt:   round.StartedAt(),
		ReadyAt:   readyTime,
		ClosingAt: round.ClosingAt(),
		ClosedAt:  round.ClosedAt(),
	}

	return record, newFundingFeeRecords(round)
}

// newFundingFeeRecords builds the funding-fee records of a round from its synced
// funding-fee entries. The records are linked to the round via the round's own id
// (round_id) and are shared by both the closed-round and active-round snapshot paths.
func newFundingFeeRecords(round *ArbitrageRound) []FundingFee {
	var fees []FundingFee
	for _, fee := range round.syncState.FundingFeeRecords {
		fees = append(fees, fee)
	}

	return fees
}

// InsertClosedRound persists the closed round summary and its funding-fee records in
// a single transaction. The funding-fee rows are linked to the round via the round's
// own id (round_id).
func (s *RoundInsertService) InsertClosedRound(round *ArbitrageRound) error {
	if round.State() != RoundClosed {
		return fmt.Errorf("inserted round is not closed: %s", round.State())
	}
	record, fees := s.newClosedRoundRecord(round)

	if err := s.insertClosedRound(record, fees); err != nil {
		return err
	}

	return nil
}

// insertClosedRound writes the round record and its funding-fee records in a single transaction.
// The funding-fee rows are linked to the round via the round's own id (round_id), which
// is known before insertion, so there is no need to read back the auto-incremented gid.
func (s *RoundInsertService) insertClosedRound(
	record ClosedRoundRecord, fees []FundingFee,
) error {
	tx, err := s.db.Beginx()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		// rollback is a no-op if the transaction has already been committed.
		_ = tx.Rollback()
	}()

	colNames, values := extractDBColumns(record)
	roundSQL, roundArgs, err := sq.Insert("xfundingv2_closed_rounds").
		Columns(colNames...).
		Values(values...).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to build round insert query: %w", err)
	}

	if _, err := tx.ExecContext(s.ctx, roundSQL, roundArgs...); err != nil {
		return fmt.Errorf("failed to insert arbitrage round: %w", err)
	}

	if err := upsertFundingFees(s.ctx, tx, s.db.DriverName(), fees); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit round insert transaction: %w", err)
	}

	return nil
}

// ActiveRoundRecord is a point-in-time snapshot of an active (non-closed) round
// persisted to the DB. The `db` tags name the xfundingv2_round_snapshots columns and
// are the single source of truth for the insert column list and values.
type ActiveRoundRecord struct {
	ID string `db:"id"`

	InstanceID string `db:"strategy_instance_id"`

	SpotSymbol    string `db:"spot_symbol"`
	FuturesSymbol string `db:"futures_symbol"`

	SpotExchange    string `db:"spot_exchange"`
	FuturesExchange string `db:"futures_exchange"`

	Direction       string `db:"direction"`
	CollateralAsset string `db:"collateral_asset"`

	Leverage             fixedpoint.Value `db:"leverage"`
	TriggeredFundingRate fixedpoint.Value `db:"triggered_funding_rate"`
	AnnualizedRate       fixedpoint.Value `db:"annualized_rate"`
	FundingIncome        fixedpoint.Value `db:"funding_income"`

	SpotPosition    fixedpoint.Value `db:"spot_position"`
	FuturesPosition fixedpoint.Value `db:"futures_position"`
	State           string           `db:"state"`

	// prices used to calculate the unrealized PnL
	SpotPrice    fixedpoint.Value `db:"spot_price"`
	FuturesPrice fixedpoint.Value `db:"futures_price"`

	// Average Cost
	SpotAverageCost    fixedpoint.Value `db:"spot_average_cost"`
	FuturesAverageCost fixedpoint.Value `db:"futures_average_cost"`

	// realized PnLs
	SpotPnL       fixedpoint.Value `db:"spot_pnl"`
	SpotNetPnL    fixedpoint.Value `db:"spot_net_pnl"`
	FuturesPnL    fixedpoint.Value `db:"futures_pnl"`
	FuturesNetPnL fixedpoint.Value `db:"futures_net_pnl"`
	NetPnL        fixedpoint.Value `db:"net_pnl"`

	// unrealized PnLs
	UnrealizedSpotPnL    fixedpoint.Value `db:"unrealized_spot_pnl"`
	UnrealizedFuturesPnL fixedpoint.Value `db:"unrealized_futures_pnl"`

	TotalNetPnL        fixedpoint.Value `db:"total_net_pnl"`
	TotalSpotNetPnL    fixedpoint.Value `db:"total_spot_net_pnl"`
	TotalFuturesNetPnL fixedpoint.Value `db:"total_futures_net_pnl"`

	StartedAt time.Time `db:"started_at"`
}

// newActiveRoundRecord builds a point-in-time snapshot of an active (non-closed)
// round, marking its open spot and futures legs to the current order books to
// derive the unrealized PnL fields. It also returns the round's funding-fee records
// so the snapshot path persists the same funding-fee rows as the closed-round path.
func (s *RoundInsertService) newActiveRoundRecord(
	round *ArbitrageRound,
	spotOrderBook, futuresOrderBook types.OrderBook,
) (ActiveRoundRecord, []FundingFee) {
	pnl := round.UnrealizedPnL(spotOrderBook, futuresOrderBook)

	record := ActiveRoundRecord{
		ID:         round.ID(),
		InstanceID: s.instanceID,

		SpotSymbol:    round.SpotSymbol(),
		FuturesSymbol: round.FuturesSymbol(),

		SpotExchange:    round.syncState.SpotExchangeName.String(),
		FuturesExchange: round.syncState.FuturesExchangeName.String(),

		Direction:       string(round.syncState.DirectionPolicy.Direction),
		CollateralAsset: round.CollateralAsset(),

		Leverage:             round.syncState.Leverage,
		TriggeredFundingRate: round.TriggeredFundingRate(),
		AnnualizedRate:       round.AnnualizedRate(),
		FundingIncome:        pnl.FundingIncome,

		SpotPosition:    pnl.SpotPosition.Base,
		FuturesPosition: pnl.FuturesPosition.Base,
		State:           round.State().String(),

		SpotPrice:          pnl.SpotPrice,
		FuturesPrice:       pnl.FuturesPrice,
		SpotAverageCost:    pnl.SpotPosition.AverageCost,
		FuturesAverageCost: pnl.FuturesPosition.AverageCost,

		SpotPnL:       pnl.SpotProfitStats.AccumulatedPnL,
		SpotNetPnL:    pnl.SpotProfitStats.AccumulatedNetProfit,
		FuturesPnL:    pnl.FuturesProfitStats.AccumulatedPnL,
		FuturesNetPnL: pnl.FuturesProfitStats.AccumulatedNetProfit,
		NetPnL:        pnl.NetPnL(),

		UnrealizedSpotPnL:    pnl.UnrealizedSpotPnL,
		UnrealizedFuturesPnL: pnl.UnrealizedFuturesPnL,

		TotalNetPnL:        pnl.TotalPnL(),
		TotalSpotNetPnL:    pnl.TotalSpotNetPnL(),
		TotalFuturesNetPnL: pnl.TotalFuturesNetPnL(),

		StartedAt: round.StartedAt(),
	}

	return record, newFundingFeeRecords(round)
}

// InsertActiveRound persists a point-in-time snapshot of an active (non-closed)
// round into the xfundingv2_round_snapshots table together with the round's
// funding-fee records. The snapshot table keeps a single row per round (unique on
// id), so re-snapshotting the same round upserts the existing row rather than
// appending a new one. Funding-fee rows are likewise upserted (keyed by
// round_id, txn) so they can be re-persisted when the round is later closed.
func (s *RoundInsertService) InsertActiveRound(
	round *ArbitrageRound,
	spotOrderBook, futuresOrderBook types.OrderBook,
) error {
	if round.State() == RoundClosed {
		return fmt.Errorf("given round is not active but closed: %s", round)
	}

	record, fees := s.newActiveRoundRecord(round, spotOrderBook, futuresOrderBook)

	return s.insertActiveRound(record, fees)
}

// insertActiveRound upserts the active round snapshot row and its funding-fee rows
// in a single transaction. The snapshot is keyed by the round's own id (unique in
// the snapshots table), so an existing snapshot is updated in place, keeping exactly
// one live snapshot per active round. Funding fees are upserted on (round_id, txn).
func (s *RoundInsertService) insertActiveRound(
	record ActiveRoundRecord, fees []FundingFee,
) error {
	tx, err := s.db.Beginx()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		// rollback is a no-op if the transaction has already been committed.
		_ = tx.Rollback()
	}()

	colNames, values := extractDBColumns(record)
	snapshotSQL, snapshotArgs, err := sq.Insert("xfundingv2_round_snapshots").
		Columns(colNames...).
		Values(values...).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to build active round snapshot query: %w", err)
	}

	if _, err := tx.ExecContext(s.ctx, snapshotSQL, snapshotArgs...); err != nil {
		return fmt.Errorf("failed to upsert active round snapshot: %w", err)
	}

	if err := upsertFundingFees(s.ctx, tx, s.db.DriverName(), fees); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit active round snapshot transaction: %w", err)
	}

	return nil
}

// upsertFundingFees upserts the given funding-fee rows into xfundingv2_funding_fees
// within the provided transaction. Rows are keyed by (round_id, txn); an existing
// row is updated in place, so the same fees can be persisted both while the round is
// an active snapshot and again when it is closed. It is a no-op when fees is empty.
func upsertFundingFees(
	ctx context.Context, tx *sqlx.Tx, driverName string, fees []FundingFee,
) error {
	if len(fees) == 0 {
		return nil
	}

	colNames, _ := extractDBColumns(FundingFee{})
	feeBuilder := sq.Insert("xfundingv2_funding_fees").Columns(colNames...)
	for _, fee := range fees {
		_, values := extractDBColumns(fee)
		feeBuilder = feeBuilder.Values(values...)
	}

	suffix := upsertSuffix(driverName, colNames, "round_id", "txn")
	feeSQL, feeArgs, err := feeBuilder.Suffix(suffix).ToSql()
	if err != nil {
		return fmt.Errorf("failed to build funding fee upsert query: %w", err)
	}

	if _, err := tx.ExecContext(ctx, feeSQL, feeArgs...); err != nil {
		return fmt.Errorf("failed to upsert round funding fees: %w", err)
	}

	return nil
}

// upsertSuffix builds a dialect-specific upsert clause that updates every non-key
// column when a row with the same conflict key already exists. MySQL uses
// ON DUPLICATE KEY UPDATE (relying on the matching unique index); sqlite3 and other
// ON CONFLICT dialects use ON CONFLICT(<keys>) DO UPDATE SET.
func upsertSuffix(driverName string, columns []string, conflictKeys ...string) string {
	isConflictKey := make(map[string]struct{}, len(conflictKeys))
	for _, key := range conflictKeys {
		isConflictKey[key] = struct{}{}
	}

	mysql := driverName == "mysql"
	assignments := make([]string, 0, len(columns))
	for _, col := range columns {
		if _, found := isConflictKey[col]; found {
			continue
		}
		if mysql {
			assignments = append(assignments, fmt.Sprintf("%s=VALUES(%s)", col, col))
		} else {
			assignments = append(assignments, fmt.Sprintf("%s=excluded.%s", col, col))
		}
	}

	if mysql {
		return "ON DUPLICATE KEY UPDATE " + strings.Join(assignments, ", ")
	}

	// sqlite3 (and other ON CONFLICT dialects)
	return "ON CONFLICT(" + strings.Join(conflictKeys, ", ") + ") DO UPDATE SET " +
		strings.Join(assignments, ", ")
}

// extractDBColumns returns the list of column names and values of a struct record based on its `db` tags.
func extractDBColumns(record any) ([]string, []any) {
	rt := reflect.TypeOf(record)
	for rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}
	rv := reflect.ValueOf(record)
	for rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}

	colnames := make([]string, 0, rt.NumField())
	values := make([]any, 0, rt.NumField())
	for i := 0; i < rt.NumField(); i++ {
		if col, ok := dbColumnName(rt.Field(i)); ok {
			value := rv.Field(i).Interface()
			colnames = append(colnames, col)
			values = append(values, value)
		}
	}
	return colnames, values
}

// dbColumnName returns the db column name of a struct field and whether it should be
// persisted. Unexported fields, fields with no `db` tag, and fields tagged "gid",
// "-", or "" are skipped. Tag options (e.g. "name,opt") are ignored beyond the name.
func dbColumnName(field reflect.StructField) (string, bool) {
	if !field.IsExported() {
		return "", false
	}

	tag, ok := field.Tag.Lookup("db")
	if !ok {
		return "", false
	}

	name := tag
	if idx := strings.IndexByte(tag, ','); idx >= 0 {
		name = tag[:idx]
	}

	switch name {
	case "", "-", "gid":
		return "", false
	}
	return name, true
}
