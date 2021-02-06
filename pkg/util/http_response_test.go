package util

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResponse_DecodeJSON(t *testing.T) {
	type temp struct {
		Name string `json:"name"`
	}
	json := `{"name":"Test Name","a":"a"}`
	reader := ioutil.NopCloser(bytes.NewReader([]byte(json)))
	resp, err := NewResponse(&http.Response{
		StatusCode: 200,
		Body:       reader,
	})
	assert.NoError(t, err)
	assert.Equal(t, json, resp.String())

	var result temp
	assert.NoError(t, resp.DecodeJSON(&result))
	assert.Equal(t, "Test Name", result.Name)
}

func TestResponse_IsError(t *testing.T) {
	resp := &Response{Response: &http.Response{}}
	cases := map[int]bool{
		100: false,
		200: false,
		300: false,
		400: true,
		500: true,
	}

	for code, isErr := range cases {
		resp.StatusCode = code
		assert.Equal(t, isErr, resp.IsError())
	}
}

func TestResponse_IsJSON(t *testing.T) {
	cases := map[string]bool{
		"text/json":                       true,
		"application/json":                true,
		"application/json; charset=utf-8": true,
		"text/html":                       false,
	}
	for k, v := range cases {
		resp := &Response{Response: &http.Response{}}
		resp.Header = http.Header{}
		resp.Header.Set("content-type", k)
		assert.Equal(t, v, resp.IsJSON())
	}
}

func TestResponse_IsHTML(t *testing.T) {
	cases := map[string]bool{
		"text/json":                       false,
		"application/json":                false,
		"application/json; charset=utf-8": false,
		"text/html":                       true,
	}
	for k, v := range cases {
		resp := &Response{Response: &http.Response{}}
		resp.Header = http.Header{}
		resp.Header.Set("content-type", k)
		assert.Equal(t, v, resp.IsHTML())
	}
}
