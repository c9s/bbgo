package marketdata

import (
	"fmt"
	"os"
	"strconv"
)

// validateByDefault mirrors the MARKETDATA_VALIDATE environment variable. It is
// read once at init so the check costs one bool test per cursor, not a getenv.
var validateByDefault = func() bool {
	v, _ := strconv.ParseBool(os.Getenv("MARKETDATA_VALIDATE"))
	return v
}()

// ValidatingCursor wraps a Cursor and fails with ErrOutOfOrder if the wrapped
// cursor violates the non-decreasing ordering guarantee.
//
// It costs one OrderKey comparison per event, so it is opt-in: tests wrap
// explicitly with Validate, and production wraps via ValidateIfEnabled, which
// honours MARKETDATA_VALIDATE.
type ValidatingCursor struct {
	inner Cursor
	name  string

	last    OrderKey
	hasLast bool
	err     error
}

// Validate wraps c so that out-of-order events become an error.
func Validate(c Cursor, name string) *ValidatingCursor {
	return &ValidatingCursor{inner: c, name: name}
}

// ValidateIfEnabled wraps c only when MARKETDATA_VALIDATE is set to a truthy
// value; otherwise it returns c unchanged.
func ValidateIfEnabled(c Cursor, name string) Cursor {
	if validateByDefault {
		return Validate(c, name)
	}
	return c
}

func (c *ValidatingCursor) Next() bool {
	if c.err != nil {
		return false
	}

	if !c.inner.Next() {
		return false
	}

	ev := c.inner.Event()
	key := ev.Key
	key.SourceIndex = 0

	if c.hasLast && key.Compare(c.last) < 0 {
		c.err = fmt.Errorf("%w: source %s emitted %+v after %+v",
			ErrOutOfOrder, c.name, key, c.last)
		return false
	}

	c.last, c.hasLast = key, true
	return true
}

func (c *ValidatingCursor) Event() *Event { return c.inner.Event() }

func (c *ValidatingCursor) Err() error {
	if c.err != nil {
		return c.err
	}
	return c.inner.Err()
}

func (c *ValidatingCursor) Close() error { return c.inner.Close() }
