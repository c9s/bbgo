package apikey

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnvKeyLoader_Load(t *testing.T) {
	// Simulate environment variables
	envVars := []string{
		"TEST_API_KEY_1=key1",
		"TEST_API_SECRET_1=secret1",
		"TEST_API_EXTRA_1=extra1",
		"TEST_API_KEY_2=key2",
		"TEST_API_SECRET_2=secret2",
		"TEST_API_EXTRA_2=extra2",
		"INVALID_ENV_VAR=value",
	}

	// Initialize EnvKeyLoader
	loader := NewEnvKeyLoader("TEST_API_", "", "KEY", "SECRET", "EXTRA")

	// Load environment variables
	source, err := loader.Load(envVars)

	// Assert no error occurred
	if assert.NoError(t, err) {

		// Assert the returned Source is not nil
		if assert.NotNil(t, source) {
			t.Logf("loaded source: %+v", source)
		}
	}

	// Assert the number of Entries in Source
	if assert.Len(t, source.Entries, 2) {
		// Validate the first Entry
		entry1 := source.Entries[0]
		assert.Equal(t, 1, entry1.Index)
		assert.Equal(t, "key1", entry1.Fields["KEY"])
		assert.Equal(t, "secret1", entry1.Fields["SECRET"])
		assert.Equal(t, "extra1", entry1.Fields["EXTRA"])

		// Validate the second Entry
		entry2 := source.Entries[1]
		assert.Equal(t, 2, entry2.Index)
		assert.Equal(t, "key2", entry2.Fields["KEY"])
		assert.Equal(t, "secret2", entry2.Fields["SECRET"])
		assert.Equal(t, "extra2", entry2.Fields["EXTRA"])
	}
}

func Test_NewSourceFromArray(t *testing.T) {
	data := [][]string{
		{"key1", "secret1"},
		{"key2", "secret2"},
		{"key3", "secret3"},
	}

	source := NewSourceFromArray(data)
	assert.NotNil(t, source)
	assert.Len(t, source.Entries, 3)
	for _, entry := range source.Entries {
		t.Logf("Index: %d, Key: %s, Secret: %s\n", entry.Index, entry.Key, entry.Secret)
	}
}
