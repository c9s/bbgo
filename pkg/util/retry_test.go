package util

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func addAndCheck(a *int, target int) error {
	if *a++; *a == target {
		return nil
	} else {
		return fmt.Errorf("a is not %v. It is %v\n", target, *a)
	}
}

func TestRetry(t *testing.T) {
	type test struct {
		input     int
		targetNum int
		ans       int
		ansErr    error
	}
	tests := []test{
		{input: 0, targetNum: 3, ans: 3, ansErr: nil},
		{input: 0, targetNum: 10, ans: 3, ansErr: errors.New("failed in retry")},
	}

	for _, tc := range tests {
		errHandled := false

		err := Retry(context.Background(), 3, 1*time.Second, func() error {
			return addAndCheck(&tc.input, tc.targetNum)
		}, func(e error) { errHandled = true })

		assert.Equal(t, true, errHandled)
		if tc.ansErr == nil {
			assert.NoError(t, err)
		} else {
			assert.Contains(t, err.Error(), tc.ansErr.Error())
		}
		assert.Equal(t, tc.ans, tc.input)
	}
}

func TestRetryWithPredicator(t *testing.T) {
	type test struct {
		count      int
		f          func() error
		errHandler func(error)
		predicator RetryPredicator
		ansCount   int
		ansErr     error
	}
	knownErr := errors.New("Duplicate entry '1-389837488-1' for key 'UNI_Trade'")
	unknownErr := errors.New("Some Error")
	tests := []test{
		{
			predicator: func(err error) bool {
				return !strings.Contains(err.Error(), "Duplicate entry")
			},
			f:        func() error { return knownErr },
			ansCount: 1,
			ansErr:   knownErr,
		},
		{
			predicator: func(err error) bool {
				return !strings.Contains(err.Error(), "Duplicate entry")
			},
			f:        func() error { return unknownErr },
			ansCount: 3,
			ansErr:   unknownErr,
		},
	}
	attempts := 3
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, tc := range tests {
		err := Retry(ctx, attempts, 100*time.Millisecond, func() error {
			tc.count++
			return tc.f()
		}, tc.errHandler, tc.predicator)

		assert.Equal(t, tc.ansCount, tc.count)
		assert.EqualError(t, errors.Cause(err), tc.ansErr.Error(), "should be equal")
	}
}

func TestRetryCtxCancel(t *testing.T) {
	result := int(0)
	target := int(3)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Retry(ctx, 5, 1*time.Second, func() error { return addAndCheck(&result, target) }, func(error) {})
	assert.Error(t, err)
	fmt.Println("Error:", err.Error())
	assert.Equal(t, int(0), result)
}
