package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/c9s/bbgo/pkg/exchange/batch"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/jmoiron/sqlx"
)

type ExchangeFuturesService interface {
	types.ExchangeRiskService
	types.ExchangeFundingFeeService
}

type FuturesService struct {
	DB                         *sqlx.DB
	PositionRiskUpdateInterval time.Duration

	positionRiskLastUpdateTime map[string]time.Time
}

func NewFuturesService(db *sqlx.DB) *FuturesService {
	return &FuturesService{
		DB:                         db,
		positionRiskLastUpdateTime: make(map[string]time.Time),
	}
}

func (s *FuturesService) QueryPositionsAndInsert(
	ctx context.Context, exchange ExchangeFuturesService, currentTime time.Time, symbol ...string) error {
	symbolStr := "*"
	if len(symbol) > 0 {
		// sort to ensure the symbolStr is the same for the same set of symbols
		sort.Slice(symbol, func(i, j int) bool {
			return symbol[i] < symbol[j]
		})
		symbolStr = strings.Join(symbol, ",")
	}
	var lastUpdateTime time.Time
	if updateTime, ok := s.positionRiskLastUpdateTime[symbolStr]; ok {
		lastUpdateTime = updateTime
	}

	if !lastUpdateTime.IsZero() {
		if currentTime.Before(lastUpdateTime) {
			return nil
		}

		if s.PositionRiskUpdateInterval != 0 && currentTime.Sub(lastUpdateTime) < s.PositionRiskUpdateInterval {
			return nil
		}
	}
	s.positionRiskLastUpdateTime[symbolStr] = currentTime

	risks, err := exchange.QueryPositionRisk(ctx, symbol...)
	if err != nil {
		return fmt.Errorf("failed to query %s position risk: %w", symbol, err)
	}

	for _, risk := range risks {
		risk.UpdateTime = types.MillisecondTimestamp(time.Now())
		if err := s.InsertPositionRisk(risk); err != nil {
			return fmt.Errorf("failed to insert position risk (%+v): %w", risk, err)
		}
	}

	return nil
}

type QueryFuturesPositionRiskOptions struct {
	Exchange string
	Symbol   string
}

func (s *FuturesService) Sync(
	ctx context.Context, service ExchangeFuturesService, symbol string, startTime, endTime time.Time,
) error {
	// TODO: sync the position history of the given time range
	// we only sync the lastest position risk record for now.
	// Binance does not provide the position risk history API for the time being.
	risks, err := service.QueryPositionRisk(ctx, symbol)
	if err != nil {
		return fmt.Errorf("failed to query position risk: %w", err)
	}
	if len(risks) > 0 {
		risk := risks[0]
		risk.UpdateTime = types.MillisecondTimestamp(time.Now())
		if err := s.InsertPositionRisk(risk); err != nil {
			return fmt.Errorf("failed to insert position risk (%+v): %w", risk, err)
		}
	}

	// sync the funding fee history
	query := batch.FuturesFundingFeeBatchQuery{
		ExchangeFundingFeeService: service,
	}
	feeC, errC := query.Query(ctx, symbol, startTime, endTime)
	keepRunning := true
	for keepRunning {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case fee, ok := <-feeC:
			if !ok {
				keepRunning = false
				continue
			}

			if err := s.InsertFundingFee(fee); err != nil {
				return fmt.Errorf("failed to insert funding fee (%+v): %w", fee, err)
			}
		case err, ok := <-errC:
			if !ok {
				keepRunning = false
				continue
			}
			if err != nil {
				return fmt.Errorf("failed to query funding fee: %w", err)
			}
		}
	}
	return nil
}

func (s *FuturesService) QueryPositionRisks(options QueryFuturesPositionRiskOptions) ([]types.PositionRisk, error) {
	columns := fieldsNamesOf(types.PositionRisk{})
	builder := sq.
		Select(columns...).
		From("futures_position_risks").
		Where(sq.Eq{"exchange": options.Exchange, "symbol": options.Symbol}).
		OrderBy("updated_at DESC")
	sql, args, err := builder.ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := s.DB.Queryx(sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var risks []types.PositionRisk
	for rows.Next() {
		var risk types.PositionRisk
		if err := rows.StructScan(&risk); err != nil {
			return nil, err
		}

		risks = append(risks, risk)
	}

	return risks, nil
}

func (s *FuturesService) InsertPositionRisk(risk types.PositionRisk) (err error) {
	sql := `
	INSERT INTO futures_position_risks (
		exchange, symbol, position_side, entry_price, leverage, liquidation_price,
		mark_price, break_even_price, unrealized_pnl, notional, initial_margin, maint_margin,
		position_initial_margin, open_order_initial_margin, adl, margin_asset,
		position_amount, updated_at
	) VALUES (
		:exchange, :symbol, :position_side, :entry_price, :leverage, :liquidation_price,
		:mark_price, :break_even_price, :unrealized_pnl, :notional, :initial_margin, :maint_margin,
		:position_initial_margin, :open_order_initial_margin, :adl, :margin_asset,
		:position_amount, :updated_at
	)`

	if s.DB.DriverName() == "mysql" {
		sql = fmt.Sprintf(
			`%s
		ON DUPLICATE KEY UPDATE exchange=:exchange, symbol=:symbol, position_side=:position_side, updated_at=:updated_at`,
			sql)
	}

	_, err = s.DB.NamedExec(sql, risk)
	return err
}

type QueryFundingFeeOptions struct {
	Exchange  string
	Symbol    string
	StartTime *time.Time
	EndTime   *time.Time
}

func (s *FuturesService) QueryFundingFeeHistory(options QueryFundingFeeOptions) ([]types.FundingFee, error) {
	columns := fieldsNamesOf(types.FundingFee{})
	builder := sq.
		Select(columns...).
		From("funding_fees").
		Where(sq.Eq{"exchange": options.Exchange, "symbol": options.Symbol}).
		OrderBy("time DESC")

	if options.StartTime != nil {
		builder = builder.Where(sq.GtOrEq{"time": *options.StartTime})
	}
	if options.EndTime != nil {
		builder = builder.Where(sq.LtOrEq{"time": *options.EndTime})
	}

	sql, args, err := builder.ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := s.DB.Queryx(sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fees []types.FundingFee
	for rows.Next() {
		var fee types.FundingFee
		if err := rows.StructScan(&fee); err != nil {
			return nil, err
		}

		fees = append(fees, fee)
	}

	return fees, nil
}

func (s *FuturesService) InsertFundingFee(fee types.FundingFee) error {
	sql := `
	INSERT INTO funding_fees (
		exchange, symbol, asset, amount, txn, time
	) VALUES (
		:exchange, :symbol, :asset, :amount, :txn, :time
	)`

	if s.DB.DriverName() == "mysql" {
		sql = fmt.Sprintf(`%s
		ON DUPLICATE KEY UPDATE exchange=:exchange, symbol=:symbol, txn=:txn`,
			sql)
	}

	_, err := s.DB.NamedExec(sql, fee)
	return err
}
