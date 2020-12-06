package bbgo

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net"
	"path/filepath"

	"github.com/go-redis/redis/v8"
)

type PersistenceService interface {
	NewStore(id string) Store
}

type Store interface {
	Load(val interface{}) error
	Save(val interface{}) error
}

type JsonPersistenceService struct {
	Directory string
}

func (s *JsonPersistenceService) NewStore(id string) *JsonStore {
	return &JsonStore{
		ID:       id,
		Filepath: filepath.Join(s.Directory, id) + ".json",
	}
}

type JsonStore struct {
	ID       string
	Filepath string
}

func (store JsonStore) Load(val interface{}) error {
	data, err := ioutil.ReadFile(store.Filepath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, val)
}

func (store JsonStore) Save(val interface{}) error {
	data, err := json.Marshal(val)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(store.Filepath, data, 0666)
}

type RedisPersistenceService struct {
	redis *redis.Client
}

func NewRedisPersistenceService(config *RedisPersistenceConfig) *RedisPersistenceService {
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

func (s *RedisPersistenceService) NewStore(id string) *RedisStore {
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
		return err
	}

	if err := json.Unmarshal([]byte(data), val); err != nil {
		return err
	}

	return nil
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
