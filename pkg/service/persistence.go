package service

import (
	"time"

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

type Expirable interface {
	Expiration() time.Duration
}

type RedisPersistenceConfig struct {
	Host      string `yaml:"host" json:"host" env:"REDIS_HOST"`
	Port      string `yaml:"port" json:"port" env:"REDIS_PORT"`
	Password  string `yaml:"password,omitempty" json:"password,omitempty" env:"REDIS_PASSWORD"`
	DB        int    `yaml:"db" json:"db" env:"REDIS_DB"`
	Namespace string `yaml:"namespace" json:"namespace" env:"REDIS_NAMESPACE"`

	// Redis is the redis client field
	// this field is optional, only used when you want to set the redis client instance in the runtime
	Redis *redis.Client
}

type JsonPersistenceConfig struct {
	Directory string `yaml:"directory" json:"directory"`
}
