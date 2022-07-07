package dynamic

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type callTest struct {
	ChildCall1 *childCall1
	ChildCall2 *childCall2
}

type childCall1 struct{}

func (c *childCall1) Subscribe(a int) {}

type childCall2 struct{}

func (c *childCall2) Subscribe(a int) {}

func TestCallStructFieldsMethod(t *testing.T) {
	c := &callTest{
		ChildCall1: &childCall1{},
		ChildCall2: &childCall2{},
	}
	err := CallStructFieldsMethod(c, "Subscribe", 10)
	assert.NoError(t, err)
}

type S struct {
	ID string
}

func (s *S) String() string { return s.ID }

func TestCallMatch(t *testing.T) {
	t.Run("simple", func(t *testing.T) {
		f := func(a int, b int) {
			assert.Equal(t, 1, a)
			assert.Equal(t, 2, b)
		}
		_, err := CallMatch(f, 1, 2)
		assert.NoError(t, err)
	})

	t.Run("interface", func(t *testing.T) {
		type A interface {
			String() string
		}
		f := func(foo int, a A) {
			assert.Equal(t, "foo", a.String())
		}
		_, err := CallMatch(f, 10, &S{ID: "foo"})
		assert.NoError(t, err)
	})

	t.Run("nil interface", func(t *testing.T) {
		type A interface {
			String() string
		}
		f := func(foo int, a A) {
			assert.Equal(t, 10, foo)
			assert.Nil(t, a)
		}
		_, err := CallMatch(f, 10)
		assert.NoError(t, err)
	})

	t.Run("struct pointer", func(t *testing.T) {
		f := func(foo int, s *S) {
			assert.Equal(t, 10, foo)
			assert.NotNil(t, s)
		}
		_, err := CallMatch(f, 10, &S{})
		assert.NoError(t, err)
	})

	t.Run("struct pointer x 2", func(t *testing.T) {
		f := func(foo int, s1, s2 *S) {
			assert.Equal(t, 10, foo)
			assert.Equal(t, "s1", s1.String())
			assert.Equal(t, "s2", s2.String())
		}
		_, err := CallMatch(f, 10, &S{ID: "s1"}, &S{ID: "s2"})
		assert.NoError(t, err)
	})

	t.Run("func factory", func(t *testing.T) {
		f := func(s *S) {
			assert.Equal(t, "factory", s.String())
		}
		_, err := CallMatch(f, func() *S {
			return &S{ID: "factory"}
		})
		assert.NoError(t, err)
	})

	t.Run("nil", func(t *testing.T) {
		f := func(s *S) {
			assert.Nil(t, s)
		}
		_, err := CallMatch(f)
		assert.NoError(t, err)
	})

	t.Run("zero struct", func(t *testing.T) {
		f := func(s S) {
			assert.Equal(t, S{}, s)
		}
		_, err := CallMatch(f)
		assert.NoError(t, err)
	})

}
