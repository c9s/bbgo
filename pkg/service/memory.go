package service

import (
	"reflect"
	"strings"
	"sync"
)

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
	mu     sync.Mutex
}

func (store *MemoryStore) Save(val interface{}) error {
	store.mu.Lock()
	defer store.mu.Unlock()

	store.memory.Slots[store.Key] = val
	return nil
}

func (store *MemoryStore) Load(val interface{}) error {
	store.mu.Lock()
	defer store.mu.Unlock()

	v := reflect.ValueOf(val)
	if data, ok := store.memory.Slots[store.Key]; ok {
		dataRV := reflect.ValueOf(data)
		v.Elem().Set(dataRV)
	} else {
		return ErrPersistenceNotExists
	}

	return nil
}

func (store *MemoryStore) Reset() error {
	store.mu.Lock()
	defer store.mu.Unlock()

	delete(store.memory.Slots, store.Key)
	return nil
}
