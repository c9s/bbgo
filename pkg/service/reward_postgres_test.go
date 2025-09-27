package service

import (
    "context"
    "testing"

    "github.com/DATA-DOG/go-sqlmock"
    "github.com/jmoiron/sqlx"
    "github.com/stretchr/testify/assert"

    "github.com/c9s/bbgo/pkg/types"
)

// Ensures IN-clause binding uses positional placeholders correctly on Postgres
// and that we don’t emit quoted identifiers for string literals.
func TestRewardService_QueryUnspent_PostgresINClause(t *testing.T) {
    db, mock, err := sqlmock.New()
    if !assert.NoError(t, err) {
        return
    }
    defer db.Close()

    // Expose as a Postgres sqlx DB so sqlx uses $1-style placeholders
    sqlxDB := sqlx.NewDb(db, "postgres")
    defer sqlxDB.Close()

    s := &RewardService{DB: sqlxDB}

    // Expect SQL with positional placeholders: $1 => :exchange, $2,$3 => :rt0,:rt1
    // Note the quoted column name "reward_type" for Postgres dialect.
    mock.ExpectQuery(`SELECT \* FROM rewards WHERE exchange = \$1 AND spent IS FALSE\s+AND "reward_type" IN \(\$2,\s*\$3\) ORDER BY created_at ASC`).
        WithArgs(types.ExchangeMax, string(types.RewardAirdrop), string(types.RewardCommission)).
        WillReturnRows(sqlmock.NewRows([]string{"gid", "uuid", "exchange", "reward_type", "currency", "quantity", "state", "note", "spent", "created_at"}))

    _, err = s.QueryUnspent(context.Background(), types.ExchangeMax, types.RewardAirdrop, types.RewardCommission)
    assert.NoError(t, err)

    assert.NoError(t, mock.ExpectationsWereMet())
}

