package testutil

import (
	"os"
	"regexp"
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
