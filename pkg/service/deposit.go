package service

import (
	"github.com/jmoiron/sqlx"

	"github.com/c9s/bbgo/pkg/types"
)

type DepositService struct {
	DB *sqlx.DB
}

func (s *DepositService) QueryLast(ex types.ExchangeName, limit int) ([]types.Deposit, error) {
	sql := "SELECT * FROM `deposits` WHERE `exchange` = :exchange ORDER BY `time` DESC LIMIT :limit"
	rows, err := s.DB.NamedQuery(sql, map[string]interface{}{
		"exchange": ex,
		"limit":    limit,
	})
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	return s.scanRows(rows)
}

func (s *DepositService) Query(exchangeName types.ExchangeName) ([]types.Deposit, error) {
	args := map[string]interface{}{
		"exchange": exchangeName,
	}
	sql := "SELECT * FROM `deposits` WHERE `exchange` = :exchange ORDER BY `time` ASC"
	rows, err := s.DB.NamedQuery(sql, args)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	return s.scanRows(rows)
}

func (s *DepositService) scanRows(rows *sqlx.Rows) (deposits []types.Deposit, err error) {
	for rows.Next() {
		var deposit types.Deposit
		if err := rows.StructScan(&deposit); err != nil {
			return deposits, err
		}

		deposits = append(deposits, deposit)
	}

	return deposits, rows.Err()
}

func (s *DepositService) Insert(deposit types.Deposit) error {
	sql := `INSERT INTO deposits (exchange, asset, address, amount, txn_id, time)
			VALUES (:exchange, :asset, :address, :amount, :txn_id, :time)`
	_, err := s.DB.NamedExec(sql, deposit)
	return err
}
