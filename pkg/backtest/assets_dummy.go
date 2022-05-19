//go:build !web
// +build !web

package backtest

import (
	"bytes"
	"errors"
	"net/http"
	"os"
	"time"
)

var assets = map[string][]byte{}

var FS = &fs{}

type fs struct{}

func (fs *fs) Open(name string) (http.File, error) {
	if name == "/" {
		return fs, nil
	}
	b, ok := assets[name]
	if !ok {
		return nil, os.ErrNotExist
	}
	return &file{name: name, size: len(b), Reader: bytes.NewReader(b)}, nil
}

func (fs *fs) Close() error                                 { return nil }
func (fs *fs) Read(p []byte) (int, error)                   { return 0, nil }
func (fs *fs) Seek(offset int64, whence int) (int64, error) { return 0, nil }
func (fs *fs) Stat() (os.FileInfo, error)                   { return fs, nil }
func (fs *fs) Name() string                                 { return "/" }
func (fs *fs) Size() int64                                  { return 0 }
func (fs *fs) Mode() os.FileMode                            { return 0755 }
func (fs *fs) ModTime() time.Time                           { return time.Time{} }
func (fs *fs) IsDir() bool                                  { return true }
func (fs *fs) Sys() interface{}                             { return nil }
func (fs *fs) Readdir(count int) ([]os.FileInfo, error) {
	files := []os.FileInfo{}
	for name, data := range assets {
		files = append(files, &file{name: name, size: len(data), Reader: bytes.NewReader(data)})
	}
	return files, nil
}

type file struct {
	name string
	size int
	*bytes.Reader
}

func (f *file) Close() error { return nil }
func (f *file) Readdir(count int) ([]os.FileInfo, error) {
	return nil, errors.New("readdir is not supported")
}
func (f *file) Stat() (os.FileInfo, error) { return f, nil }
func (f *file) Name() string               { return f.name }
func (f *file) Size() int64                { return int64(f.size) }
func (f *file) Mode() os.FileMode          { return 0644 }
func (f *file) ModTime() time.Time         { return time.Time{} }
func (f *file) IsDir() bool                { return false }
func (f *file) Sys() interface{}           { return nil }
