package httptesting

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/errors"
)

type RoundTripFunc func(req *http.Request) (*http.Response, error)

type MockTransport struct {
	getHandlers    map[string]RoundTripFunc
	postHandlers   map[string]RoundTripFunc
	deleteHandlers map[string]RoundTripFunc
	putHandlers    map[string]RoundTripFunc
}

func (transport *MockTransport) GET(path string, f RoundTripFunc) {
	if transport.getHandlers == nil {
		transport.getHandlers = make(map[string]RoundTripFunc)
	}

	transport.getHandlers[path] = f
}

func (transport *MockTransport) POST(path string, f RoundTripFunc) {
	if transport.postHandlers == nil {
		transport.postHandlers = make(map[string]RoundTripFunc)
	}

	transport.postHandlers[path] = f
}

func (transport *MockTransport) DELETE(path string, f RoundTripFunc) {
	if transport.deleteHandlers == nil {
		transport.deleteHandlers = make(map[string]RoundTripFunc)
	}

	transport.deleteHandlers[path] = f
}

func (transport *MockTransport) PUT(path string, f RoundTripFunc) {
	if transport.putHandlers == nil {
		transport.putHandlers = make(map[string]RoundTripFunc)
	}

	transport.putHandlers[path] = f
}

// Used for migration to MAX v3 api, where order cancel uses DELETE (MAX v2 api uses POST).
func (transport *MockTransport) PostOrDelete(isDelete bool, path string, f RoundTripFunc) {
	if isDelete {
		transport.DELETE(path, f)
	} else {
		transport.POST(path, f)
	}
}

func (transport *MockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var handlers map[string]RoundTripFunc

	switch strings.ToUpper(req.Method) {

	case "GET":
		handlers = transport.getHandlers
	case "POST":
		handlers = transport.postHandlers
	case "DELETE":
		handlers = transport.deleteHandlers
	case "PUT":
		handlers = transport.putHandlers

	default:
		return nil, errors.Errorf("unsupported mock transport request method: %s", req.Method)

	}

	f, ok := handlers[req.URL.Path]
	if !ok {
		return nil, errors.Errorf("roundtrip mock to %s %s is not defined", req.Method, req.URL.Path)
	}

	return f(req)
}

func MockWithJsonReply(url string, rawData interface{}) *http.Client {
	tripFunc := func(_ *http.Request) (*http.Response, error) {
		return BuildResponseJson(http.StatusOK, rawData), nil
	}

	transport := &MockTransport{}
	transport.DELETE(url, tripFunc)
	transport.GET(url, tripFunc)
	transport.POST(url, tripFunc)
	transport.PUT(url, tripFunc)
	return &http.Client{Transport: transport}
}

// RecorderEntry records a single request and response pair.
// It includes basic request and response information,
// and is used for (de)serialization to and from files.
type RecorderEntry struct {
	Timestamp time.Time       `json:"timestamp"`
	Request   *RequestRecord  `json:"request"`
	Response  *ResponseRecord `json:"response"`
	Error     string          `json:"error,omitempty"`
}

type RequestRecord struct {
	Method string      `json:"method"`
	URL    string      `json:"url"`
	Header http.Header `json:"header"`
	Body   string      `json:"body,omitempty"`
}

type ResponseRecord struct {
	Status     string      `json:"status"`
	StatusCode int         `json:"status_code"`
	Header     http.Header `json:"header"`
	Body       string      `json:"body,omitempty"`
}

// Recorder is used to record request and response pairs to files.
// It can also playback the recorded pairs by registering handlers
// to a MockTransport.
type Recorder struct {
	entries   []RecorderEntry
	dir       string
	prefix    string
	transport http.RoundTripper // underlying transport for http.RoundTripper
}

// NewRecorder creates a new Recorder instance with the given transport.
// transport is the underlying http.RoundTripper used for actual HTTP requests.
func NewRecorder(transport http.RoundTripper) *Recorder {
	return &Recorder{
		transport: transport,
	}
}

// filterCredentials removes sensitive credentials from request headers using regexp.
func filterCredentials(header http.Header) {
	patterns := []string{
		`(?i)^authorization$`,
		`(?i)^api[-_]key$`,
		`(?i)^x[-_]api[-_]key$`,
		`(?i)^cookie$`,
		`(?i)^access[-_]token$`,
		`(?i)^secret$`,
	}
	var regexps []*regexp.Regexp
	for _, p := range patterns {
		regexps = append(regexps, regexp.MustCompile(p))
	}
	for key := range header {
		for _, re := range regexps {
			if re.MatchString(key) {
				header.Del(key)
				break
			}
		}
	}
}

// cloneRequest returns a deep copy of the given http.Request, including Header and Body.
func cloneRequest(req *http.Request) *http.Request {
	cloned := req.Clone(req.Context())
	cloned.Header = req.Header.Clone()
	if req.Body != nil {
		bodyBytes, _ := io.ReadAll(req.Body)
		cloned.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))
		// restore original body for caller
		req.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))
	}
	return cloned
}

// RecordEntry records a request and response, and automatically filters sensitive credentials from the request header.
func (r *Recorder) RecordEntry(req *http.Request, resp *http.Response, err error) {
	clonedReq := cloneRequest(req)
	entry := RecorderEntry{
		Timestamp: time.Now(),
		Request: &RequestRecord{
			Method: clonedReq.Method,
			URL:    clonedReq.URL.String(),
			Header: clonedReq.Header,
		},
	}
	filterCredentials(entry.Request.Header)
	if clonedReq.Body != nil {
		bodyBytes, _ := io.ReadAll(clonedReq.Body)
		entry.Request.Body = string(bodyBytes)
		clonedReq.Body = io.NopCloser(strings.NewReader(entry.Request.Body))
	}
	if resp != nil {
		entry.Response = &ResponseRecord{
			Status:     resp.Status,
			StatusCode: resp.StatusCode,
			Header:     resp.Header.Clone(),
		}
		if resp.Body != nil {
			bodyBytes, _ := io.ReadAll(resp.Body)
			entry.Response.Body = string(bodyBytes)
			resp.Body = io.NopCloser(strings.NewReader(entry.Response.Body))
		}
	}
	if err != nil {
		entry.Error = err.Error()
	}
	r.entries = append(r.entries, entry)
}

// RoundTripFunc returns a RoundTripFunc that records request and response using the Recorder.
// f is the original RoundTripFunc to be called.
func (r *Recorder) RoundTripFunc(f RoundTripFunc) RoundTripFunc {
	return func(req *http.Request) (*http.Response, error) {
		resp, err := f(req)
		r.RecordEntry(req, resp, err)
		return resp, err
	}
}

// Save saves the recorded entries to a JSON file.
func (r *Recorder) Save(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}

	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(r.entries)
}

// Load loads recorded entries from a JSON file.
func (r *Recorder) Load(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	var entries []RecorderEntry
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&entries); err != nil {
		return err
	}
	r.entries = entries
	return nil
}

// BuildResponseFromRecord creates an http.Response from a ResponseRecord.
func BuildResponseFromRecord(respRec *ResponseRecord) *http.Response {
	resp := &http.Response{
		Status:     respRec.Status,
		StatusCode: respRec.StatusCode,
		Header:     respRec.Header.Clone(),
		Body:       io.NopCloser(strings.NewReader(respRec.Body)),
	}
	return resp
}

// LoadFromRecorder loads request/response records from file and registers handlers to MockTransport.
// Each handler will respond with the recorded response for the matching method and path.
func (transport *MockTransport) LoadFromRecorder(recorder *Recorder) error {
	for _, entry := range recorder.entries {
		if entry.Request == nil || entry.Response == nil {
			continue
		}

		u, err := url.Parse(entry.Request.URL)
		if err != nil {
			return err
		}

		path := u.Path
		method := strings.ToUpper(entry.Request.Method)
		handler := func(_ *http.Request) (*http.Response, error) {
			return BuildResponseFromRecord(entry.Response), nil
		}
		switch method {
		case "GET":
			transport.GET(path, handler)
		case "POST":
			transport.POST(path, handler)
		case "DELETE":
			transport.DELETE(path, handler)
		case "PUT":
			transport.PUT(path, handler)
		}
	}
	return nil
}

// RoundTrip implements the http.RoundTripper interface for Recorder.
// It records the request and response, and returns the response from the underlying transport.
func (r *Recorder) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := r.transport.RoundTrip(req)
	r.RecordEntry(req, resp, err)
	return resp, err
}
