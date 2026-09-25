// Package service holds business logic and status-transition validation.
// Handlers stay thin; the repository only runs queries.
package service

import "fmt"

// Error is a domain error with an HTTP status mapping.
type Error struct {
	Status  int
	Code    string
	Message string
	Details any
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

// BadRequest builds a 400 validation error.
func BadRequest(code, message string, details any) *Error {
	return &Error{Status: 400, Code: code, Message: message, Details: details}
}

// Unauthorized builds a 401 error.
func Unauthorized(code, message string) *Error {
	return &Error{Status: 401, Code: code, Message: message}
}

// Forbidden builds a 403 error.
func Forbidden(code, message string) *Error {
	return &Error{Status: 403, Code: code, Message: message}
}

// NotFound builds a 404 error.
func NotFound(resource string) *Error {
	return &Error{Status: 404, Code: "NOT_FOUND", Message: resource + " tidak ditemukan."}
}

// Conflict builds a 409 error.
func Conflict(code, message string, details any) *Error {
	return &Error{Status: 409, Code: code, Message: message, Details: details}
}
