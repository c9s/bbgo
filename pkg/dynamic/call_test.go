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
