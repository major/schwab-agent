package commands

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/output"
)

func TestNewMarketCmd_Hours_AllMarkets(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/marketdata/v1/markets", r.URL.Path)
		// The API requires a "markets" query param; our command sends all by default.
		assert.NotEmpty(t, r.URL.Query().Get("markets"))

		w.Header().Set("Content-Type", "application/json")
		// Respond with the doubly-nested structure the real API uses.
		_, _ = w.Write([]byte(`{"equity":{"EQ":{"date":"2024-01-15","isOpen":true,"marketType":"EQUITY"}}}`))
	}))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewMarketCmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "hours")

	// Assert
	require.NoError(t, err)
	var envelope output.Envelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
	assert.NotNil(t, envelope.Data)
	assert.NotEmpty(t, envelope.Metadata.Timestamp)
}

func TestNewMarketCmd_Hours_SpecificMarket(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/marketdata/v1/markets/equity", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		// Single-market endpoint returns the same doubly-nested structure.
		_, _ = w.Write([]byte(`{"equity":{"EQ":{"date":"2024-01-15","isOpen":true,"marketType":"EQUITY"}}}`))
	}))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewMarketCmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "hours", "equity")

	// Assert
	require.NoError(t, err)
	var envelope output.Envelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
	assert.NotNil(t, envelope.Data)
	assert.NotEmpty(t, envelope.Metadata.Timestamp)
}

func TestNewMarketCmd_Hours_APIError(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"server error"}`))
	}))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewMarketCmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "hours")

	// Assert
	require.Error(t, err)
}

func TestNewMarketCmd_Hours_SpecificMarketAPIError(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewMarketCmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "hours", "invalid")

	// Assert
	require.Error(t, err)
}

func TestNewMarketCmd_Movers_Success(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Contains(t, r.URL.Path, "/marketdata/v1/movers/")

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"screeners":[{"description":"AAPL","totalVolume":1000000}]}`))
	}))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewMarketCmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "movers", "$SPX")

	// Assert
	require.NoError(t, err)
	var envelope output.Envelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
	assert.NotNil(t, envelope.Data)
	assert.NotEmpty(t, envelope.Metadata.Timestamp)
}

func TestNewMarketCmd_Movers_WithFlags(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		assert.Equal(t, "VOLUME", q.Get("sort"))
		assert.Equal(t, "5", q.Get("frequency"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"screeners":[]}`))
	}))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewMarketCmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd,
		"movers",
		"--sort", "VOLUME",
		"--frequency", "5",
		"$DJI",
	)

	// Assert
	require.NoError(t, err)
	var envelope output.Envelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
	assert.NotNil(t, envelope.Data)
}

func TestNewMarketCmd_Movers_MissingIndex(t *testing.T) {
	// Arrange
	server := jsonServer(`{}`)
	defer server.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewMarketCmd(testClient(t, server), &buf)
	_, err := runTestCommand(t, cmd, "movers")

	// Assert
	require.Error(t, err)
	var valErr *apperr.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

func TestNewMarketCmd_Movers_APIError(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"server error"}`))
	}))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewMarketCmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "movers", "$SPX")

	// Assert
	require.Error(t, err)
}

func TestNewMarketCmd_NoSubcommand(t *testing.T) {
	// Arrange
	server := jsonServer(`{}`)
	defer server.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewMarketCmd(testClient(t, server), &buf)
	_, err := runTestCommand(t, cmd)

	// Assert
	require.Error(t, err)
	var valErr *apperr.ValidationError
	assert.ErrorAs(t, err, &valErr)
	assert.Contains(t, err.Error(), "requires a subcommand")
	assert.Contains(t, err.Error(), "hours")
	assert.Contains(t, err.Error(), "movers")
}
