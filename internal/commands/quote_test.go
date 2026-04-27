package commands

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQuoteGetSymbols(t *testing.T) {
	// Table-driven test covering both explicit ("quote get") and shorthand
	// ("quote") invocation paths for single and multi-symbol lookups.
	// Keeping them in one table ensures the two routing paths stay in sync.
	// Single-symbol responses are now normalized to match multi-symbol shape
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
			args:       []string{"quote", "get", "AAPL"},
			expectKeys: []string{"AAPL"},
		},
		{
			name:       "shorthand single symbol",
			serverPath: "/marketdata/v1/AAPL/quotes",
			serverBody: `{"AAPL":{"symbol":"AAPL","lastPrice":150.0}}`,
			args:       []string{"quote", "AAPL"},
			expectKeys: []string{"AAPL"},
		},
		{
			name:       "explicit multiple symbols",
			serverPath: "/marketdata/v1/quotes",
			serverBody: `{"AAPL":{"symbol":"AAPL","lastPrice":150.0},"MSFT":{"symbol":"MSFT","lastPrice":400.0}}`,
			args:       []string{"quote", "get", "AAPL", "MSFT"},
			expectKeys: []string{"AAPL", "MSFT"},
		},
		{
			name:       "shorthand multiple symbols",
			serverPath: "/marketdata/v1/quotes",
			serverBody: `{"AAPL":{"symbol":"AAPL","lastPrice":150.0},"MSFT":{"symbol":"MSFT","lastPrice":400.0}}`,
			args:       []string{"quote", "AAPL", "MSFT"},
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
			cmd := QuoteCommand(testClient(t, server), &buf)
			require.NoError(t, runTestCommand(t, cmd, tt.args...))

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

func TestQuoteGetPartialSuccess(t *testing.T) {
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
	cmd := QuoteCommand(testClient(t, server), &buf)
	require.NoError(t, runTestCommand(t, cmd, "quote", "get", "AAPL", "INVALID"))

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

func TestQuoteGetSingleNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))
	defer server.Close()

	var buf bytes.Buffer
	cmd := QuoteCommand(testClient(t, server), &buf)
	err := runTestCommand(t, cmd, "quote", "get", "INVALID")
	require.Error(t, err)

	var symErr *apperr.SymbolNotFoundError
	assert.ErrorAs(t, err, &symErr)
	assert.Empty(t, buf.String())
}

func TestQuoteGetNoArgs(t *testing.T) {
	server := jsonServer(`{}`)
	defer server.Close()

	var buf bytes.Buffer
	cmd := QuoteCommand(testClient(t, server), &buf)
	err := runTestCommand(t, cmd, "quote", "get")
	require.Error(t, err)

	var valErr *apperr.ValidationError
	assert.ErrorAs(t, err, &valErr)
	assert.Empty(t, buf.String())
}

func TestQuoteGetWithFields(t *testing.T) {
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
	cmd := QuoteCommand(testClient(t, server), &buf)
	require.NoError(t, runTestCommand(t, cmd, "quote", "get",
		"--fields", "quote",
		"--fields", "fundamental",
		"AAPL",
	))

	var envelope output.Envelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)

	aapl, ok := data["AAPL"].(map[string]any)
	require.True(t, ok, "single-symbol response should be nested under symbol key")
	assert.Equal(t, "AAPL", aapl["symbol"])
}

func TestQuoteGetMultiWithFields(t *testing.T) {
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
	cmd := QuoteCommand(testClient(t, server), &buf)
	require.NoError(t, runTestCommand(t, cmd, "quote", "get",
		"--fields", "extended",
		"AAPL", "MSFT",
	))

	var envelope output.Envelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	assert.Contains(t, data, "AAPL")
	assert.Contains(t, data, "MSFT")
}

func TestQuoteGetWithIndicative(t *testing.T) {
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
	cmd := QuoteCommand(testClient(t, server), &buf)
	require.NoError(t, runTestCommand(t, cmd, "quote", "get",
		"--indicative",
		"AAPL",
	))

	var envelope output.Envelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
	assert.NotNil(t, envelope.Data)
}

func TestQuoteGetInvalidField(t *testing.T) {
	server := jsonServer(`{}`)
	defer server.Close()

	var buf bytes.Buffer
	cmd := QuoteCommand(testClient(t, server), &buf)
	err := runTestCommand(t, cmd, "quote", "get",
		"--fields", "bogus",
		"AAPL",
	)
	require.Error(t, err)

	var valErr *apperr.ValidationError
	assert.ErrorAs(t, err, &valErr)
	assert.Contains(t, err.Error(), "bogus")
	assert.Empty(t, buf.String())
}

func TestQuoteGetNoFlagsNoExtraParams(t *testing.T) {
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
	cmd := QuoteCommand(testClient(t, server), &buf)
	require.NoError(t, runTestCommand(t, cmd, "quote", "get", "AAPL"))

	var envelope output.Envelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
	assert.NotNil(t, envelope.Data)
}

func TestQuoteGetCommaSeparatedFields(t *testing.T) {
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
	cmd := QuoteCommand(testClient(t, server), &buf)
	require.NoError(t, runTestCommand(t, cmd, "quote", "get",
		"--fields", "QUOTE,FUNDAMENTAL",
		"AAPL",
	))

	var envelope output.Envelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)

	aapl, ok := data["AAPL"].(map[string]any)
	require.True(t, ok, "single-symbol response should be nested under symbol key")
	assert.Equal(t, "AAPL", aapl["symbol"])
}

func TestQuoteGetFieldsCaseInsensitive(t *testing.T) {
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
	cmd := QuoteCommand(testClient(t, server), &buf)
	require.NoError(t, runTestCommand(t, cmd, "quote", "get",
		"--fields", "Quote",
		"--fields", "REFERENCE",
		"AAPL",
	))

	var envelope output.Envelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
	assert.NotNil(t, envelope.Data)
}
