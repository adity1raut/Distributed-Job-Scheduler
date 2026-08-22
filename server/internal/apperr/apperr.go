// Package apperr defines the structured error type every handler returns,
// so the HTTP layer always has an HTTP status and a stable machine-readable
// code to render, regardless of which layer raised the error.
package apperr

import "net/http"

type Error struct {
	Status  int    `json:"-"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return e.Message }

func New(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

func BadRequest(message string) *Error {
	return New(http.StatusBadRequest, "BAD_REQUEST", message)
}

func Unauthorized(message string) *Error {
	return New(http.StatusUnauthorized, "UNAUTHORIZED", message)
}

func Forbidden(message string) *Error {
	return New(http.StatusForbidden, "FORBIDDEN", message)
}

func NotFound(resource string) *Error {
	return New(http.StatusNotFound, "NOT_FOUND", resource+" not found")
}

func Conflict(message string) *Error {
	return New(http.StatusConflict, "CONFLICT", message)
}

func TooManyRequests(message string) *Error {
	return New(http.StatusTooManyRequests, "RATE_LIMITED", message)
}

func Internal(message string) *Error {
	return New(http.StatusInternalServerError, "INTERNAL", message)
}

// As unwraps err into an *Error, falling back to a generic 500 so callers
// never have to nil-check before reading a status code.
func As(err error) *Error {
	if appErr, ok := err.(*Error); ok {
		return appErr
	}
	return Internal(err.Error())
}
