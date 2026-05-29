package service

import "errors"

var (
	ErrMonitorNotFound = errors.New("monitor not found")
	ErrInvalidJSON     = errors.New("invalid JSON")
)
