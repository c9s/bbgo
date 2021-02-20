package service

type PersistenceServiceFacade struct {
	Redis  *RedisPersistenceService
	Json   *JsonPersistenceService
	Memory *MemoryService
}
