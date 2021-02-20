package service

import (
	"context"
	"encoding/json"
	"net"
	"strings"

	"github.com/go-redis/redis/v8"

	"github.com/c9s/bbgo/pkg/bbgo"
)

type RedisPersistenceService struct {
	redis *redis.Client
}

func NewRedisPersistenceService(config *bbgo.RedisPersistenceConfig) *RedisPersistenceService {
	client := redis.NewClient(&redis.Options{
		Addr: net.JoinHostPort(config.Host, config.Port),
		// Username:           "", // username is only for redis 6.0
		Password: config.Password, // no password set
		DB:       config.DB,       // use default DB
	})

	return &RedisPersistenceService{
		redis: client,
	}
}

func (s *RedisPersistenceService) NewStore(id string, subIDs ...string) bbgo.Store {
	if len(subIDs) > 0 {
		id += ":" + strings.Join(subIDs, ":")
	}

	return &RedisStore{
		redis: s.redis,
		ID:    id,
	}
}

type RedisStore struct {
	redis *redis.Client

	ID string
}

func (store *RedisStore) Load(val interface{}) error {
	cmd := store.redis.Get(context.Background(), store.ID)
	data, err := cmd.Result()
	if err != nil {
		if err == redis.Nil {
			return ErrPersistenceNotExists
		}

		return err
	}

	if len(data) == 0 {
		return ErrPersistenceNotExists
	}

	return json.Unmarshal([]byte(data), val)
}

func (store *RedisStore) Save(val interface{}) error {
	data, err := json.Marshal(val)
	if err != nil {
		return err
	}

	cmd := store.redis.Set(context.Background(), store.ID, data, 0)
	_, err = cmd.Result()
	return err
}

func (store *RedisStore) Reset() error {
	_, err := store.redis.Del(context.Background(), store.ID).Result()
	return err
}
