package bbgo

import (
	"context"
	"os"
	"reflect"
	"sync"

	"github.com/codingconcepts/env"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/dynamic"
	"github.com/c9s/bbgo/pkg/service"
)

var defaultPersistenceServiceFacade = &service.PersistenceServiceFacade{
	Memory: service.NewMemoryService(),
}

// Sync syncs the object properties into the persistence layer
func Sync(ctx context.Context, obj interface{}) {
	id := dynamic.CallID(obj)
	if len(id) == 0 {
		log.Warnf("InstanceID() is not provided, can not sync persistence")
		return
	}

	isolation := GetIsolationFromContext(ctx)

	ps := isolation.persistenceServiceFacade.Get()

	locker, ok := obj.(sync.Locker)
	if ok {
		locker.Lock()
		defer locker.Unlock()
	}

	err := storePersistenceFields(obj, id, ps)
	if err != nil {
		log.WithError(err).Errorf("persistence sync failed")
	}
}

func loadPersistenceFields(obj interface{}, id string, persistence service.PersistenceService) error {
	return dynamic.IterateFieldsByTag(obj, "persistence", true, func(tag string, field reflect.StructField, value reflect.Value) error {
		log.Debugf("[loadPersistenceFields] loading value into field %v, tag = %s, original value = %v", field, tag, value)

		newValueInf := dynamic.NewTypeValueInterface(value.Type())
		// inf := value.Interface()
		store := persistence.NewStore("state", id, tag)
		if err := store.Load(&newValueInf); err != nil {
			if err == service.ErrPersistenceNotExists {
				log.Debugf("[loadPersistenceFields] state key does not exist, id = %v, tag = %s", id, tag)
				return nil
			}

			return err
		}

		newValue := reflect.ValueOf(newValueInf)
		if value.Kind() != reflect.Ptr && newValue.Kind() == reflect.Ptr {
			newValue = newValue.Elem()
		}

		log.Debugf("[loadPersistenceFields] %v = %v -> %v\n", field, value, newValue)

		value.Set(newValue)
		return nil
	})
}

func storePersistenceFields(obj interface{}, id string, persistence service.PersistenceService) error {
	return dynamic.IterateFieldsByTag(obj, "persistence", true, func(tag string, ft reflect.StructField, fv reflect.Value) error {
		log.Debugf("[storePersistenceFields] storing value from field %v, tag = %s, original value = %v", ft, tag, fv)

		inf := fv.Interface()
		store := persistence.NewStore("state", id, tag)
		return store.Save(inf)
	})
}

func NewPersistenceServiceFacade(conf *PersistenceConfig) (*service.PersistenceServiceFacade, error) {
	facade := &service.PersistenceServiceFacade{
		Memory: service.NewMemoryService(),
	}

	if conf.Redis != nil {
		if err := env.Set(conf.Redis); err != nil {
			return nil, err
		}

		redisPersistence := service.NewRedisPersistenceService(conf.Redis)
		facade.Redis = redisPersistence
	}

	if conf.Json != nil {
		if _, err := os.Stat(conf.Json.Directory); os.IsNotExist(err) {
			if err2 := os.MkdirAll(conf.Json.Directory, 0777); err2 != nil {
				return nil, errors.Wrapf(err2, "can not create directory: %s", conf.Json.Directory)
			}
		}

		jsonPersistence := &service.JsonPersistenceService{Directory: conf.Json.Directory}
		facade.Json = jsonPersistence
	}

	return facade, nil
}

func ConfigurePersistence(ctx context.Context, environ *Environment, conf *PersistenceConfig) error {
	facade, err := NewPersistenceServiceFacade(conf)
	if err != nil {
		return err
	}

	isolation := GetIsolationFromContext(ctx)
	isolation.persistenceServiceFacade = facade

	environ.PersistentService = facade
	return nil
}
