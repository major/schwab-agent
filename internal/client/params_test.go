package client

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetParam(t *testing.T) {
	tests := []struct {
		name          string
		key           string
		value         string
		expectedInMap bool
		expectedValue string
	}{
		{
			name:          "non-empty value is added",
			key:           "symbol",
			value:         "AAPL",
			expectedInMap: true,
			expectedValue: "AAPL",
		},
		{
			name:          "empty value is not added",
			key:           "symbol",
			value:         "",
			expectedInMap: false,
			expectedValue: "",
		},
		{
			name:          "whitespace-only value is added (not trimmed)",
			key:           "filter",
			value:         "   ",
			expectedInMap: true,
			expectedValue: "   ",
		},
		{
			name:          "multiple keys can be set",
			key:           "status",
			value:         "FILLED",
			expectedInMap: true,
			expectedValue: "FILLED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			m := make(map[string]string)

			// Act
			setParam(m, tt.key, tt.value)

			// Assert
			if tt.expectedInMap {
				require.Contains(t, m, tt.key)
				assert.Equal(t, tt.expectedValue, m[tt.key])
			} else {
				assert.NotContains(t, m, tt.key)
			}
		})
	}
}

func TestSetParam_OverwritesExistingValue(t *testing.T) {
	// Arrange
	m := map[string]string{"symbol": "AAPL"}

	// Act
	setParam(m, "symbol", "MSFT")

	// Assert
	assert.Equal(t, "MSFT", m["symbol"])
}

func TestSetParam_DoesNotRemoveExistingValueWhenNewValueEmpty(t *testing.T) {
	// Arrange
	m := map[string]string{"symbol": "AAPL"}

	// Act
	setParam(m, "symbol", "")

	// Assert
	assert.Equal(t, "AAPL", m["symbol"])
}

func TestDefaultDateRange(t *testing.T) {
	tests := []struct {
		name              string
		from              string
		to                string
		expectFromDefault bool
		expectToDefault   bool
	}{
		{
			name:              "both empty uses defaults",
			from:              "",
			to:                "",
			expectFromDefault: true,
			expectToDefault:   true,
		},
		{
			name:              "from provided, to empty uses to default",
			from:              "2024-01-01T00:00:00Z",
			to:                "",
			expectFromDefault: false,
			expectToDefault:   true,
		},
		{
			name:              "from empty, to provided uses from default",
			from:              "",
			to:                "2024-12-31T23:59:59Z",
			expectFromDefault: true,
			expectToDefault:   false,
		},
		{
			name:              "both provided passes through unchanged",
			from:              "2024-01-01T00:00:00Z",
			to:                "2024-12-31T23:59:59Z",
			expectFromDefault: false,
			expectToDefault:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			now := time.Now().UTC()
			expectedFrom := now.AddDate(0, 0, -60).Format(time.RFC3339)

			// Act
			fromDate, toDate := defaultDateRange(tt.from, tt.to)

			// Assert
			// Verify RFC3339 format by parsing
			_, err := time.Parse(time.RFC3339, fromDate)
			require.NoError(t, err, "fromDate should be valid RFC3339")

			_, err = time.Parse(time.RFC3339, toDate)
			require.NoError(t, err, "toDate should be valid RFC3339")

			if tt.expectFromDefault {
				// Check that fromDate is approximately 60 days ago (within 1 second tolerance)
				parsedFrom, _ := time.Parse(time.RFC3339, fromDate)
				expectedFromTime, _ := time.Parse(time.RFC3339, expectedFrom)
				diff := parsedFrom.Sub(expectedFromTime).Abs()
				assert.Less(t, diff, time.Second, "fromDate should be approximately 60 days ago")
			} else {
				assert.Equal(t, tt.from, fromDate, "fromDate should match provided value")
			}

			if tt.expectToDefault {
				// Check that toDate is approximately now (within 1 second tolerance)
				parsedTo, _ := time.Parse(time.RFC3339, toDate)
				diff := parsedTo.Sub(now).Abs()
				assert.Less(t, diff, time.Second, "toDate should be approximately now")
			} else {
				assert.Equal(t, tt.to, toDate, "toDate should match provided value")
			}
		})
	}
}

func TestDefaultDateRange_RFC3339Format(t *testing.T) {
	// Arrange & Act
	fromDate, toDate := defaultDateRange("", "")

	// Assert
	// Verify both dates are valid RFC3339 format
	_, err := time.Parse(time.RFC3339, fromDate)
	require.NoError(t, err, "fromDate should be valid RFC3339")

	_, err = time.Parse(time.RFC3339, toDate)
	require.NoError(t, err, "toDate should be valid RFC3339")

	// Verify they contain timezone info (RFC3339 requires it)
	assert.Contains(t, fromDate, "Z", "fromDate should contain timezone indicator")
	assert.Contains(t, toDate, "Z", "toDate should contain timezone indicator")
}

func TestDefaultDateRange_DateRangeLogic(t *testing.T) {
	// Arrange
	now := time.Now().UTC()
	sixtyDaysAgo := now.AddDate(0, 0, -60)

	// Act
	fromDate, toDate := defaultDateRange("", "")

	// Assert
	parsedFrom, _ := time.Parse(time.RFC3339, fromDate)
	parsedTo, _ := time.Parse(time.RFC3339, toDate)

	// fromDate should be approximately 60 days before toDate
	daysDiff := parsedTo.Sub(parsedFrom).Hours() / 24
	assert.Greater(t, daysDiff, 59.0, "should be approximately 60 days apart")
	assert.Less(t, daysDiff, 61.0, "should be approximately 60 days apart")

	// fromDate should be close to 60 days ago
	assert.True(t, parsedFrom.After(sixtyDaysAgo.Add(-time.Minute)), "fromDate should be close to 60 days ago")
	assert.True(t, parsedFrom.Before(sixtyDaysAgo.Add(time.Minute)), "fromDate should be close to 60 days ago")
}
