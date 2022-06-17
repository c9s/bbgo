package service

import (
	"context"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func TestRewardService_InsertAndQueryUnspent(t *testing.T) {
	db, err := prepareDB(t)
	if err != nil {
		t.Fatal(err)
	}

	defer db.Close()

	ctx := context.Background()

	xdb := sqlx.NewDb(db.DB, "sqlite3")
	service := &RewardService{DB: xdb}

	err = service.Insert(types.Reward{
		UUID:      "test01",
		Exchange:  "max",
		Type:      "commission",
		Currency:  "BTC",
		Quantity:  fixedpoint.One,
		State:     "done",
		Spent:     false,
		CreatedAt: types.Time(time.Now()),
	})
	assert.NoError(t, err)

	rewards, err := service.QueryUnspent(ctx, types.ExchangeMax)
	assert.NoError(t, err)
	assert.NotEmpty(t, rewards)
	assert.Len(t, rewards, 1)
	assert.Equal(t, types.RewardCommission, rewards[0].Type)

	err = service.Insert(types.Reward{
		UUID:      "test02",
		Exchange:  "max",
		Type:      "airdrop",
		Currency:  "MAX",
		Quantity:  fixedpoint.NewFromInt(1000000),
		State:     "done",
		Spent:     false,
		CreatedAt: types.Time(time.Now()),
	})
	assert.NoError(t, err)

	rewards, err = service.QueryUnspent(ctx, types.ExchangeMax)
	assert.NoError(t, err)
	assert.NotEmpty(t, rewards)
	assert.Len(t, rewards, 1, "airdrop should not be included")
	assert.Equal(t, types.RewardCommission, rewards[0].Type)

	rewards, err = service.QueryUnspent(ctx, types.ExchangeMax, types.RewardAirdrop)
	assert.NoError(t, err)
	assert.NotEmpty(t, rewards)
	assert.Len(t, rewards, 1, "airdrop should be included")
	assert.Equal(t, types.RewardAirdrop, rewards[0].Type)

	rewards, err = service.QueryUnspent(ctx, types.ExchangeMax, types.RewardCommission)
	assert.NoError(t, err)
	assert.NotEmpty(t, rewards)
	assert.Len(t, rewards, 1, "should select 1 reward")
	assert.Equal(t, types.RewardCommission, rewards[0].Type)
}

func TestRewardService_AggregateUnspentCurrencyPosition(t *testing.T) {
	db, err := prepareDB(t)
	if err != nil {
		t.Fatal(err)
	}

	defer db.Close()

	ctx := context.Background()

	xdb := sqlx.NewDb(db.DB, "sqlite3")
	service := &RewardService{DB: xdb}

	now := time.Now()

	err = service.Insert(types.Reward{
		UUID:      "test01",
		Exchange:  "max",
		Type:      "commission",
		Currency:  "BTC",
		Quantity:  fixedpoint.One,
		State:     "done",
		Spent:     false,
		CreatedAt: types.Time(now),
	})
	assert.NoError(t, err)

	err = service.Insert(types.Reward{
		UUID:      "test02",
		Exchange:  "max",
		Type:      "commission",
		Currency:  "LTC",
		Quantity:  fixedpoint.NewFromInt(2),
		State:     "done",
		Spent:     false,
		CreatedAt: types.Time(now),
	})
	assert.NoError(t, err)

	err = service.Insert(types.Reward{
		UUID:      "test03",
		Exchange:  "max",
		Type:      "airdrop",
		Currency:  "MAX",
		Quantity:  fixedpoint.NewFromInt(1000000),
		State:     "done",
		Spent:     false,
		CreatedAt: types.Time(now),
	})
	assert.NoError(t, err)

	currencyPositions, err := service.AggregateUnspentCurrencyPosition(ctx, types.ExchangeMax, now.Add(-10*time.Second))
	assert.NoError(t, err)
	assert.NotEmpty(t, currencyPositions)
	assert.Len(t, currencyPositions, 2)

	v, ok := currencyPositions["LTC"]
	assert.True(t, ok)
	assert.Equal(t, fixedpoint.NewFromInt(2), v)

	v, ok = currencyPositions["BTC"]
	assert.True(t, ok)
	assert.Equal(t, fixedpoint.One, v)
}
