package httptesting

import (
	"net/http"
	"os"
	"testing"
)

var AlwaysRecord = false
var RecordIfFileNotFound = false

func RunHttpTestWithRecorder(t *testing.T, client *http.Client, recordFile string) (bool, func()) {
	mockTransport := &MockTransport{}
	recorder := NewRecorder(http.DefaultTransport)

	_, fErr := os.Stat(recordFile)
	notFound := fErr != nil && os.IsNotExist(fErr)
	shouldRecord := RecordIfFileNotFound && notFound

	if os.Getenv("TEST_HTTP_RECORD") == "1" || shouldRecord || AlwaysRecord {
		client.Transport = recorder
		return true, func() {
			if err := recorder.Save(recordFile); err != nil {
				t.Errorf("failed to save recorded requests: %v", err)
			}
		}
	} else {
		if err := recorder.Load(recordFile); err != nil {
			t.Fatalf("failed to load recorded requests: %v", err)
		}

		if err := mockTransport.LoadFromRecorder(recorder); err != nil {
			t.Fatalf("failed to load recordings: %v", err)
		}

		client.Transport = mockTransport
		return false, func() {}
	}
}
