package bbgo

type PersistenceServiceFacade struct {
	Redis *RedisPersistenceService
	Json *JsonPersistenceService
}
