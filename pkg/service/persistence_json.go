package service

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
)

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
		if err2 := os.MkdirAll(store.Directory, 0777); err2 != nil {
			return err2
		}
	}

	p := filepath.Join(store.Directory, store.ID) + ".json"

	if _, err := os.Stat(p); os.IsNotExist(err) {
		return ErrPersistenceNotExists
	}

	data, err := ioutil.ReadFile(p)
	if err != nil {
		return err
	}

	if len(data) == 0 {
		return ErrPersistenceNotExists
	}

	return json.Unmarshal(data, val)
}

func (store JsonStore) Save(val interface{}) error {
	if _, err := os.Stat(store.Directory); os.IsNotExist(err) {
		if err2 := os.MkdirAll(store.Directory, 0777); err2 != nil {
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
