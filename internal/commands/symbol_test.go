package commands

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-agent/internal/output"
)

// TestSymbolBuild verifies the symbol build subcommand produces correct OCC symbols.
func TestSymbolBuild(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		wantSymbol string
		wantPutCall string
		wantStrike float64
	}{
		{
			name:        "aapl call",
			args:        []string{"symbol", "build", "--underlying", "AAPL", "--expiration", "2025-06-20", "--strike", "200", "--call"},
			wantSymbol:  "AAPL  250620C00200000",
			wantPutCall: "CALL",
			wantStrike:  200,
		},
		{
			name:        "spy put with decimal",
			args:        []string{"symbol", "build", "--underlying", "SPY", "--expiration", "2025-12-19", "--strike", "450.50", "--put"},
			wantSymbol:  "SPY   251219P00450500",
			wantPutCall: "PUT",
			wantStrike:  450.5,
		},
		{
			name:        "googl call",
			args:        []string{"symbol", "build", "--underlying", "GOOGL", "--expiration", "2025-01-17", "--strike", "150", "--call"},
			wantSymbol:  "GOOGL 250117C00150000",
			wantPutCall: "CALL",
			wantStrike:  150,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			cmd := SymbolCommand(&buf)

			err := runTestCommand(t, cmd, tt.args...)
			require.NoError(t, err)

			var env output.Envelope
			err = json.Unmarshal(buf.Bytes(), &env)
			require.NoError(t, err)

			data, ok := env.Data.(map[string]any)
			require.True(t, ok)
			assert.Equal(t, tt.wantSymbol, data["symbol"])
			assert.Equal(t, tt.wantPutCall, data["put_call"])
			assert.Equal(t, tt.wantStrike, data["strike"])
			assert.NotEmpty(t, env.Metadata.Timestamp)
		})
	}
}

// TestSymbolBuildValidation verifies that symbol build rejects invalid input.
func TestSymbolBuildValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		wantMsg string
	}{
		{
			name:    "no put or call flag",
			args:    []string{"symbol", "build", "--underlying", "AAPL", "--expiration", "2025-06-20", "--strike", "200"},
			wantMsg: "exactly one of --call or --put is required",
		},
		{
			name:    "both put and call flags",
			args:    []string{"symbol", "build", "--underlying", "AAPL", "--expiration", "2025-06-20", "--strike", "200", "--call", "--put"},
			wantMsg: "exactly one of --call or --put is required",
		},
		{
			name:    "invalid expiration format",
			args:    []string{"symbol", "build", "--underlying", "AAPL", "--expiration", "06/20/2025", "--strike", "200", "--call"},
			wantMsg: "YYYY-MM-DD",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			cmd := SymbolCommand(&buf)

			err := runTestCommand(t, cmd, tt.args...)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantMsg)
		})
	}
}

// TestSymbolParse verifies the symbol parse subcommand decomposes OCC symbols.
func TestSymbolParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		symbol         string
		wantUnderlying string
		wantExpiration string
		wantPutCall    string
		wantStrike     float64
	}{
		{
			name:           "aapl call",
			symbol:         "AAPL  250620C00200000",
			wantUnderlying: "AAPL",
			wantExpiration: "2025-06-20",
			wantPutCall:    "CALL",
			wantStrike:     200,
		},
		{
			name:           "spy put with decimal",
			symbol:         "SPY   251219P00450500",
			wantUnderlying: "SPY",
			wantExpiration: "2025-12-19",
			wantPutCall:    "PUT",
			wantStrike:     450.5,
		},
		{
			name:           "single char underlying",
			symbol:         "X     250620C00001500",
			wantUnderlying: "X",
			wantExpiration: "2025-06-20",
			wantPutCall:    "CALL",
			wantStrike:     1.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			cmd := SymbolCommand(&buf)

			err := runTestCommand(t, cmd, "symbol", "parse", tt.symbol)
			require.NoError(t, err)

			var env output.Envelope
			err = json.Unmarshal(buf.Bytes(), &env)
			require.NoError(t, err)

			data, ok := env.Data.(map[string]any)
			require.True(t, ok)
			assert.Equal(t, tt.symbol, data["symbol"])
			assert.Equal(t, tt.wantUnderlying, data["underlying"])
			assert.Equal(t, tt.wantExpiration, data["expiration"])
			assert.Equal(t, tt.wantPutCall, data["put_call"])
			assert.Equal(t, tt.wantStrike, data["strike"])
			assert.NotEmpty(t, env.Metadata.Timestamp)
		})
	}
}

// TestSymbolParseValidation verifies that symbol parse rejects invalid input.
func TestSymbolParseValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		wantMsg string
	}{
		{
			name:    "no argument",
			args:    []string{"symbol", "parse"},
			wantMsg: "OCC symbol argument is required",
		},
		{
			name:    "too short",
			args:    []string{"symbol", "parse", "AAPL"},
			wantMsg: "must be 21 characters",
		},
		{
			name:    "invalid put/call char",
			args:    []string{"symbol", "parse", "AAPL  250620X00200000"},
			wantMsg: "invalid put/call indicator",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			cmd := SymbolCommand(&buf)

			err := runTestCommand(t, cmd, tt.args...)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantMsg)
		})
	}
}

// TestSymbolParseRoundTrip verifies that build -> parse produces consistent components.
func TestSymbolParseRoundTrip(t *testing.T) {
	var buildBuf bytes.Buffer
	buildCmd := SymbolCommand(&buildBuf)

	// Build
	err := runTestCommand(t, buildCmd, "symbol", "build",
		"--underlying", "TSLA", "--expiration", "2026-01-16", "--strike", "275.50", "--put")
	require.NoError(t, err)

	var buildEnv output.Envelope
	err = json.Unmarshal(buildBuf.Bytes(), &buildEnv)
	require.NoError(t, err)
	buildData := buildEnv.Data.(map[string]any)
	symbol := buildData["symbol"].(string)

	// Parse
	var parseBuf bytes.Buffer
	parseCmd := SymbolCommand(&parseBuf)

	err = runTestCommand(t, parseCmd, "symbol", "parse", symbol)
	require.NoError(t, err)

	var parseEnv output.Envelope
	err = json.Unmarshal(parseBuf.Bytes(), &parseEnv)
	require.NoError(t, err)
	parseData := parseEnv.Data.(map[string]any)

	// Components should match
	assert.Equal(t, "TSLA", parseData["underlying"])
	assert.Equal(t, "2026-01-16", parseData["expiration"])
	assert.Equal(t, "PUT", parseData["put_call"])
	assert.Equal(t, 275.5, parseData["strike"])
	assert.Equal(t, symbol, parseData["symbol"])
}
