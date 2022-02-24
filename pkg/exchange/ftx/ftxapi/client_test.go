package ftxapi

import (
	"context"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func maskSecret(s string) string {
	re := regexp.MustCompile(`\b(\w{4})\w+\b`)
	s = re.ReplaceAllString(s, "$1******")
	return s
}

func integrationTestConfigured(t *testing.T) (key, secret string, ok bool) {
	var hasKey, hasSecret bool
	key, hasKey = os.LookupEnv("FTX_API_KEY")
	secret, hasSecret = os.LookupEnv("FTX_API_SECRET")
	ok = hasKey && hasSecret && os.Getenv("TEST_FTX") == "1"
	if ok {
		t.Logf("ftx api integration test enabled, key = %s, secret = %s", maskSecret(key), maskSecret(secret))
	}
	return key, secret, ok
}

func TestClient_NewGetAccountRequest(t *testing.T) {
	key, secret, ok := integrationTestConfigured(t)
	if !ok {
		t.SkipNow()
		return
	}

	ctx, cancel := context.WithTimeout(context.TODO(), 15 * time.Second)
	defer cancel()

	client := NewClient()
	client.Auth(key, secret, "")
	req := client.NewGetAccountRequest()
	account ,err := req.Do(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, account)
	t.Logf("account: %+v", account)
}
