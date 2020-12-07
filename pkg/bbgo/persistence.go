package bbgo

import "fmt"

type PersistenceSelector struct {
	// StoreID is the store you want to use.
	StoreID string `json:"store" yaml:"store"`

	// Type is the persistence type
	Type string `json:"type" yaml:"type"`
}

// Persistence is used for strategy to inject the persistence.
type Persistence struct {
	PersistenceSelector *PersistenceSelector `json:"persistence,omitempty" yaml:"persistence,omitempty"`

	Facade *PersistenceServiceFacade `json:"-" yaml:"-"`
}

func (p *Persistence) backendService(t string) (service PersistenceService, err error) {
	switch t {
	case "json":
		service = p.Facade.Json

	case "redis":
		service = p.Facade.Redis

	case "memory":
		service = p.Facade.Memory

	default:
		err = fmt.Errorf("unsupported persistent type %s", t)
	}

	return service, err
}

func (p *Persistence) Load(val interface{}, subIDs ...string) error {
	service, err := p.backendService(p.PersistenceSelector.Type)
	if err != nil {
		return err
	}

	if p.PersistenceSelector.StoreID == "" {
		return fmt.Errorf("persistence.store can not be empty")
	}

	store := service.NewStore(p.PersistenceSelector.StoreID, subIDs...)
	return store.Load(val)
}

func (p *Persistence) Save(val interface{}, subIDs ...string) error {
	service, err := p.backendService(p.PersistenceSelector.Type)
	if err != nil {
		return err
	}

	if p.PersistenceSelector.StoreID == "" {
		return fmt.Errorf("persistence.store can not be empty")
	}

	store := service.NewStore(p.PersistenceSelector.StoreID, subIDs...)
	return store.Save(val)
}

type PersistenceServiceFacade struct {
	Redis *RedisPersistenceService
	Json  *JsonPersistenceService
	Memory *MemoryService
}
