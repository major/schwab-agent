package commands

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	schwab "github.com/major/schwab-go/schwab"
	"github.com/major/schwab-go/schwab/marketdata"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/output"
)

// testClientWithMarketData creates a *client.Ref with both the internal client
// and a schwab-go marketdata.Client pointing at the given httptest server.
func testClientWithMarketData(t *testing.T, server *httptest.Server) *client.Ref {
	t.Helper()
	ref := testClient(t, server)
	ref.MarketData = marketdata.NewClient(
		schwab.WithToken("test-token"),
		schwab.WithBaseURL(server.URL),
	)
	return ref
}

func TestNewMarketCmd_Hours_AllMarkets(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/markets", r.URL.Path)
		// The API requires a "markets" query param; our command sends all by default.
		assert.NotEmpty(t, r.URL.Query().Get("markets"))

		w.Header().Set("Content-Type", "application/json")
		// Respond with the doubly-nested structure the real API uses.
		_, _ = w.Write([]byte(`{"equity":{"EQ":{"date":"2024-01-15","isOpen":true,"marketType":"EQUITY"}}}`))
	}))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewMarketCmd(testClientWithMarketData(t, srv), &buf)
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
		assert.Equal(t, "/markets/equity", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		// Single-market endpoint returns the same doubly-nested structure.
		_, _ = w.Write([]byte(`{"equity":{"EQ":{"date":"2024-01-15","isOpen":true,"marketType":"EQUITY"}}}`))
	}))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewMarketCmd(testClientWithMarketData(t, srv), &buf)
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
	cmd := NewMarketCmd(testClientWithMarketData(t, srv), &buf)
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
	cmd := NewMarketCmd(testClientWithMarketData(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "hours", "invalid")

	// Assert
	require.Error(t, err)
}

func TestNewMarketCmd_Movers_Success(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Contains(t, r.URL.Path, "/movers/")

		w.Header().Set("Content-Type", "application/json")
		// schwab-go MoverResponse shape: screeners array with full Screener fields.
		resp := `{"screeners":[{"symbol":"AAPL","description":"Apple Inc",` +
			`"direction":"up","last":150.0,"change":2.5,` +
			`"netPercentChange":1.69,"marketShare":0.05,` +
			`"totalVolume":1000000,"trades":5000}]}`
		_, _ = w.Write([]byte(resp))
	}))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewMarketCmd(testClientWithMarketData(t, srv), &buf)
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
	cmd := NewMarketCmd(testClientWithMarketData(t, srv), &buf)
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
	// Arrange - requireArg fires before the API call, so the server is never hit.
	server := jsonServer(`{}`)
	defer server.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewMarketCmd(testClientWithMarketData(t, server), &buf)
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
	cmd := NewMarketCmd(testClientWithMarketData(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "movers", "$SPX")

	// Assert
	require.Error(t, err)
}

func TestNewMarketCmd_Movers_FrequencyZero(t *testing.T) {
	// Arrange - verify that --frequency 0 sends frequency=0 in the query string
	// rather than being omitted. schwab-go uses *int so nil omits and non-nil sends.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		assert.Equal(t, "0", q.Get("frequency"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"screeners":[]}`))
	}))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewMarketCmd(testClientWithMarketData(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "movers", "--frequency", "0", "$SPX")

	// Assert
	require.NoError(t, err)
	var envelope output.Envelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
	assert.NotNil(t, envelope.Data)
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
	require.ErrorAs(t, err, &valErr)
	assert.Contains(t, err.Error(), "requires a subcommand")
	assert.Contains(t, err.Error(), "hours")
	assert.Contains(t, err.Error(), "movers")
}
