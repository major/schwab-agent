package commands

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	schwabErrors "github.com/major/schwab-agent/internal/errors"
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
	assert.NotNil(t, envelope.Metadata["timestamp"])
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
	assert.NotNil(t, envelope.Metadata["timestamp"])
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

	assert.Equal(t, float64(2), envelope.Metadata["requested"])
	assert.Equal(t, float64(1), envelope.Metadata["returned"])
	assert.NotNil(t, envelope.Metadata["timestamp"])
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

	var symErr *schwabErrors.SymbolNotFoundError
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

	var valErr *schwabErrors.ValidationError
	assert.ErrorAs(t, err, &valErr)
	assert.Empty(t, buf.String())
}
