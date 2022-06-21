package optimizer

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_jsonToYamlConfig(t *testing.T) {
	err := os.Mkdir(".tmpconfig", 0755)
	assert.NoError(t, err)

	tf, err := jsonToYamlConfig(".tmpconfig", []byte(`{
	}`))
	assert.NoError(t, err)
	assert.NotNil(t, tf)
	assert.NotEmpty(t, tf.Name())

	_ = os.RemoveAll(".tmpconfig")
}
