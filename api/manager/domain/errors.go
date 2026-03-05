package domain

import "errors"

var (
	ErrNotFound      = errors.New("not found")
	ErrNoKubeConfig  = errors.New("kubernetes configuration not provided")
	ErrNilQueryInput = errors.New("query options is nil")
	ErrNoClient      = errors.New("kubernetes client is not initialized")
)
