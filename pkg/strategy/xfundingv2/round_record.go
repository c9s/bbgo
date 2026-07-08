package xfundingv2

import (
	"context"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

// ClosedRoundRecord is the summary of a closed arbitrage round persisted to the DB
type ClosedRoundRecord struct {
	// ID is the round's own DB-independent identifier (UUID generated at creation)
	ID string

	InstanceID string

	SpotSymbol    string
	FuturesSymbol string

	SpotExchange    string
	FuturesExchange string

	Direction       string
	CollateralAsset string

	Leverage             fixedpoint.Value
	TriggeredFundingRate fixedpoint.Value
	AnnualizedRate       fixedpoint.Value
	FundingIncome        fixedpoint.Value

	SpotPnL       fixedpoint.Value
	SpotNetPnL    fixedpoint.Value
	FuturesPnL    fixedpoint.Value
	FuturesNetPnL fixedpoint.Value
	NetPnL        fixedpoint.Value

	NumHoldingIntervals int

	StartTime   time.Time
	ReadyTime   time.Time
	ClosingTime time.Time
	ClosedTime  time.Time
}

// FundingFeeRecord is an individual funding-fee entry of a round persisted to the DB
type FundingFeeRecord struct {
	RoundID string
	Asset   string
	Amount  fixedpoint.Value
	Txn     int64
	Time    time.Time
}

// RoundInsertService persists round records into the DB
type RoundInsertService struct {
	db *sqlx.DB

	ctx        context.Context
	instanceID string
}

func NewRoundInsertService(ctx context.Context, db *sqlx.DB, instanceID string) *RoundInsertService {
	return &RoundInsertService{
		ctx:        ctx,
		db:         db,
		instanceID: instanceID,
	}
}

// newClosedRoundRecord builds the summary record and funding-fee records from a closed round
func (s *RoundInsertService) newClosedRoundRecord(round *ArbitrageRound) (ClosedRoundRecord, []FundingFeeRecord) {
	pnl := round.RealizedPnL()

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

		NumHoldingIntervals: round.NumHoldingIntervals(round.LastUpdateTime()),

		StartTime:   round.StartTime(),
		ReadyTime:   round.ReadyTime(),
		ClosingTime: round.ClosingTime(),
		ClosedTime:  round.ClosedTime(),
	}

	var fees []FundingFeeRecord
	for _, fee := range round.syncState.FundingFeeRecords {
		fees = append(fees, FundingFeeRecord{
			RoundID: round.ID(),
			Asset:   fee.Asset,
			Amount:  fee.Amount,
			Txn:     fee.Txn,
			Time:    fee.Time,
		})
	}

	return record, fees
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
	record ClosedRoundRecord, fees []FundingFeeRecord,
) error {
	tx, err := s.db.Beginx()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		// rollback is a no-op if the transaction has already been committed.
		_ = tx.Rollback()
	}()

	roundSQL, roundArgs, err := sq.Insert("xfundingv2_closed_rounds").
		Columns(
			"id",
			"strategy_instance_id",
			"spot_symbol",
			"futures_symbol",
			"spot_exchange",
			"futures_exchange",
			"direction",
			"collateral_asset",
			"leverage",
			"triggered_funding_rate",
			"annualized_rate",
			"funding_income",
			"spot_pnl",
			"spot_net_pnl",
			"futures_pnl",
			"futures_net_pnl",
			"net_pnl",
			"num_holding_intervals",
			"start_time",
			"ready_time",
			"closing_time",
			"closed_time",
		).
		Values(
			record.ID,
			record.InstanceID,
			record.SpotSymbol,
			record.FuturesSymbol,
			record.SpotExchange,
			record.FuturesExchange,
			record.Direction,
			record.CollateralAsset,
			record.Leverage,
			record.TriggeredFundingRate,
			record.AnnualizedRate,
			record.FundingIncome,
			record.SpotPnL,
			record.SpotNetPnL,
			record.FuturesPnL,
			record.FuturesNetPnL,
			record.NetPnL,
			record.NumHoldingIntervals,
			record.StartTime,
			record.ReadyTime,
			record.ClosingTime,
			record.ClosedTime,
		).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to build round insert query: %w", err)
	}

	if _, err := tx.ExecContext(s.ctx, roundSQL, roundArgs...); err != nil {
		return fmt.Errorf("failed to insert arbitrage round: %w", err)
	}

	if len(fees) > 0 {
		feeBuilder := sq.Insert("xfundingv2_funding_fees").
			Columns("round_id", "asset", "amount", "txn", "time")
		for _, fee := range fees {
			feeBuilder = feeBuilder.Values(fee.RoundID, fee.Asset, fee.Amount, fee.Txn, fee.Time)
		}

		feeSQL, feeArgs, err := feeBuilder.ToSql()
		if err != nil {
			return fmt.Errorf("failed to build funding fee insert query: %w", err)
		}

		if _, err := tx.ExecContext(s.ctx, feeSQL, feeArgs...); err != nil {
			return fmt.Errorf("failed to insert round funding fees: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit round insert transaction: %w", err)
	}

	return nil
}
