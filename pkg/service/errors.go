package service

import "github.com/pkg/errors"

var ErrPersistenceNotExists = errors.New("persistent data does not exists")
