package fixedpoint

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValueScan(t *testing.T) {
	for _, input := range []string{"0.12345678", "-42.125", "1e-3"} {
		t.Run(input, func(t *testing.T) {
			var text, binary Value
			require.NoError(t, text.Scan(input))
			require.NoError(t, binary.Scan([]byte(input)))
			assert.Equal(t, binary, text)
		})
	}
	t.Run("invalid text", func(t *testing.T) {
		var value Value
		assert.Error(t, value.Scan("not-a-number"))
	})
}
