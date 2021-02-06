package service

import (
	"context"

	"github.com/c9s/rockhopper"
	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

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

func (s *DatabaseService) Close() error {
	return s.DB.Close()
}

func (s *DatabaseService) Upgrade(ctx context.Context) error {
	dialect, err := rockhopper.LoadDialect(s.Driver)
	if err != nil {
		return err
	}

	loader := &rockhopper.GoMigrationLoader{}
	migrations, err := loader.Load()
	if err != nil {
		return err
	}

	// sqlx.DB is different from sql.DB
	rh := rockhopper.New(s.Driver, dialect, s.DB.DB)

	currentVersion, err := rh.CurrentVersion()
	if err != nil {
		return err
	}

	if err := rockhopper.Up(ctx, rh, migrations, currentVersion, 0); err != nil {
		return err
	}

	return nil
}

func ReformatMysqlDSN(dsn string) (string, error) {
	config, err := mysql.ParseDSN(dsn)
	if err != nil {
		return "", err
	}

	config.ParseTime = true
	dsn = config.FormatDSN()
	return dsn, nil
}
