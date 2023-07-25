package httptesting

import (
	"net/http"
	"strings"

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
