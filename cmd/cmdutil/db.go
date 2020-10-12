package cmdutil

import (
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/spf13/viper"
)

func ConnectMySQL() (*sqlx.DB, error) {
	mysqlURL := viper.GetString("mysql-url")
	mysqlURL = fmt.Sprintf("%s?parseTime=true", mysqlURL)
	return sqlx.Connect("mysql", mysqlURL)
}
