// Package output provides JSON envelope types and writers for schwab-agent responses.
package output

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/major/schwab-agent/internal/apperr"
)

// Metadata holds the standard metadata fields for response envelopes.
type Metadata struct {
	Timestamp string `json:"timestamp"`
	Account   string `json:"account,omitempty"`
	Requested int    `json:"requested,omitempty"`
	Returned  int    `json:"returned,omitempty"`
}

// NewMetadata returns metadata pre-populated with the current UTC timestamp.
func NewMetadata() Metadata {
	return Metadata{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

// TimestampMeta returns metadata with the standard UTC timestamp field set.
// It exists as the success-envelope helper used by command packages; NewMetadata
// remains the lower-level constructor for callers that add more metadata fields.
func TimestampMeta() Metadata {
	return NewMetadata()
}

// Envelope is the standard JSON response wrapper for successful operations.
type Envelope struct {
	Data     any      `json:"data"`
	Errors   []string `json:"errors,omitempty"`
	Metadata Metadata `json:"metadata"`
}

// ErrorEnvelope is the standard JSON response wrapper for error responses.
type ErrorEnvelope struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail contains error code, message, and optional details.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// WriteSuccess writes a successful response with data and metadata to the writer.
// The response is formatted as a JSON envelope with data and metadata fields.
func WriteSuccess(w io.Writer, data any, metadata Metadata) error {
	envelope := Envelope{
		Data:     data,
		Metadata: metadata,
	}
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(envelope)
}

// WriteError writes an error response to the writer.
// It maps the error type to an appropriate error code string using apperr.ErrorCode().
// The response is formatted as a JSON envelope with error details.
func WriteError(w io.Writer, err error) error {
	code := apperr.ErrorCode(err)
	message := err.Error()

	// Extract details from specific error types
	var details string
	if schwabErr, ok := errors.AsType[*apperr.SchwabError](err); ok {
		details = schwabErr.Details()
	}

	// Include the upstream response body when available so API errors from
	// Schwab are visible in the CLI output instead of just "status: NNN".
	if httpErr, ok := errors.AsType[*apperr.HTTPError](err); ok {
		if httpErr.Body != "" {
			details = fmt.Sprintf("status: %d, body: %s", httpErr.StatusCode, httpErr.Body)
		} else {
			details = fmt.Sprintf("status: %d", httpErr.StatusCode)
		}
	}

	errorEnvelope := ErrorEnvelope{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
			Details: details,
		},
	}
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(errorEnvelope)
}

// WritePartial writes a response with both data and errors (partial success).
// The response is formatted as a JSON envelope with data, errors, and metadata fields.
func WritePartial(w io.Writer, data any, errs []string, metadata Metadata) error {
	envelope := Envelope{
		Data:     data,
		Errors:   errs,
		Metadata: metadata,
	}
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(envelope)
}
