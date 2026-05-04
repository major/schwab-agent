package commands

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	structclierrors "github.com/leodido/structcli/errors"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewQuoteCmd(t *testing.T) {
	// Table-driven test covering both explicit ("quote get") and shorthand
	// ("quote") invocation paths for single and multi-symbol lookups.
	// Single-symbol responses are normalized to match multi-symbol shape
	// (keyed by symbol), so all cases use the same expectKeys assertion.
	tests := []struct {
		name       string
		serverPath string
		serverBody string
		args       []string
		expectKeys []string
	}{
		{
			name:       "explicit single symbol",
			serverPath: "/marketdata/v1/AAPL/quotes",
			serverBody: `{"AAPL":{"symbol":"AAPL","lastPrice":150.0}}`,
			args:       []string{"get", "AAPL"},
			expectKeys: []string{"AAPL"},
		},
		{
			name:       "explicit multiple symbols",
			serverPath: "/marketdata/v1/quotes",
			serverBody: `{"AAPL":{"symbol":"AAPL","lastPrice":150.0},"MSFT":{"symbol":"MSFT","lastPrice":400.0}}`,
			args:       []string{"get", "AAPL", "MSFT"},
			expectKeys: []string{"AAPL", "MSFT"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == tt.serverPath {
					_, _ = w.Write([]byte(tt.serverBody))
					return
				}
				w.WriteHeader(http.StatusNotFound)
			}))
			defer server.Close()

			var buf bytes.Buffer
			cmd := NewQuoteCmd(testClient(t, server), &buf)
			_, err := runTestCommand(t, cmd, tt.args...)
			require.NoError(t, err)

			var envelope output.Envelope
			require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
			data, ok := envelope.Data.(map[string]any)
			require.True(t, ok)

			for _, key := range tt.expectKeys {
				assert.Contains(t, data, key)
			}
			assert.Empty(t, envelope.Errors)
			assert.NotEmpty(t, envelope.Metadata.Timestamp)
		})
	}
}

func TestNewQuoteGetPartialSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/marketdata/v1/quotes" {
			// Only AAPL found; INVALID is absent from the response.
			_, _ = w.Write([]byte(`{"AAPL":{"symbol":"AAPL","lastPrice":150.0}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	var buf bytes.Buffer
	cmd := NewQuoteCmd(testClient(t, server), &buf)
	_, err := runTestCommand(t, cmd, "get", "AAPL", "INVALID")
	require.NoError(t, err)

	var envelope output.Envelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	assert.Contains(t, data, "AAPL")
	assert.NotContains(t, data, "INVALID")

	require.Len(t, envelope.Errors, 1)
	assert.Contains(t, envelope.Errors[0], "INVALID")

	assert.Equal(t, 2, envelope.Metadata.Requested)
	assert.Equal(t, 1, envelope.Metadata.Returned)
	assert.NotEmpty(t, envelope.Metadata.Timestamp)
}

func TestNewQuoteGetSingleNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))
	defer server.Close()

	var buf bytes.Buffer
	cmd := NewQuoteCmd(testClient(t, server), &buf)
	_, err := runTestCommand(t, cmd, "get", "INVALID")
	require.Error(t, err)

	var symErr *apperr.SymbolNotFoundError
	assert.ErrorAs(t, err, &symErr)
	assert.Empty(t, buf.String())
}

func TestNewQuoteGetNoArgs(t *testing.T) {
	server := jsonServer(`{}`)
	defer server.Close()

	var buf bytes.Buffer
	cmd := NewQuoteCmd(testClient(t, server), &buf)
	_, err := runTestCommand(t, cmd, "get")
	require.Error(t, err)

	// Cobra's MinimumNArgs(1) produces a usage error, not a ValidationError
	// The error message should indicate at least 1 argument is required
	assert.Contains(t, err.Error(), "requires at least 1 arg")
	assert.Empty(t, buf.String())
}

func TestNewQuoteNoSubcommand(t *testing.T) {
	server := jsonServer(`{}`)
	defer server.Close()

	var buf bytes.Buffer
	cmd := NewQuoteCmd(testClient(t, server), &buf)
	_, err := runTestCommand(t, cmd)
	require.Error(t, err)

	var valErr *apperr.ValidationError
	assert.ErrorAs(t, err, &valErr)
	assert.Contains(t, err.Error(), "requires a subcommand")
	assert.Contains(t, err.Error(), "get")
}

func TestNewQuoteGetWithFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Verify the fields query parameter is present in the URL.
		q := r.URL.Query()
		assert.Equal(t, "quote,fundamental", q.Get("fields"))

		if r.URL.Path == "/marketdata/v1/AAPL/quotes" {
			_, _ = w.Write([]byte(`{"AAPL":{"symbol":"AAPL","lastPrice":150.0}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	var buf bytes.Buffer
	cmd := NewQuoteCmd(testClient(t, server), &buf)
	_, err := runTestCommand(t, cmd, "get",
		"--fields", "quote",
		"--fields", "fundamental",
		"AAPL",
	)
	require.NoError(t, err)

	var envelope output.Envelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)

	aapl, ok := data["AAPL"].(map[string]any)
	require.True(t, ok, "single-symbol response should be nested under symbol key")
	assert.Equal(t, "AAPL", aapl["symbol"])
}

func TestNewQuoteGetMultiWithFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		q := r.URL.Query()
		assert.Equal(t, "extended", q.Get("fields"))

		if r.URL.Path == "/marketdata/v1/quotes" {
			_, _ = w.Write([]byte(`{"AAPL":{"symbol":"AAPL","lastPrice":150.0},"MSFT":{"symbol":"MSFT","lastPrice":400.0}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	var buf bytes.Buffer
	cmd := NewQuoteCmd(testClient(t, server), &buf)
	_, err := runTestCommand(t, cmd, "get",
		"--fields", "extended",
		"AAPL", "MSFT",
	)
	require.NoError(t, err)

	var envelope output.Envelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	assert.Contains(t, data, "AAPL")
	assert.Contains(t, data, "MSFT")
}

func TestNewQuoteGetWithIndicative(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Verify the indicative query parameter is set to "true".
		q := r.URL.Query()
		assert.Equal(t, "true", q.Get("indicative"))

		if r.URL.Path == "/marketdata/v1/AAPL/quotes" {
			_, _ = w.Write([]byte(`{"AAPL":{"symbol":"AAPL","lastPrice":150.0}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	var buf bytes.Buffer
	cmd := NewQuoteCmd(testClient(t, server), &buf)
	_, err := runTestCommand(t, cmd, "get",
		"--indicative",
		"AAPL",
	)
	require.NoError(t, err)

	var envelope output.Envelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
	assert.NotNil(t, envelope.Data)
}

func TestNewQuoteGetInvalidField(t *testing.T) {
	server := jsonServer(`{}`)
	defer server.Close()

	var buf bytes.Buffer
	cmd := NewQuoteCmd(testClient(t, server), &buf)
	_, err := runTestCommand(t, cmd, "get",
		"--fields", "bogus",
		"AAPL",
	)
	require.Error(t, err)

	// Validation now runs inside structcli.Unmarshal via quoteGetOpts.Validate(),
	// producing structcli's ValidationError instead of apperr's.
	var valErr *structclierrors.ValidationError
	assert.ErrorAs(t, err, &valErr)
	assert.Contains(t, err.Error(), "bogus")
	assert.Empty(t, buf.String())
}

func TestNewQuoteGetTechnicalFieldSuggestsTAHelp(t *testing.T) {
	server := jsonServer(`{}`)
	defer server.Close()

	var buf bytes.Buffer
	cmd := NewQuoteCmd(testClient(t, server), &buf)
	_, err := runTestCommand(t, cmd, "get",
		"--fields", "technical",
		"AAPL",
	)
	require.Error(t, err)

	assert.Contains(t, err.Error(), `invalid field "technical"`)
	assert.Contains(t, err.Error(), "schwab-agent ta --help")
	assert.Empty(t, buf.String())
}

func TestNewQuoteGetNoFlagsNoExtraParams(t *testing.T) {
	// When no --fields or --indicative flags are provided, no extra
	// query parameters should appear in the request URL.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		q := r.URL.Query()
		assert.Empty(t, q.Get("fields"), "fields should be absent when not specified")
		assert.Empty(t, q.Get("indicative"), "indicative should be absent when not specified")

		if r.URL.Path == "/marketdata/v1/AAPL/quotes" {
			_, _ = w.Write([]byte(`{"AAPL":{"symbol":"AAPL","lastPrice":150.0}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	var buf bytes.Buffer
	cmd := NewQuoteCmd(testClient(t, server), &buf)
	_, err := runTestCommand(t, cmd, "get", "AAPL")
	require.NoError(t, err)

	var envelope output.Envelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
	assert.NotNil(t, envelope.Data)
}

func TestNewQuoteGetCommaSeparatedFields(t *testing.T) {
	// --fields QUOTE,FUNDAMENTAL should be split and normalized to lowercase,
	// producing the same result as --fields QUOTE --fields FUNDAMENTAL.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		q := r.URL.Query()
		assert.Equal(t, "quote,fundamental", q.Get("fields"))

		if r.URL.Path == "/marketdata/v1/AAPL/quotes" {
			_, _ = w.Write([]byte(`{"AAPL":{"symbol":"AAPL","lastPrice":150.0}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	var buf bytes.Buffer
	cmd := NewQuoteCmd(testClient(t, server), &buf)
	_, err := runTestCommand(t, cmd, "get",
		"--fields", "QUOTE,FUNDAMENTAL",
		"AAPL",
	)
	require.NoError(t, err)

	var envelope output.Envelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)

	aapl, ok := data["AAPL"].(map[string]any)
	require.True(t, ok, "single-symbol response should be nested under symbol key")
	assert.Equal(t, "AAPL", aapl["symbol"])
}

func TestNewQuoteGetFieldsCaseInsensitive(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Mixed-case input should be normalized to lowercase.
		q := r.URL.Query()
		assert.Equal(t, "quote,reference", q.Get("fields"))

		if r.URL.Path == "/marketdata/v1/AAPL/quotes" {
			_, _ = w.Write([]byte(`{"AAPL":{"symbol":"AAPL","lastPrice":150.0}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	var buf bytes.Buffer
	cmd := NewQuoteCmd(testClient(t, server), &buf)
	_, err := runTestCommand(t, cmd, "get",
		"--fields", "Quote",
		"--fields", "REFERENCE",
		"AAPL",
	)
	require.NoError(t, err)

	var envelope output.Envelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
	assert.NotNil(t, envelope.Data)
}
