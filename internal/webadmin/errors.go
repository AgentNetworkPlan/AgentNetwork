package webadmin

import "errors"

var (
	// ErrInvalidToken indicates an invalid authentication token.
	ErrInvalidToken = errors.New("invalid token")

	// ErrSessionExpired indicates that the session has expired.
	ErrSessionExpired = errors.New("session expired")

	// ErrUnauthorized indicates unauthorized access.
	ErrUnauthorized = errors.New("unauthorized")

	// ErrServerNotRunning indicates the server is not running.
	ErrServerNotRunning = errors.New("server not running")
)
