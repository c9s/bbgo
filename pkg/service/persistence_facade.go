package service

type PersistenceServiceFacade struct {
	Redis  *RedisPersistenceService
	Json   *JsonPersistenceService
	Memory *MemoryService
}

// Get returns the preferred persistence service by fallbacks
// Redis will be preferred at the first position.
func (facade *PersistenceServiceFacade) Get() PersistenceService {
	if facade.Redis != nil {
		return facade.Redis
	}

	if facade.Json != nil {
		return facade.Json
	}

	return facade.Memory
}
