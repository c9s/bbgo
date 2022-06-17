package service

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"

	"github.com/go-redis/redis/v8"
	log "github.com/sirupsen/logrus"
)

type RedisPersistenceService struct {
	redis *redis.Client
}

func NewRedisPersistenceService(config *RedisPersistenceConfig) *RedisPersistenceService {
	client := redis.NewClient(&redis.Options{
		Addr: net.JoinHostPort(config.Host, config.Port),
		// Username:           "", // username is only for redis 6.0
		// pragma: allowlist nextline secret
		Password: config.Password, // no password set
		DB:       config.DB,       // use default DB
	})

	return &RedisPersistenceService{
		redis: client,
	}
}

func (s *RedisPersistenceService) NewStore(id string, subIDs ...string) Store {
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
	if store.redis == nil {
		return errors.New("can not load from redis, possible cause: redis persistence is not configured, or you are trying to use redis in back-test")
	}

	cmd := store.redis.Get(context.Background(), store.ID)
	data, err := cmd.Result()

	log.Debugf("[redis] get key %q, data = %s", store.ID, string(data))

	if err != nil {
		if err == redis.Nil {
			return ErrPersistenceNotExists
		}

		return err
	}

	// skip null data
	if len(data) == 0 || data == "null" {
		return ErrPersistenceNotExists
	}

	return json.Unmarshal([]byte(data), val)
}

func (store *RedisStore) Save(val interface{}) error {
	if val == nil {
		return nil
	}

	data, err := json.Marshal(val)
	if err != nil {
		return err
	}

	cmd := store.redis.Set(context.Background(), store.ID, data, 0)
	_, err = cmd.Result()

	log.Debugf("[redis] set key %q, data = %s", store.ID, string(data))

	return err
}

func (store *RedisStore) Reset() error {
	_, err := store.redis.Del(context.Background(), store.ID).Result()
	return err
}
