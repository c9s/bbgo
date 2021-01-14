package cmdutil

import (
	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

func ConnectMySQL(dsn string) (*sqlx.DB, error) {
	config, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, err
	}

	config.ParseTime = true
	dsn = config.FormatDSN()
	return sqlx.Connect("mysql", dsn)
}
