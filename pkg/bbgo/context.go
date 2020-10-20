package bbgo

import (
	"sync"
)

// deprecated: legacy context struct
type Context struct {
	sync.Mutex
}

