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

func TestQuoteGetSingleSymbol(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
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
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "AAPL", data["symbol"])
	assert.NotEmpty(t, envelope.Metadata.Timestamp)
	assert.Empty(t, envelope.Errors)
}

func TestQuoteGetMultipleSymbols(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/marketdata/v1/quotes" {
			_, _ = w.Write([]byte(`{"AAPL":{"symbol":"AAPL","lastPrice":150.0},"MSFT":{"symbol":"MSFT","lastPrice":400.0}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	var buf bytes.Buffer
	cmd := QuoteCommand(testClient(t, server), &buf)
	require.NoError(t, runTestCommand(t, cmd, "quote", "get", "AAPL", "MSFT"))

	var envelope output.Envelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	assert.Contains(t, data, "AAPL")
	assert.Contains(t, data, "MSFT")
	assert.Empty(t, envelope.Errors)
	assert.NotEmpty(t, envelope.Metadata.Timestamp)
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
	assert.Equal(t, "AAPL", data["symbol"])
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
	assert.Equal(t, "AAPL", data["symbol"])
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
