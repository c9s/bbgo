package httptesting

import (
	"encoding/json"
	"net/http"
	"os"
)

// Simplied client for testing that doesn't require multiple URLs

type EchoSave struct {
	// saveTo provides a way for tests to verify http.Request fields.

	// An http.Client's transport layer has only one method, so there's no way to
	// return variables while adhering to it's interface.  One solution is to use
	// type casting where the caller must know the transport layer is actually
	// of type "EchoSave".  But a cleaner approach is to pass in the address of
	// a local variable, and store the http.Request there.

	// Callers provide the address of a local variable, which is stored here.
	saveTo  **http.Request
	content string
	err     error
}

func (st *EchoSave) RoundTrip(req *http.Request) (*http.Response, error) {
	if st.saveTo != nil {
		// If the caller provided a local variable, update it with the latest http.Request
		*st.saveTo = req
	}
	resp := BuildResponseString(http.StatusOK, st.content)
	SetHeader(resp, "Content-Type", "application/json")
	return resp, st.err
}

func HttpClientFromFile(filename string) *http.Client {
	rawBytes, err := os.ReadFile(filename)
	transport := EchoSave{err: err, content: string(rawBytes)}
	return &http.Client{Transport: &transport}
}

func HttpClientWithContent(content string) *http.Client {
	transport := EchoSave{content: content}
	return &http.Client{Transport: &transport}
}

func HttpClientWithError(err error) *http.Client {
	transport := EchoSave{err: err}
	return &http.Client{Transport: &transport}
}

func HttpClientWithJson(jsonData interface{}) *http.Client {
	jsonBytes, err := json.Marshal(jsonData)
	transport := EchoSave{err: err, content: string(jsonBytes)}
	return &http.Client{Transport: &transport}
}

// "Saver" refers to saving the *http.Request in a local variable provided by the caller.
func HttpClientSaver(saved **http.Request, content string) *http.Client {
	transport := EchoSave{saveTo: saved, content: content}
	return &http.Client{Transport: &transport}
}

func HttpClientSaverWithJson(saved **http.Request, jsonData interface{}) *http.Client {
	jsonBytes, err := json.Marshal(jsonData)
	transport := EchoSave{saveTo: saved, err: err, content: string(jsonBytes)}
	return &http.Client{Transport: &transport}
}
