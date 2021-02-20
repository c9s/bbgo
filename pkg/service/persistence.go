package service

type PersistenceService interface {
	NewStore(id string, subIDs ...string) Store
}

type Store interface {
	Load(val interface{}) error
	Save(val interface{}) error
	Reset() error
}

type RedisPersistenceConfig struct {
	Host     string `json:"host" env:"REDIS_HOST"`
	Port     string `json:"port" env:"REDIS_PORT"`
	Password string `json:"password" env:"REDIS_PASSWORD"`
	DB       int    `json:"db" env:"REDIS_DB"`
}

type JsonPersistenceConfig struct {
	Directory string `json:"directory"`
}

