package apperr

import (
	"errors"
	"fmt"
	"testing"

	structclierrors "github.com/leodido/structcli/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExitCodeForAuthRequiredError verifies AuthRequiredError maps to exit code 3.
func TestExitCodeForAuthRequiredError(t *testing.T) {
	err := NewAuthRequiredError("auth required", nil)
	code := ExitCodeFor(err)
	assert.Equal(t, 3, code)
}

// TestExitCodeForAuthExpiredError verifies AuthExpiredError maps to exit code 3.
func TestExitCodeForAuthExpiredError(t *testing.T) {
	err := NewAuthExpiredError("auth expired", nil)
	code := ExitCodeFor(err)
	assert.Equal(t, 3, code)
}

// TestExitCodeForAuthCallbackError verifies AuthCallbackError maps to exit code 3.
func TestExitCodeForAuthCallbackError(t *testing.T) {
	err := NewAuthCallbackError("auth callback failed", nil)
	code := ExitCodeFor(err)
	assert.Equal(t, 3, code)
}

// TestExitCodeForOrderRejectedError verifies OrderRejectedError maps to exit code 5.
func TestExitCodeForOrderRejectedError(t *testing.T) {
	err := NewOrderRejectedError("order rejected", nil)
	code := ExitCodeFor(err)
	assert.Equal(t, 5, code)
}

// TestExitCodeForSymbolNotFoundError verifies SymbolNotFoundError maps to exit code 2.
func TestExitCodeForSymbolNotFoundError(t *testing.T) {
	err := NewSymbolNotFoundError("symbol not found", nil)
	code := ExitCodeFor(err)
	assert.Equal(t, 2, code)
}

// TestExitCodeForAccountNotFoundError verifies AccountNotFoundError maps to exit code 2.
func TestExitCodeForAccountNotFoundError(t *testing.T) {
	err := NewAccountNotFoundError("account not found", nil)
	code := ExitCodeFor(err)
	assert.Equal(t, 2, code)
}

// TestExitCodeForHTTPError verifies HTTPError maps to exit code 4.
func TestExitCodeForHTTPError(t *testing.T) {
	err := NewHTTPError("http error", 500, "Internal Server Error", nil)
	code := ExitCodeFor(err)
	assert.Equal(t, 4, code)
}

// TestExitCodeForValidationError verifies ValidationError maps to exit code 1.
func TestExitCodeForValidationError(t *testing.T) {
	err := NewValidationError("validation failed", nil)
	code := ExitCodeFor(err)
	assert.Equal(t, 1, code)
}

// TestExitCodeForOrderBuildError verifies OrderBuildError maps to exit code 1.
func TestExitCodeForOrderBuildError(t *testing.T) {
	err := NewOrderBuildError("order build failed", nil)
	code := ExitCodeFor(err)
	assert.Equal(t, 1, code)
}

// TestExitCodeForSchwabError verifies SchwabError maps to exit code 1.
func TestExitCodeForSchwabError(t *testing.T) {
	err := NewSchwabError("generic error", nil)
	code := ExitCodeFor(err)
	assert.Equal(t, 1, code)
}

// TestExitCodeForNilError verifies nil error maps to exit code 0.
func TestExitCodeForNilError(t *testing.T) {
	code := ExitCodeFor(nil)
	assert.Equal(t, 0, code)
}

// TestExitCodeForStandardError verifies standard errors.New() maps to exit code 1.
func TestExitCodeForStandardError(t *testing.T) {
	err := errors.New("standard error")
	code := ExitCodeFor(err)
	assert.Equal(t, 1, code)
}

// TestErrorCodeForAuthRequiredError verifies ErrorCode returns correct string.
func TestErrorCodeForAuthRequiredError(t *testing.T) {
	err := NewAuthRequiredError("auth required", nil)
	code := ErrorCode(err)
	assert.Equal(t, "AUTH_REQUIRED", code)
}

// TestErrorCodeForAuthExpiredError verifies ErrorCode returns correct string.
func TestErrorCodeForAuthExpiredError(t *testing.T) {
	err := NewAuthExpiredError("auth expired", nil)
	code := ErrorCode(err)
	assert.Equal(t, "AUTH_EXPIRED", code)
}

// TestErrorCodeForAuthCallbackError verifies ErrorCode returns correct string.
func TestErrorCodeForAuthCallbackError(t *testing.T) {
	err := NewAuthCallbackError("auth callback failed", nil)
	code := ErrorCode(err)
	assert.Equal(t, "AUTH_CALLBACK", code)
}

// TestErrorCodeForOrderRejectedError verifies ErrorCode returns correct string.
func TestErrorCodeForOrderRejectedError(t *testing.T) {
	err := NewOrderRejectedError("order rejected", nil)
	code := ErrorCode(err)
	assert.Equal(t, "ORDER_REJECTED", code)
}

// TestErrorCodeForSymbolNotFoundError verifies ErrorCode returns correct string.
func TestErrorCodeForSymbolNotFoundError(t *testing.T) {
	err := NewSymbolNotFoundError("symbol not found", nil)
	code := ErrorCode(err)
	assert.Equal(t, "SYMBOL_NOT_FOUND", code)
}

// TestErrorCodeForAccountNotFoundError verifies ErrorCode returns correct string.
func TestErrorCodeForAccountNotFoundError(t *testing.T) {
	err := NewAccountNotFoundError("account not found", nil)
	code := ErrorCode(err)
	assert.Equal(t, "ACCOUNT_NOT_FOUND", code)
}

// TestErrorCodeForHTTPError verifies ErrorCode returns correct string.
func TestErrorCodeForHTTPError(t *testing.T) {
	err := NewHTTPError("http error", 500, "Internal Server Error", nil)
	code := ErrorCode(err)
	assert.Equal(t, "HTTP_ERROR", code)
}

// TestErrorCodeForValidationError verifies ErrorCode returns correct string.
func TestErrorCodeForValidationError(t *testing.T) {
	err := NewValidationError("validation failed", nil)
	code := ErrorCode(err)
	assert.Equal(t, "VALIDATION_ERROR", code)
}

// TestErrorCodeForOrderBuildError verifies ErrorCode returns correct string.
func TestErrorCodeForOrderBuildError(t *testing.T) {
	err := NewOrderBuildError("order build failed", nil)
	code := ErrorCode(err)
	assert.Equal(t, "ORDER_BUILD_ERROR", code)
}

// TestErrorCodeForStructcliValidationError verifies structcli's ValidationError
// (produced by the Validatable interface) maps to VALIDATION_ERROR, same as
// our internal ValidationError.
func TestErrorCodeForStructcliValidationError(t *testing.T) {
	err := &structclierrors.ValidationError{
		ContextName: "test-cmd",
		Errors:      []error{fmt.Errorf("field is invalid")},
	}
	code := ErrorCode(err)
	assert.Equal(t, "VALIDATION_ERROR", code)
}

// TestExitCodeForStructcliValidationError verifies structcli's ValidationError
// maps to exit code 1 (default for non-exitCoder errors).
func TestExitCodeForStructcliValidationError(t *testing.T) {
	err := &structclierrors.ValidationError{
		ContextName: "test-cmd",
		Errors:      []error{fmt.Errorf("field is invalid")},
	}
	code := ExitCodeFor(err)
	assert.Equal(t, 1, code)
}

// TestErrorCodeForSchwabError verifies ErrorCode returns correct string.
func TestErrorCodeForSchwabError(t *testing.T) {
	err := NewSchwabError("generic error", nil)
	code := ErrorCode(err)
	assert.Equal(t, "UNKNOWN", code)
}

// TestErrorCodeForStandardError verifies ErrorCode returns UNKNOWN for standard errors.
func TestErrorCodeForStandardError(t *testing.T) {
	err := errors.New("standard error")
	code := ErrorCode(err)
	assert.Equal(t, "UNKNOWN", code)
}

// TestErrorMessagePreservation verifies error messages are preserved.
func TestErrorMessagePreservation(t *testing.T) {
	message := "test error message"
	err := NewSchwabError(message, nil)
	assert.Equal(t, message, err.Error())
}

// TestErrorChainPreservation verifies error chains are preserved through wrapping.
func TestErrorChainPreservation(t *testing.T) {
	underlying := errors.New("root cause")
	err := NewAuthRequiredError("auth required", underlying)

	assert.ErrorIs(t, err, underlying)
	assert.ErrorIs(t, err.Unwrap(), underlying)
}

// TestErrorChainTraversal verifies errors.AsType() can traverse the error chain.
func TestErrorChainTraversal(t *testing.T) {
	underlying := errors.New("root cause")
	err := NewAuthRequiredError("auth required", underlying)

	authErr, ok := errors.AsType[*AuthRequiredError](err)
	require.True(t, ok, "errors.AsType() should find AuthRequiredError in chain")

	// Note: errors.AsType() does not automatically traverse to parent types
	// without explicit As() method implementation. This test verifies
	// that the specific error type can be found.
	assert.Equal(t, "auth required", authErr.Message)
}

// TestHTTPErrorAttributes verifies HTTPError stores status code and body.
func TestHTTPErrorAttributes(t *testing.T) {
	statusCode := 500
	body := "Internal Server Error"
	err := NewHTTPError("http error", statusCode, body, nil)
	assert.Equal(t, statusCode, err.StatusCode)
	assert.Equal(t, body, err.Body)
}

// TestDetailsFieldPreservation verifies Details is set through constructor option.
func TestDetailsFieldPreservation(t *testing.T) {
	details := "Try checking your credentials"
	err := NewAuthRequiredError("auth required", nil, WithDetails(details))
	assert.Equal(t, details, err.Details())
}

// TestWrappedAuthRequiredError verifies wrapped AuthRequiredError maps correctly.
func TestWrappedAuthRequiredError(t *testing.T) {
	underlying := errors.New("invalid token")
	err := NewAuthRequiredError("auth required", underlying)
	code := ExitCodeFor(err)
	assert.Equal(t, 3, code)
}

// TestWrappedOrderRejectedError verifies wrapped OrderRejectedError maps correctly.
func TestWrappedOrderRejectedError(t *testing.T) {
	underlying := errors.New("insufficient funds")
	err := NewOrderRejectedError("order rejected", underlying)
	code := ExitCodeFor(err)
	assert.Equal(t, 5, code)
}

// TestWrappedSymbolNotFoundError verifies wrapped SymbolNotFoundError maps correctly.
func TestWrappedSymbolNotFoundError(t *testing.T) {
	underlying := errors.New("api returned empty data")
	err := NewSymbolNotFoundError("symbol not found", underlying)
	code := ExitCodeFor(err)
	assert.Equal(t, 2, code)
}

// TestWrappedAccountNotFoundError verifies wrapped AccountNotFoundError maps correctly.
func TestWrappedAccountNotFoundError(t *testing.T) {
	underlying := errors.New("account lookup failed")
	err := NewAccountNotFoundError("account not found", underlying)
	code := ExitCodeFor(err)
	assert.Equal(t, 2, code)
}

// TestWrappedHTTPError verifies wrapped HTTPError maps correctly.
func TestWrappedHTTPError(t *testing.T) {
	underlying := errors.New("connection failed")
	err := NewHTTPError("http error", 502, "Bad Gateway", underlying)
	code := ExitCodeFor(err)
	assert.Equal(t, 4, code)
}

// TestWrappedValidationError verifies wrapped ValidationError maps correctly.
func TestWrappedValidationError(t *testing.T) {
	underlying := errors.New("symbol is empty")
	err := NewValidationError("validation failed", underlying)
	code := ExitCodeFor(err)
	assert.Equal(t, 1, code)
}

// TestWrappedOrderBuildError verifies wrapped OrderBuildError maps correctly.
func TestWrappedOrderBuildError(t *testing.T) {
	underlying := errors.New("invalid order parameters")
	err := NewOrderBuildError("order build failed", underlying)
	code := ExitCodeFor(err)
	assert.Equal(t, 1, code)
}

// TestWithDetails_AllConstructors verifies the WithDetails option sets the
// details field on every error constructor. The existing TestDetailsFieldPreservation
// covers AuthRequiredError; this table-driven test covers the remaining types.
func TestWithDetails_AllConstructors(t *testing.T) {
	const hint = "check your settings"

	tests := []struct {
		name    string
		details string
	}{
		{"AuthExpired", NewAuthExpiredError("expired", nil, WithDetails(hint)).Details()},
		{"AuthCallback", NewAuthCallbackError("callback", nil, WithDetails(hint)).Details()},
		{"OrderRejected", NewOrderRejectedError("rejected", nil, WithDetails(hint)).Details()},
		{"SymbolNotFound", NewSymbolNotFoundError("not found", nil, WithDetails(hint)).Details()},
		{"AccountNotFound", NewAccountNotFoundError("not found", nil, WithDetails(hint)).Details()},
		{"HTTPError", NewHTTPError("http", 500, "body", nil, WithDetails(hint)).Details()},
		{"Validation", NewValidationError("invalid", nil, WithDetails(hint)).Details()},
		{"OrderBuild", NewOrderBuildError("build", nil, WithDetails(hint)).Details()},
		{"SchwabError", NewSchwabError("generic", nil, WithDetails(hint)).Details()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, hint, tt.details)
		})
	}
}
