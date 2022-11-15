package grid2

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStrategy_checkRequiredInvestmentByQuantity(t *testing.T) {
	s := &Strategy{}

	t.Run("basic base balance check", func(t *testing.T) {
		err := s.checkRequiredInvestmentByQuantity(number(2.0), number(10_000.0),
			number(1.0), number(10_000.0),
			number(0.1), number(19000.0), []Pin{})
		assert.Error(t, err)
		assert.EqualError(t, err, "baseInvestment setup 2.000000 is greater than the total base balance 1.000000")
	})

	t.Run("basic quote balance check", func(t *testing.T) {
		err := s.checkRequiredInvestmentByQuantity(number(1.0), number(10_000.0),
			number(1.0), number(100.0),
			number(0.1), number(19_000.0), []Pin{})
		assert.Error(t, err)
		assert.EqualError(t, err, "quoteInvestment setup 10000.000000 is greater than the total quote balance 100.000000")
	})
}
