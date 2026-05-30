package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/jmoiron/sqlx"
)

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
	ctx context.Context, exchange types.ExchangeRiskService, currentTime time.Time, symbol ...string) error {
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
		if err := s.Insert(risk); err != nil {
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
	ctx context.Context, service types.ExchangeRiskService, symbol string,
) error {
	// TODO: sync the position history of the given time range
	// we only sync the lastest position risk record for now.
	// Binance does not provide the position risk history API for the time being.
	risks, err := service.QueryPositionRisk(ctx, symbol)
	if err != nil {
		return fmt.Errorf("failed to query position risk: %w", err)
	}
	if len(risks) == 0 {
		return nil
	}

	risk := risks[0]
	risk.UpdateTime = types.MillisecondTimestamp(time.Now())
	if err := s.Insert(risk); err != nil {
		return fmt.Errorf("failed to insert position risk (%+v): %w", risk, err)
	}
	return nil
}

func (s *FuturesService) Query(options QueryFuturesPositionRiskOptions) ([]types.PositionRisk, error) {
	builder := sq.
		Select("*").
		From("futures_position_risks").
		Where(sq.Eq{"exchange": options.Exchange, "symbol": options.Symbol}).
		OrderBy("update_time DESC")
	sql, args, err := builder.ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := s.DB.NamedQuery(sql, args)
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

func (s *FuturesService) Insert(risk types.PositionRisk) (err error) {
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
