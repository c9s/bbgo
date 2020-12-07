package bbgo

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/go-redis/redis/v8"
)

type PersistenceService interface {
	NewStore(id string, subIDs ...string) Store
}

type Store interface {
	Load(val interface{}) error
	Save(val interface{}) error
	Reset() error
}

type MemoryService struct {
	Slots map[string]interface{}
}

func NewMemoryService() *MemoryService {
	return &MemoryService{
		Slots: make(map[string]interface{}),
	}
}

func (s *MemoryService) NewStore(id string, subIDs ...string) Store {
	key := strings.Join(append([]string{id}, subIDs...), ":")
	return &MemoryStore{
		Key:    key,
		memory: s,
	}
}

type MemoryStore struct {
	Key    string
	memory *MemoryService
}

func (store *MemoryStore) Save(val interface{}) error {
	store.memory.Slots[store.Key] = val
	return nil
}

func (store *MemoryStore) Load(val interface{}) error {
	v := reflect.ValueOf(val)
	if data, ok := store.memory.Slots[store.Key]; ok {
		v.Elem().Set(reflect.ValueOf(data).Elem())
	} else {
		return os.ErrNotExist
	}
	return nil
}

func (store *MemoryStore) Reset() error {
	delete(store.memory.Slots, store.Key)
	return nil
}

type JsonPersistenceService struct {
	Directory string
}

func (s *JsonPersistenceService) NewStore(id string, subIDs ...string) Store {
	return &JsonStore{
		ID:        id,
		Directory: filepath.Join(append([]string{s.Directory}, subIDs...)...),
	}
}

type JsonStore struct {
	ID        string
	Directory string
}

func (store JsonStore) Reset() error {
	if _, err := os.Stat(store.Directory); os.IsNotExist(err) {
		return nil
	}

	p := filepath.Join(store.Directory, store.ID) + ".json"
	if _, err := os.Stat(p); os.IsNotExist(err) {
		return nil
	}

	return os.Remove(p)
}

func (store JsonStore) Load(val interface{}) error {
	if _, err := os.Stat(store.Directory); os.IsNotExist(err) {
		if err2 := os.Mkdir(store.Directory, 0777); err2 != nil {
			return err2
		}
	}

	p := filepath.Join(store.Directory, store.ID) + ".json"

	if _, err := os.Stat(p); os.IsNotExist(err) {
		return os.ErrNotExist
	}

	data, err := ioutil.ReadFile(p)
	if err != nil {
		return err
	}

	if len(data) == 0 {
		return os.ErrNotExist
	}

	return json.Unmarshal(data, val)
}

func (store JsonStore) Save(val interface{}) error {
	if _, err := os.Stat(store.Directory); os.IsNotExist(err) {
		if err2 := os.Mkdir(store.Directory, 0777); err2 != nil {
			return err2
		}
	}

	data, err := json.Marshal(val)
	if err != nil {
		return err
	}

	p := filepath.Join(store.Directory, store.ID) + ".json"
	return ioutil.WriteFile(p, data, 0666)
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
	cmd := store.redis.Get(context.Background(), store.ID)
	data, err := cmd.Result()
	if err != nil {
		if err == redis.Nil {
			return os.ErrNotExist
		}

		return err
	}

	if len(data) == 0 {
		return os.ErrNotExist
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
