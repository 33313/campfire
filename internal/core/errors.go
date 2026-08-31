// Package core is the application. Every operation lives here exactly once,
// behind one permission check and one validation, and both the HTTP API and
// the MCP server dispatch into it.
//
// That is a requirement rather than a convenience (SPEC.md §14.1): a second
// implementation of authorisation or validation would be a second set of
// rules to keep in step, and the permission bug would live on the interface
// fewer people look at.
package core

import "fmt"

// Code classifies an error so that each interface can render it in its own
// terms — an HTTP status, an MCP tool error — without either one deciding
// what went wrong.
type Code string

const (
	CodeInvalid         Code = "invalid"
	CodeUnauthenticated Code = "unauthenticated"
	CodeForbidden       Code = "forbidden"
	CodeNotFound        Code = "not_found"
	CodeConflict        Code = "conflict"
	CodeRateLimited     Code = "rate_limited"
	CodeTooFarBehind    Code = "too_far_behind"
)

type Error struct {
	Code    Code   `json:"code"`
	Message string `json:"message"`
	// Field names the input that was rejected, when one input was.
	Field string `json:"field,omitempty"`
}

func (e *Error) Error() string { return string(e.Code) + ": " + e.Message }

func errf(code Code, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

func invalidField(field, format string, args ...any) *Error {
	return &Error{Code: CodeInvalid, Message: fmt.Sprintf(format, args...), Field: field}
}

var (
	errForbidden = &Error{Code: CodeForbidden, Message: "you may not do that"}
	errNotFound  = &Error{Code: CodeNotFound, Message: "not found"}
)
