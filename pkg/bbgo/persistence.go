package bbgo

import (
	"fmt"
	"reflect"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/service"
)

type PersistenceSelector struct {
	// StoreID is the store you want to use.
	StoreID string `json:"store" yaml:"store"`

	// Type is the persistence type
	Type string `json:"type" yaml:"type"`
}

// Persistence is used for strategy to inject the persistence.
type Persistence struct {
	PersistenceSelector *PersistenceSelector `json:"persistence,omitempty" yaml:"persistence,omitempty"`

	Facade *service.PersistenceServiceFacade `json:"-" yaml:"-"`
}

func (p *Persistence) backendService(t string) (service.PersistenceService, error) {
	switch t {
	case "json":
		return p.Facade.Json, nil

	case "redis":
		if p.Facade.Redis == nil {
			log.Warn("redis persistence is not available, fallback to memory backend")
			return p.Facade.Memory, nil
		}
		return p.Facade.Redis, nil

	case "memory":
		return p.Facade.Memory, nil

	}

	return nil, fmt.Errorf("unsupported persistent type %s", t)
}

func (p *Persistence) Load(val interface{}, subIDs ...string) error {
	ps, err := p.backendService(p.PersistenceSelector.Type)
	if err != nil {
		return err
	}

	log.Debugf("using persistence store %T for loading", ps)

	if p.PersistenceSelector.StoreID == "" {
		p.PersistenceSelector.StoreID = "default"
	}

	store := ps.NewStore(p.PersistenceSelector.StoreID, subIDs...)
	return store.Load(val)
}

func (p *Persistence) Save(val interface{}, subIDs ...string) error {
	ps, err := p.backendService(p.PersistenceSelector.Type)
	if err != nil {
		return err
	}

	log.Debugf("using persistence store %T for storing", ps)

	if p.PersistenceSelector.StoreID == "" {
		p.PersistenceSelector.StoreID = "default"
	}

	store := ps.NewStore(p.PersistenceSelector.StoreID, subIDs...)
	return store.Save(val)
}

type StructFieldIterator func(tag string, ft reflect.StructField, fv reflect.Value) error

func iterateFieldsByTag(obj interface{}, tagName string, cb StructFieldIterator) error {
	sv := reflect.ValueOf(obj)
	st := reflect.TypeOf(obj)

	if st.Kind() != reflect.Ptr {
		return fmt.Errorf("f needs to be a pointer of a struct, %s given", st)
	}

	// solve the reference
	st = st.Elem()
	sv = sv.Elem()

	if st.Kind() != reflect.Struct {
		return fmt.Errorf("f needs to be a struct, %s given", st)
	}

	for i := 0; i < sv.NumField(); i++ {
		fv := sv.Field(i)
		ft := st.Field(i)

		fvt := fv.Type()
		_ = fvt

		// skip unexported fields
		if !st.Field(i).IsExported() {
			continue
		}

		tag, ok := ft.Tag.Lookup(tagName)
		if !ok {
			continue
		}

		if err := cb(tag, ft, fv); err != nil {
			return err
		}
	}

	return nil
}

// https://github.com/xiaojun207/go-base-utils/blob/master/utils/Clone.go
func newTypeValueInterface(typ reflect.Type) interface{} {
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
		dst := reflect.New(typ).Elem()
		return dst.Addr().Interface()
	} else {
		dst := reflect.New(typ)
		return dst.Interface()
	}
}

func loadPersistenceFields(obj interface{}, id string, persistence service.PersistenceService) error {

	return iterateFieldsByTag(obj, "persistence", func(tag string, field reflect.StructField, value reflect.Value) error {
		newValueInf := newTypeValueInterface(value.Type())
		// inf := value.Interface()
		store := persistence.NewStore(id, tag)
		if err := store.Load(&newValueInf); err != nil {
			return err
		}

		newValue := reflect.ValueOf(newValueInf)
		if value.Kind() != reflect.Ptr && newValue.Kind() == reflect.Ptr {
			newValue = newValue.Elem()
		}

		// log.Debugf("%v = %v (%s) -> %v (%s)\n", field, value, value.Type(), newValue, newValue.Type())

		value.Set(newValue)
		return nil
	})
}

func storePersistenceFields(obj interface{}, id string, persistence service.PersistenceService) error {
	return iterateFieldsByTag(obj, "persistence", func(tag string, ft reflect.StructField, fv reflect.Value) error {
		inf := fv.Interface()

		store := persistence.NewStore(id, tag)
		if err := store.Save(inf); err != nil {
			return err
		}

		return nil
	})
}
