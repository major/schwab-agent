// Package apperr defines the error hierarchy and exit code mapping for schwab-agent.
package apperr

import (
	"errors"
)

// ErrorOption configures optional fields on SchwabError during construction.
type ErrorOption func(*SchwabError)

// WithDetails sets a human-readable hint or remediation message on the error.
func WithDetails(d string) ErrorOption {
	return func(e *SchwabError) { e.details = d }
}

// SchwabError is the base error type for all schwab-agent errors.
// It wraps an underlying error to preserve the error chain.
type SchwabError struct {
	Message string
	Cause   error
	details string
}

// Details returns the human-readable hint or remediation message, if any.
func (e *SchwabError) Details() string {
	return e.details
}

// Error implements the error interface.
func (e *SchwabError) Error() string {
	return e.Message
}

// Unwrap returns the underlying error, enabling error chain traversal.
func (e *SchwabError) Unwrap() error {
	return e.Cause
}

// ExitCode returns the process exit code for this error type.
func (e *SchwabError) ExitCode() int {
	return 1
}

// AuthRequiredError indicates that authentication is required.
type AuthRequiredError struct {
	SchwabError
}

// NewAuthRequiredError creates a new AuthRequiredError wrapping the given cause.
func NewAuthRequiredError(message string, cause error, opts ...ErrorOption) *AuthRequiredError {
	e := &AuthRequiredError{
		SchwabError: SchwabError{
			Message: message,
			Cause:   cause,
		},
	}
	for _, o := range opts {
		o(&e.SchwabError)
	}
	return e
}

// ExitCode returns the process exit code for this error type.
func (e *AuthRequiredError) ExitCode() int {
	return 3
}

// AuthExpiredError indicates that authentication has expired.
type AuthExpiredError struct {
	SchwabError
}

// NewAuthExpiredError creates a new AuthExpiredError wrapping the given cause.
func NewAuthExpiredError(message string, cause error, opts ...ErrorOption) *AuthExpiredError {
	e := &AuthExpiredError{
		SchwabError: SchwabError{
			Message: message,
			Cause:   cause,
		},
	}
	for _, o := range opts {
		o(&e.SchwabError)
	}
	return e
}

// ExitCode returns the process exit code for this error type.
func (e *AuthExpiredError) ExitCode() int {
	return 3
}

// AuthCallbackError indicates that authentication callback failed.
type AuthCallbackError struct {
	SchwabError
}

// NewAuthCallbackError creates a new AuthCallbackError wrapping the given cause.
func NewAuthCallbackError(message string, cause error, opts ...ErrorOption) *AuthCallbackError {
	e := &AuthCallbackError{
		SchwabError: SchwabError{
			Message: message,
			Cause:   cause,
		},
	}
	for _, o := range opts {
		o(&e.SchwabError)
	}
	return e
}

// ExitCode returns the process exit code for this error type.
func (e *AuthCallbackError) ExitCode() int {
	return 3
}

// OrderRejectedError indicates that an order was rejected.
type OrderRejectedError struct {
	SchwabError
}

// NewOrderRejectedError creates a new OrderRejectedError wrapping the given cause.
func NewOrderRejectedError(message string, cause error, opts ...ErrorOption) *OrderRejectedError {
	e := &OrderRejectedError{
		SchwabError: SchwabError{
			Message: message,
			Cause:   cause,
		},
	}
	for _, o := range opts {
		o(&e.SchwabError)
	}
	return e
}

// ExitCode returns the process exit code for this error type.
func (e *OrderRejectedError) ExitCode() int {
	return 5
}

// SymbolNotFoundError indicates that a symbol was not found.
type SymbolNotFoundError struct {
	SchwabError
}

// NewSymbolNotFoundError creates a new SymbolNotFoundError wrapping the given cause.
func NewSymbolNotFoundError(message string, cause error, opts ...ErrorOption) *SymbolNotFoundError {
	e := &SymbolNotFoundError{
		SchwabError: SchwabError{
			Message: message,
			Cause:   cause,
		},
	}
	for _, o := range opts {
		o(&e.SchwabError)
	}
	return e
}

// ExitCode returns the process exit code for this error type.
func (e *SymbolNotFoundError) ExitCode() int {
	return 2
}

// AccountNotFoundError indicates that an account was not found.
type AccountNotFoundError struct {
	SchwabError
}

// NewAccountNotFoundError creates a new AccountNotFoundError wrapping the given cause.
func NewAccountNotFoundError(message string, cause error, opts ...ErrorOption) *AccountNotFoundError {
	e := &AccountNotFoundError{
		SchwabError: SchwabError{
			Message: message,
			Cause:   cause,
		},
	}
	for _, o := range opts {
		o(&e.SchwabError)
	}
	return e
}

// ExitCode returns the process exit code for this error type.
func (e *AccountNotFoundError) ExitCode() int {
	return 2
}

// HTTPError indicates that an HTTP request returned a non-2xx error status code.
type HTTPError struct {
	SchwabError
	StatusCode int
	Body       string
}

// NewHTTPError creates a new HTTPError wrapping the given cause.
func NewHTTPError(message string, statusCode int, body string, cause error, opts ...ErrorOption) *HTTPError {
	e := &HTTPError{
		SchwabError: SchwabError{
			Message: message,
			Cause:   cause,
		},
		StatusCode: statusCode,
		Body:       body,
	}
	for _, o := range opts {
		o(&e.SchwabError)
	}
	return e
}

// ExitCode returns the process exit code for this error type.
func (e *HTTPError) ExitCode() int {
	return 4
}

// ValidationError indicates that input validation failed.
type ValidationError struct {
	SchwabError
}

// NewValidationError creates a new ValidationError wrapping the given cause.
func NewValidationError(message string, cause error, opts ...ErrorOption) *ValidationError {
	e := &ValidationError{
		SchwabError: SchwabError{
			Message: message,
			Cause:   cause,
		},
	}
	for _, o := range opts {
		o(&e.SchwabError)
	}
	return e
}

// ExitCode returns the process exit code for this error type.
func (e *ValidationError) ExitCode() int {
	return 1
}

// OrderBuildError indicates that order building failed.
type OrderBuildError struct {
	SchwabError
}

// NewOrderBuildError creates a new OrderBuildError wrapping the given cause.
func NewOrderBuildError(message string, cause error, opts ...ErrorOption) *OrderBuildError {
	e := &OrderBuildError{
		SchwabError: SchwabError{
			Message: message,
			Cause:   cause,
		},
	}
	for _, o := range opts {
		o(&e.SchwabError)
	}
	return e
}

// ExitCode returns the process exit code for this error type.
func (e *OrderBuildError) ExitCode() int {
	return 1
}

// NewSchwabError creates a new SchwabError wrapping the given cause.
func NewSchwabError(message string, cause error, opts ...ErrorOption) *SchwabError {
	e := &SchwabError{
		Message: message,
		Cause:   cause,
	}
	for _, o := range opts {
		o(e)
	}
	return e
}

// exitCoder is an interface for types that can provide an exit code.
type exitCoder interface {
	error
	ExitCode() int
}

// ExitCodeFor determines the appropriate exit code for the given error.
// Returns 0 if err is nil, otherwise returns the appropriate exit code.
func ExitCodeFor(err error) int {
	if err == nil {
		return 0
	}

	if coder, ok := errors.AsType[exitCoder](err); ok {
		return coder.ExitCode()
	}

	return 1
}

// ErrorCode returns the error classification code for JSON responses.
func ErrorCode(err error) string {
	if _, ok := errors.AsType[*AuthRequiredError](err); ok {
		return "AUTH_REQUIRED"
	}

	if _, ok := errors.AsType[*AuthExpiredError](err); ok {
		return "AUTH_EXPIRED"
	}

	if _, ok := errors.AsType[*AuthCallbackError](err); ok {
		return "AUTH_CALLBACK"
	}

	if _, ok := errors.AsType[*OrderRejectedError](err); ok {
		return "ORDER_REJECTED"
	}

	if _, ok := errors.AsType[*SymbolNotFoundError](err); ok {
		return "SYMBOL_NOT_FOUND"
	}

	if _, ok := errors.AsType[*AccountNotFoundError](err); ok {
		return "ACCOUNT_NOT_FOUND"
	}

	if _, ok := errors.AsType[*HTTPError](err); ok {
		return "HTTP_ERROR"
	}

	if _, ok := errors.AsType[*ValidationError](err); ok {
		return "VALIDATION_ERROR"
	}

	if _, ok := errors.AsType[*OrderBuildError](err); ok {
		return "ORDER_BUILD_ERROR"
	}

	return "UNKNOWN"
}
