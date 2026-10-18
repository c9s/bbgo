package testutil

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func maskSecret(s string) string {
	re := regexp.MustCompile(`\b(\w{4})\w+\b`)
	s = re.ReplaceAllString(s, "$1******")
	return s
}

func IntegrationTestConfigured(t *testing.T, prefix string) (key, secret string, ok bool) {
	var hasKey, hasSecret bool
	key, hasKey = os.LookupEnv(prefix + "_API_KEY")
	secret, hasSecret = os.LookupEnv(prefix + "_API_SECRET")
	ok = hasKey && hasSecret && os.Getenv("TEST_"+prefix) == "1"
	if ok {
		t.Logf(prefix+" api integration test enabled, key = %s, secret = %s", maskSecret(key), maskSecret(secret))
	}

	return key, secret, ok
}

// APIKeyConfigured gates an integration test for a service that authenticates
// with a key alone.
//
// IntegrationTestConfigured requires both <PREFIX>_API_KEY and
// <PREFIX>_API_SECRET, which does not fit a service like AmberData whose
// authentication is a single static header with no secret to pair with it.
func APIKeyConfigured(t *testing.T, prefix string) (key string, ok bool) {
	prefix = strings.ToUpper(prefix)

	var hasKey bool
	key, hasKey = os.LookupEnv(prefix + "_API_KEY")
	ok = hasKey && os.Getenv("TEST_"+prefix) == "1"
	if ok {
		t.Logf("%s api integration test enabled, key = %s", prefix, maskSecret(key))
	}

	return key, ok
}

func IntegrationTestWithPassphraseConfigured(t *testing.T, prefix string) (key, secret, passphrase string, ok bool) {
	var hasKey, hasSecret, hasPassphrase bool
	prefix = strings.ToUpper(prefix)
	key, hasKey = os.LookupEnv(prefix + "_API_KEY")
	secret, hasSecret = os.LookupEnv(prefix + "_API_SECRET")
	passphrase, hasPassphrase = os.LookupEnv(prefix + "_API_PASSPHRASE")
	ok = hasKey && hasSecret && hasPassphrase && os.Getenv("TEST_"+prefix) == "1"
	if ok {
		t.Logf("%s api integration test enabled, key = %s, secret = %s, passphrase= %s", prefix, maskSecret(key), maskSecret(secret), maskSecret(passphrase))
	} else {
		t.Skipf("%s api key is not configured, skipping integration test", prefix)
	}

	return key, secret, passphrase, ok
}
