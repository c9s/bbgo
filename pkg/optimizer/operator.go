package optimizer

type OpFunc func(configJson []byte, next func(configJson []byte) error) error
