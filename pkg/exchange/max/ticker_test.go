package max

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/testing/httptesting"
	"github.com/c9s/bbgo/pkg/testutil"
)

func TestExchange_QueryTickers_AllSymbols(t *testing.T) {
	key, secret, ok := testutil.IntegrationTestConfigured(t, "MAX")
	if !ok {
		t.SkipNow()
	}

	e := New(key, secret, "")

	isRecording, saveRecord := httptesting.RunHttpTestWithRecorder(t, e.v3client.HttpClient, "testdata/"+t.Name()+".json")
	defer saveRecord()

	if isRecording && !ok {
		t.Skipf("MAX api key is not configured, skipping integration test")
	}

	got, err := e.QueryTickers(context.Background())
	if assert.NoError(t, err) {
		assert.True(t, len(got) > 1, "max: attempting to get all symbol tickers, but get 1 or less")

		t.Logf("tickers: %+v", got)
	}
}

func TestExchange_QueryTickers_SomeSymbols(t *testing.T) {
	key, secret, ok := testutil.IntegrationTestConfigured(t, "MAX")
	if !ok {
		t.Skipf("api key/secret are not configured")
	}

	e := New(key, secret, "")

	isRecording, saveRecord := httptesting.RunHttpTestWithRecorder(t, e.v3client.HttpClient, "testdata/"+t.Name()+".json")
	defer saveRecord()

	if isRecording && !ok {
		t.Skipf("MAX api key is not configured, skipping integration test")
	}

	got, err := e.QueryTickers(context.Background(), "BTCUSDT", "ETHUSDT")
	if assert.NoError(t, err) {
		assert.Len(t, got, 2, "max: attempting to get two symbols, but number of tickers do not match")
	}
}

func TestExchange_QueryTickers_SingleSymbol(t *testing.T) {
	key, secret, ok := testutil.IntegrationTestConfigured(t, "MAX")
	if !ok {
		t.Skipf("api key/secret are not configured")
	}

	e := New(key, secret, "")

	isRecording, saveRecord := httptesting.RunHttpTestWithRecorder(t, e.v3client.HttpClient, "testdata/"+t.Name()+".json")
	defer saveRecord()

	if isRecording && !ok {
		t.Skipf("MAX api key is not configured, skipping integration test")
	}

	got, err := e.QueryTickers(context.Background(), "BTCUSDT")
	if assert.NoError(t, err) {
		assert.Len(t, got, 1, "max: attempting to get 1 symbols, but number of tickers do not match")
	}
}
