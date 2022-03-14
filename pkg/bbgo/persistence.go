package bbgo

import (
	"fmt"

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
