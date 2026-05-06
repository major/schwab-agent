package commands

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	schwab "github.com/major/schwab-go/schwab"

	"github.com/major/schwab-agent/internal/apperr"
)

func TestMapSchwabGoError(t *testing.T) {
	t.Parallel()

	plainErr := errors.New("plain error")

	tests := []struct {
		name         string
		input        error
		expectedErr  error
		expectedType any
	}{
		{
			name:        "nil returns nil",
			input:       nil,
			expectedErr: nil,
		},
		{
			name: "401 maps to auth expired",
			input: fmt.Errorf("wrap: %w", &schwab.APIError{
				StatusCode: 401,
				Message:    "token expired",
				Body:       "unauthorized",
			}),
			expectedType: &apperr.AuthExpiredError{},
		},
		{
			name: "404 maps to http error",
			input: &schwab.APIError{
				StatusCode: 404,
				Message:    "not found",
				Body:       "missing",
			},
			expectedType: &apperr.HTTPError{},
		},
		{
			name: "500 maps to http error",
			input: fmt.Errorf("wrap: %w", &schwab.APIError{
				StatusCode: 500,
				Message:    "server exploded",
				Body:       "boom",
			}),
			expectedType: &apperr.HTTPError{},
		},
		{
			name:        "non api error passes through",
			input:       plainErr,
			expectedErr: plainErr,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := mapSchwabGoError(tc.input)

			switch {
			case tc.expectedErr == nil && tc.expectedType == nil:
				require.NoError(t, got)
			case tc.expectedErr != nil:
				require.Same(t, tc.expectedErr, got)
			case tc.expectedType != nil:
				switch tc.expectedType.(type) {
				case *apperr.AuthExpiredError:
					var authErr *apperr.AuthExpiredError
					require.ErrorAs(t, got, &authErr)
					assert.Equal(t, "token expired", authErr.Message)
					require.NoError(t, authErr.Cause)
				case *apperr.HTTPError:
					var httpErr *apperr.HTTPError
					require.ErrorAs(t, got, &httpErr)
					var apiErr *schwab.APIError
					if errors.As(tc.input, &apiErr) {
						assert.Equal(t, apiErr.StatusCode, httpErr.StatusCode)
						assert.Equal(t, apiErr.Body, httpErr.Body)
						assert.Equal(t, fmt.Sprintf("HTTP %d", apiErr.StatusCode), httpErr.Message)
					}
				}
			}
		})
	}
}
