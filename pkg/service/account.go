package service

import (
	"github.com/c9s/bbgo/pkg/types"
	"github.com/jmoiron/sqlx"
	"go.uber.org/multierr"
	"time"
)

type AccountService struct {
	DB *sqlx.DB
}

func NewAccountService(db *sqlx.DB) *AccountService {
	return &AccountService{DB: db}
}

func (s *AccountService) InsertAsset(time time.Time, name types.ExchangeName, account string, assets types.AssetMap) error {

	if s.DB == nil {
		//skip db insert when no db connection setting.
		return nil
	}

	var err error
	for _, v := range assets {
		_, _err := s.DB.Exec(`
			insert into nav_history_details ( exchange, subaccount, time, currency, balance_in_usd, balance_in_btc,
		                                              balance,available,locked)
				values (?,?,?,?,?,?,?,?,?);
		`, name, account, time, v.Currency, v.InUSD, v.InBTC, v.Total, 0, 0 /* v.Available, v.Lock */)

		err = multierr.Append(err, _err) // successful request

	}
	return err
}
