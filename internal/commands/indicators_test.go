package commands

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/client"
)

func TestNewIndicatorsCmd_Metadata(t *testing.T) {
	// Arrange
	ref := &client.Ref{}
	var buf bytes.Buffer
	cmd := NewIndicatorsCmd(ref, &buf)

	// Assert
	assert.Equal(t, "indicators SYMBOL [SYMBOL...]", cmd.Use)
	assert.Equal(t, "market-data", cmd.GroupID)
	assert.Empty(t, cmd.Aliases, "indicators should not have aliases")
	assert.Contains(t, cmd.Short, "shortcut")
	assert.Contains(t, cmd.Long, "ta dashboard")
}

func TestNewIndicatorsCmd_Flags(t *testing.T) {
	// Arrange
	ref := &client.Ref{}
	var buf bytes.Buffer
	cmd := NewIndicatorsCmd(ref, &buf)

	// Assert - structcli-registered flags exist with correct defaults
	intervalFlag := cmd.Flags().Lookup("interval")
	require.NotNil(t, intervalFlag, "--interval flag should be registered")
	assert.Equal(t, "daily", intervalFlag.DefValue)

	pointsFlag := cmd.Flags().Lookup("points")
	require.NotNil(t, pointsFlag, "--points flag should be registered")
	assert.Equal(t, "1", pointsFlag.DefValue)
}

func TestNewIndicatorsCmd_NoArgs(t *testing.T) {
	// Arrange
	ref := &client.Ref{}
	var buf bytes.Buffer
	cmd := NewIndicatorsCmd(ref, &buf)

	// Act
	_, err := runTestCommand(t, cmd)

	// Assert - MinimumNArgs(1) triggers an error
	require.Error(t, err)
}

func TestNewIndicatorsCmd_HelpText(t *testing.T) {
	// Arrange
	ref := &client.Ref{}
	var buf bytes.Buffer
	cmd := NewIndicatorsCmd(ref, &buf)

	// Act
	output, err := runTestCommand(t, cmd, "--help")
	require.NoError(t, err)

	// Assert
	assert.Contains(t, output, "indicators SYMBOL")
	assert.Contains(t, output, "ta dashboard")
	assert.Contains(t, output, "--interval")
	assert.Contains(t, output, "--points")
}

func TestNewIndicatorsCmd_ValidEnvelope(t *testing.T) {
	// Arrange - same pattern as TestTADashboard_ValidEnvelope
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/marketdata/v1/pricehistory")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mockCandleListJSON("AAPL", 252)))
	}))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewIndicatorsCmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "AAPL")
	require.NoError(t, err)
	outputBytes := append([]byte(nil), buf.Bytes()...)

	// Assert
	envelope, data := decodeTAEnvelope(t, bytes.NewBuffer(outputBytes))
	assert.NotEmpty(t, envelope.Metadata.Timestamp)
	assert.Equal(t, "dashboard", data["indicator"])
	assert.Equal(t, "AAPL", data["symbol"])
	assert.Equal(t, "daily", data["interval"])

	_, root := decodeTAEnvelopeRoot(t, bytes.NewBuffer(outputBytes))
	_, ok := root["AAPL"].(map[string]any)
	require.True(t, ok, "indicators shortcut should return the normalized symbol map")
}

func TestNewIndicatorsCmd_IntervalFlag(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mockCandleListJSON("NVDA", 252)))
	}))
	defer srv.Close()

	// Act
	var buf bytes.Buffer
	cmd := NewIndicatorsCmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "--interval", "weekly", "NVDA")
	require.NoError(t, err)

	// Assert
	_, data := decodeTAEnvelope(t, &buf)
	assert.Equal(t, "weekly", data["interval"])
}

func TestNewIndicatorsCmd_InvalidInterval(t *testing.T) {
	// Arrange
	ref := &client.Ref{}
	var buf bytes.Buffer
	cmd := NewIndicatorsCmd(ref, &buf)

	// Act
	_, err := runTestCommand(t, cmd, "--interval", "bogus", "AAPL")

	// Assert - normalizeFlagValidationErrorFunc wraps invalid enum values
	var valErr *apperr.ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Contains(t, valErr.Error(), "bogus")
}
