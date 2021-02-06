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
