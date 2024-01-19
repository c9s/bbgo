package service

import (
	"context"

	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"

	"github.com/c9s/rockhopper/v2"

	mysqlMigrations "github.com/c9s/bbgo/pkg/migrations/mysql"
	sqlite3Migrations "github.com/c9s/bbgo/pkg/migrations/sqlite3"
)

// reflect cache for database
var dbCache = NewReflectCache()

type DatabaseService struct {
	Driver string
	DSN    string
	DB     *sqlx.DB
}

func NewDatabaseService(driver, dsn string) *DatabaseService {
	if driver == "mysql" {
		var err error
		dsn, err = ReformatMysqlDSN(dsn)
		if err != nil {
			// incorrect mysql dsn is logical exception
			panic(err)
		}
	}

	return &DatabaseService{
		Driver: driver,
		DSN:    dsn,
	}

}

func (s *DatabaseService) Connect() error {
	var err error
	s.DB, err = sqlx.Connect(s.Driver, s.DSN)
	return err
}

func (s *DatabaseService) Insert(record interface{}) error {
	sql := dbCache.InsertSqlOf(record)
	_, err := s.DB.NamedExec(sql, record)
	return err
}

func (s *DatabaseService) Close() error {
	return s.DB.Close()
}

func (s *DatabaseService) Upgrade(ctx context.Context) error {
	dialect, err := rockhopper.LoadDialect(s.Driver)
	if err != nil {
		return err
	}

	var migrations rockhopper.MigrationSlice

	switch s.Driver {
	case "sqlite3":
		migrations = sqlite3Migrations.Migrations()
	case "mysql":
		migrations = mysqlMigrations.Migrations()

	}

	// sqlx.DB is different from sql.DB
	rh := rockhopper.New(s.Driver, dialect, s.DB.DB, rockhopper.TableName)

	if err := rh.Touch(ctx); err != nil {
		return err
	}

	migrations = migrations.FilterPackage([]string{"main"}).SortAndConnect()
	if len(migrations) == 0 {
		return nil
	}

	_, lastAppliedMigration, err := rh.FindLastAppliedMigration(ctx, migrations)
	if err != nil {
		return err
	}

	if lastAppliedMigration != nil {
		return rockhopper.Up(ctx, rh, lastAppliedMigration.Next, 0)
	}

	// TODO: use align in the next major version
	// return rockhopper.Align(ctx, rh, 20231123125402, migrations)
	return rockhopper.Up(ctx, rh, migrations.Head(), 0)
}

func ReformatMysqlDSN(dsn string) (string, error) {
	config, err := mysql.ParseDSN(dsn)
	if err != nil {
		return "", err
	}

	// we need timestamp and datetime fields to be parsed into time.Time struct
	config.ParseTime = true
	dsn = config.FormatDSN()
	return dsn, nil
}
