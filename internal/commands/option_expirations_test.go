package commands

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/output"
)

func TestSelectNearestExpiration(t *testing.T) {
	// Fixed reference time: Wednesday 2026-05-06 12:00 ET.
	// All DTE calculations use America/New_York calendar dates.
	nyLoc, loadErr := time.LoadLocation("America/New_York")
	require.NoError(t, loadErr, "loading America/New_York timezone")
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, nyLoc)

	tests := []struct {
		name        string
		expirations []string
		targetDTE   int
		now         time.Time
		want        string
		wantErrType any // nil, *apperr.ValidationError, or *apperr.SymbolNotFoundError
	}{
		{
			name:        "exact match 30 DTE",
			expirations: []string{"2026-06-05"},
			targetDTE:   30,
			now:         now,
			want:        "2026-06-05",
			wantErrType: nil,
		},
		{
			name: "nearest picks closer expiration",
			// 2026-05-22 = 16 DTE, 2026-06-19 = 44 DTE. Target 30.
			// |16-30|=14, |44-30|=14 -> tie, pick earlier (2026-05-22).
			// Use asymmetric distances to test nearest logic without tie-breaking.
			// 2026-05-22 = 16 DTE, 2026-06-26 = 51 DTE. Target 30.
			// |16-30|=14, |51-30|=21 -> 2026-05-22 is closer.
			expirations: []string{"2026-05-22", "2026-06-26"},
			targetDTE:   30,
			now:         now,
			want:        "2026-05-22",
			wantErrType: nil,
		},
		{
			name: "nearest picks later when closer",
			// 2026-05-13 = 7 DTE, 2026-05-29 = 23 DTE. Target 20.
			// |7-20|=13, |23-20|=3 -> 2026-05-29 is closer.
			expirations: []string{"2026-05-13", "2026-05-29"},
			targetDTE:   20,
			now:         now,
			want:        "2026-05-29",
			wantErrType: nil,
		},
		{
			name: "tie breaking picks earlier expiration",
			// 2026-05-16 = 10 DTE, 2026-05-26 = 20 DTE. Target 15.
			// |10-15|=5, |20-15|=5 -> equidistant, pick earlier (2026-05-16).
			expirations: []string{"2026-05-16", "2026-05-26"},
			targetDTE:   15,
			now:         now,
			want:        "2026-05-16",
			wantErrType: nil,
		},
		{
			name: "tie breaking with reversed input order",
			// Same equidistant case but expirations listed in reverse order.
			// Must still pick earlier date (2026-05-16).
			expirations: []string{"2026-05-26", "2026-05-16"},
			targetDTE:   15,
			now:         now,
			want:        "2026-05-16",
			wantErrType: nil,
		},
		{
			name:        "single expiration always selected",
			expirations: []string{"2026-12-18"},
			targetDTE:   45,
			now:         now,
			want:        "2026-12-18",
			wantErrType: nil,
		},
		{
			name:        "invalid DTE zero returns ValidationError",
			expirations: []string{"2026-06-05"},
			targetDTE:   0,
			now:         now,
			wantErrType: &apperr.ValidationError{},
		},
		{
			name:        "invalid DTE negative returns ValidationError",
			expirations: []string{"2026-06-05"},
			targetDTE:   -1,
			now:         now,
			wantErrType: &apperr.ValidationError{},
		},
		{
			name:        "empty expirations returns SymbolNotFoundError",
			expirations: []string{},
			targetDTE:   30,
			now:         now,
			wantErrType: &apperr.SymbolNotFoundError{},
		},
		{
			name:        "nil expirations returns SymbolNotFoundError",
			expirations: nil,
			targetDTE:   30,
			now:         now,
			wantErrType: &apperr.SymbolNotFoundError{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got, err := selectNearestExpiration(tt.expirations, tt.targetDTE, tt.now)

			// Assert
			switch tt.wantErrType.(type) {
			case nil:
				require.NoError(
					t,
					err,
					"selectNearestExpiration(%v, %d) unexpected error",
					tt.expirations,
					tt.targetDTE,
				)
				assert.Equal(
					t,
					tt.want,
					got,
					"selectNearestExpiration(%v, %d) = %q, want %q",
					tt.expirations,
					tt.targetDTE,
					got,
					tt.want,
				)
			case *apperr.ValidationError:
				require.Error(
					t,
					err,
					"selectNearestExpiration(%v, %d) expected ValidationError",
					tt.expirations,
					tt.targetDTE,
				)
				var target *apperr.ValidationError
				require.ErrorAs(
					t,
					err,
					&target,
					"selectNearestExpiration(%v, %d) error type = %T, want *apperr.ValidationError",
					tt.expirations,
					tt.targetDTE,
					err,
				)
			case *apperr.SymbolNotFoundError:
				require.Error(
					t,
					err,
					"selectNearestExpiration(%v, %d) expected SymbolNotFoundError",
					tt.expirations,
					tt.targetDTE,
				)
				var target *apperr.SymbolNotFoundError
				require.ErrorAs(
					t,
					err,
					&target,
					"selectNearestExpiration(%v, %d) error type = %T, want *apperr.SymbolNotFoundError",
					tt.expirations,
					tt.targetDTE,
					err,
				)
			}
		})
	}
}

func TestSelectNearestExpirationTimezone(t *testing.T) {
	// Verify DTE uses America/New_York calendar dates, not UTC.
	// At 2026-05-06 23:00 UTC = 2026-05-06 19:00 ET (same calendar day).
	// At 2026-05-07 01:00 UTC = 2026-05-06 21:00 ET (prev calendar day in ET).
	// Expiration 2026-05-07 should be 1 DTE from the ET perspective of May 6.
	nyLoc, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)

	// 2026-05-07 01:00 UTC = 2026-05-06 21:00 ET.
	// In ET, "today" is May 6. Expiration May 7 = 1 DTE.
	// Without timezone handling (using UTC date May 7), DTE would be 0.
	nowUTC := time.Date(2026, 5, 7, 1, 0, 0, 0, time.UTC)

	got, err := selectNearestExpiration(
		[]string{"2026-05-07", "2026-05-21"},
		1,
		nowUTC.In(nyLoc),
	)
	require.NoError(t, err)
	assert.Equal(t, "2026-05-07", got, "should use ET calendar date for DTE, not UTC")
}

func TestFlattenExpirationRows(t *testing.T) {
	// Expected column order for the compact expirations view.
	wantColumns := []string{"expiration", "dte", "expirationType", "standard"}

	tests := []struct {
		name     string
		chain    *client.ExpirationChain
		now      time.Time
		wantCols []string
		wantRows [][]any
		wantErr  bool
	}{
		{
			name: "single standard monthly expiration",
			chain: &client.ExpirationChain{
				ExpirationList: []client.ExpirationDate{
					{ExpirationDate: "2026-06-19"},
				},
			},
			now:      mustNewYorkTime(t, 5, 6, 12, 0),
			wantCols: wantColumns,
			wantRows: [][]any{
				{"2026-06-19", 44, "R", true},
			},
		},
		{
			name: "multiple expirations sorted by date",
			chain: &client.ExpirationChain{
				ExpirationList: []client.ExpirationDate{
					{ExpirationDate: "2026-07-17"},
					{ExpirationDate: "2026-05-15"},
					{ExpirationDate: "2026-06-19"},
				},
			},
			now:      mustNewYorkTime(t, 5, 6, 12, 0),
			wantCols: wantColumns,
			wantRows: [][]any{
				// Rows sorted by expiration date ascending.
				{"2026-05-15", 9, "R", true},
				{"2026-06-19", 44, "R", true},
				{"2026-07-17", 72, "R", true},
			},
		},
		{
			name: "weekly expiration on non-third-Friday",
			chain: &client.ExpirationChain{
				ExpirationList: []client.ExpirationDate{
					{ExpirationDate: "2026-05-08"},
				},
			},
			now:      mustNewYorkTime(t, 5, 6, 12, 0),
			wantCols: wantColumns,
			wantRows: [][]any{
				{"2026-05-08", 2, "S", false},
			},
		},
		{
			name: "empty expiration list",
			chain: &client.ExpirationChain{
				ExpirationList: []client.ExpirationDate{},
			},
			now:      mustNewYorkTime(t, 5, 6, 12, 0),
			wantCols: wantColumns,
			wantRows: [][]any{},
		},
		{
			name:     "nil chain",
			chain:    nil,
			now:      mustNewYorkTime(t, 5, 6, 12, 0),
			wantCols: wantColumns,
			wantRows: [][]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			gotCols, gotRows, err := flattenExpirationRows(tt.chain, tt.now)

			// Assert - error
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Assert - columns match
			assert.Equal(t, tt.wantCols, gotCols, "columns mismatch")

			// Assert - row count
			require.Len(t, gotRows, len(tt.wantRows), "row count mismatch")

			// Assert - row content
			for i, wantRow := range tt.wantRows {
				gotRow := gotRows[i]
				require.Len(t, gotRow, len(wantColumns), "row %d length mismatch", i)
				assert.Equal(t, wantRow[0], gotRow[0], "row %d expiration", i)
				assert.Equal(t, wantRow[1], gotRow[1], "row %d dte", i)
				assert.Equal(t, wantRow[2], gotRow[2], "row %d expirationType", i)
				assert.Equal(t, wantRow[3], gotRow[3], "row %d standard", i)
			}
		})
	}
}

func TestFlattenExpirationRowsRowCount(t *testing.T) {
	// Verify that rowCount in metadata matches len(rows).
	chain := &client.ExpirationChain{
		ExpirationList: []client.ExpirationDate{
			{ExpirationDate: "2026-05-15"},
			{ExpirationDate: "2026-06-19"},
			{ExpirationDate: "2026-07-17"},
		},
	}
	now := mustNewYorkTime(t, 5, 6, 12, 0)

	_, gotRows, err := flattenExpirationRows(chain, now)
	require.NoError(t, err)
	assert.Len(t, gotRows, 3, "rowCount should match number of expirations")
}

// mustNewYorkTime creates a [time.Time] in America/New_York for the test year 2026.
// Fails the test if the timezone cannot be loaded.
//
//nolint:unparam // test helper designed for reuse; current fixtures happen to share month/day values
func mustNewYorkTime(t *testing.T, month time.Month, day, hour, minute int) time.Time {
	t.Helper()
	loc, err := time.LoadLocation("America/New_York")
	require.NoError(t, err, "loading America/New_York timezone")
	return time.Date(2026, month, day, hour, minute, 0, 0, loc)
}

func TestOptionExpirationsCommand(t *testing.T) {
	t.Run("successful compact output", func(t *testing.T) {
		// Arrange - mock server returning two expirations (schwab-go uses "expiration" field).
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Path == "/marketdata/v1/expirationchain" {
				assert.Equal(t, "AAPL", r.URL.Query().Get("symbol"))
				_, _ = w.Write([]byte(`{"expirationList":[{"expiration":"2026-06-19"},{"expiration":"2026-05-15"}]}`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		var buf bytes.Buffer
		cmd := NewOptionCmd(testClient(t, server), &buf)

		// Act
		_, err := runTestCommand(t, cmd, "expirations", "AAPL")

		// Assert - no error
		require.NoError(t, err)

		var envelope output.Envelope
		require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))

		data, ok := envelope.Data.(map[string]any)
		require.True(t, ok, "data should be a map")

		assert.Equal(t, "AAPL", data["underlying"])

		// Columns should match the expirations view.
		cols, ok := data["columns"].([]any)
		require.True(t, ok, "columns should be a slice")
		assert.Len(t, cols, 4, "expected 4 columns: expiration, dte, expirationType, standard")

		// Rows sorted by expiration date ascending: 2026-05-15 before 2026-06-19.
		rows, ok := data["rows"].([]any)
		require.True(t, ok, "rows should be a slice")
		assert.Len(t, rows, 2, "expected 2 expiration rows")

		// rowCount matches row count.
		rowCount, ok := data["rowCount"].(float64) // JSON numbers decode as float64
		require.True(t, ok)
		assert.InDelta(t, 2.0, rowCount, 0.001)

		// metadata.returned matches row count.
		assert.Equal(t, 2, envelope.Metadata.Returned)
	})

	t.Run("empty chain returns SymbolNotFoundError", func(t *testing.T) {
		// Arrange - mock server returning empty expiration list.
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Path == "/marketdata/v1/expirationchain" {
				_, _ = w.Write([]byte(`{"expirationList":[]}`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		var buf bytes.Buffer
		cmd := NewOptionCmd(testClient(t, server), &buf)

		// Act
		_, err := runTestCommand(t, cmd, "expirations", "AAPL")

		// Assert
		require.Error(t, err)
		var symbolErr *apperr.SymbolNotFoundError
		require.ErrorAs(t, err, &symbolErr, "expected SymbolNotFoundError, got %T", err)
		assert.Contains(t, err.Error(), "no expirations found for AAPL")
	})

	t.Run("missing symbol argument", func(t *testing.T) {
		// Arrange
		server := jsonServer(`{}`)
		defer server.Close()

		var buf bytes.Buffer
		cmd := NewOptionCmd(testClient(t, server), &buf)

		// Act
		_, err := runTestCommand(t, cmd, "expirations")

		// Assert - cobra.ExactArgs returns a plain error
		require.Error(t, err)
		assert.Contains(t, err.Error(), "accepts 1 arg(s), received 0")
	})
}
