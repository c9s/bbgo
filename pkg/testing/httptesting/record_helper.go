package httptesting

import (
	"net/http"
	"os"
	"testing"
)

var AlwaysRecord = false
var RecordIfFileNotFound = false

// RunHttpTestWithRecorder sets up HTTP recording or playback for integration tests.
// It configures the provided http.Client to either record live HTTP requests to a file,
// or replay them from a file for deterministic testing. The function returns a boolean
// indicating whether recording is enabled, and a cleanup function to save recordings.
//
// Usage example:
//
//	// Enable recording for this test case
//	httptesting.AlwaysRecord = true
//	isRecording, saveRecord := httptesting.RunHttpTestWithRecorder(t, client.HttpClient, "testdata/record.json")
//	defer saveRecord()
//
//	// ... run your test logic ...
//
// Environment variable settings:
//
//	TEST_HTTP_RECORD=1   # enable recording mode
//	TEST_HTTP_LIVE=1     # enable live mode (no recording)
//
// The variable httptesting.AlwaysRecord can also be set directly in your test case to force recording mode.
// If recording is enabled, HTTP requests are captured and saved to the specified file.
// If not, requests are replayed from the file for fast, repeatable tests.
func RunHttpTestWithRecorder(t *testing.T, client *http.Client, recordFile string) (bool, func()) {
	mockTransport := &MockTransport{}
	recorder := NewRecorder(http.DefaultTransport)

	_, fErr := os.Stat(recordFile)
	notFound := fErr != nil && os.IsNotExist(fErr)
	shouldRecord := RecordIfFileNotFound && notFound

	isLive := os.Getenv("TEST_HTTP_LIVE") == "1"

	if os.Getenv("TEST_HTTP_RECORD") == "1" || isLive || shouldRecord || AlwaysRecord {
		client.Transport = recorder

		if isLive {
			return true, func() {}
		}

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
