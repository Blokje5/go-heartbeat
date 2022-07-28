package heartbeat

import "errors"

// Lifecycle Errors
var (
	ErrFailedTransition = errors.New("failed lifecycle transition")
	ErrAlreadyStarted = errors.New("already started")
	ErrAlreadyStopped = errors.New("already stopped")
)
