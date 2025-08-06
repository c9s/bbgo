package bfxapi

import (
	"strconv"
	"sync/atomic"
	"time"
)

type EpochNonceGenerator struct {
	nonce    uint64
	lastTime uint64
	getTime  func() uint64
}

// GetNonce is a naive nonce producer that takes the current Unix nano epoch
// and counts upwards.
// This is a naive approach because the nonce bound to the currently used API
// key and as such needs to be synchronised with other instances using the same
// key in order to avoid race conditions.
func (u *EpochNonceGenerator) GetNonce() (nonceStr string) {
	currentTime := u.getTime()
	if currentTime > u.lastTime {
		atomic.StoreUint64(&u.nonce, currentTime)
		nonceStr = strconv.FormatUint(atomic.AddUint64(&u.nonce, 0), 10)
		u.lastTime = currentTime
	} else {
		nonceStr = strconv.FormatUint(atomic.AddUint64(&u.nonce, 1), 10)
	}
	return nonceStr
}

func NewEpochNonceGenerator() *EpochNonceGenerator {
	nonce := nanoSecondTimeStamp()
	return &EpochNonceGenerator{
		nonce:    nonce,
		lastTime: nonce,
		getTime:  nanoSecondTimeStamp,
	}
}

func NewV2EpochNonceGenerator() *EpochNonceGenerator {
	nonce := milliSecondTimeStamp()
	return &EpochNonceGenerator{
		nonce:    nonce,
		lastTime: nonce,
		getTime:  milliSecondTimeStamp,
	}
}

func milliSecondTimeStamp() uint64 {
	// it returns the second * 1000
	return uint64(time.Now().Unix()) * 1000
}

func nanoSecondTimeStamp() uint64 {
	// it returns the second * 1e9
	return uint64(time.Now().Unix()) * 1e9
}
