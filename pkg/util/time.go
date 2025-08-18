package util

import (
	"time"
)

func UnixMilli() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}
