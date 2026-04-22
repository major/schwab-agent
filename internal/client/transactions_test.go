package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-agent/internal/models"
)

func TestTransactions_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/trader/v1/accounts/abc123def456/transactions", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		// Default startDate and endDate should always be present.
		query := r.URL.Query()
		assert.NotEmpty(t, query.Get("startDate"), "startDate should be set by default")
		assert.NotEmpty(t, query.Get("endDate"), "endDate should be set by default")

		w.Header().Set("Content-Type", "application/json")
		response := []models.Transaction{
			{
				ActivityID:  int64Ptr(1001),
				Time:        strPtr("2024-01-15T10:30:00Z"),
				Type:        transactionTypePtr(models.TransactionTypeTrade),
				Status:      strPtr("FILLED"),
				Description: strPtr("BUY 100 AAPL"),
				NetAmount:   floatPtr(-15000.00),
			},
			{
				ActivityID:  int64Ptr(1002),
				Time:        strPtr("2024-01-16T14:45:00Z"),
				Type:        transactionTypePtr(models.TransactionTypeDividend),
				Status:      strPtr("SETTLED"),
				Description: strPtr("DIVIDEND AAPL"),
				NetAmount:   floatPtr(50.00),
			},
		}
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.Transactions(context.Background(), "abc123def456", TransactionParams{})

	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, int64(1001), *result[0].ActivityID)
	assert.Equal(t, "BUY 100 AAPL", *result[0].Description)
	assert.Equal(t, int64(1002), *result[1].ActivityID)
	assert.Equal(t, "DIVIDEND AAPL", *result[1].Description)
}

func TestTransactions_WithTypeFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/trader/v1/accounts/abc123def456/transactions", r.URL.Path)

		// Verify query parameters are passed
		query := r.URL.Query()
		assert.Equal(t, "TRADE", query.Get("types"))

		w.Header().Set("Content-Type", "application/json")
		response := []models.Transaction{
			{
				ActivityID:  int64Ptr(1001),
				Type:        transactionTypePtr(models.TransactionTypeTrade),
				Description: strPtr("BUY 100 AAPL"),
			},
		}
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.Transactions(context.Background(), "abc123def456", TransactionParams{
		Types: "TRADE",
	})

	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "BUY 100 AAPL", *result[0].Description)
}

func TestTransactions_WithDateFilters(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)

		// Verify query parameters are passed
		query := r.URL.Query()
		assert.Equal(t, "2024-01-01", query.Get("startDate"))
		assert.Equal(t, "2024-01-31", query.Get("endDate"))

		w.Header().Set("Content-Type", "application/json")
		response := []models.Transaction{
			{
				ActivityID:  int64Ptr(1001),
				Time:        strPtr("2024-01-15T10:30:00Z"),
				Description: strPtr("BUY 100 AAPL"),
			},
		}
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.Transactions(context.Background(), "abc123def456", TransactionParams{
		StartDate: "2024-01-01",
		EndDate:   "2024-01-31",
	})

	require.NoError(t, err)
	require.Len(t, result, 1)
}

func TestTransactions_WithAllFilters(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		assert.Equal(t, "TRADE", query.Get("types"))
		assert.Equal(t, "2024-01-01", query.Get("startDate"))
		assert.Equal(t, "2024-01-31", query.Get("endDate"))

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode([]models.Transaction{}))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	_, err := c.Transactions(context.Background(), "abc123def456", TransactionParams{
		Types:     "TRADE",
		StartDate: "2024-01-01",
		EndDate:   "2024-01-31",
	})

	require.NoError(t, err)
}

func TestTransactions_EmptyList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.Transactions(context.Background(), "abc123def456", TransactionParams{})

	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestTransaction_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/trader/v1/accounts/abc123def456/transactions/1001", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		response := models.Transaction{
			ActivityID:  int64Ptr(1001),
			Time:        strPtr("2024-01-15T10:30:00Z"),
			Type:        transactionTypePtr(models.TransactionTypeTrade),
			Status:      strPtr("FILLED"),
			Description: strPtr("BUY 100 AAPL"),
			NetAmount:   floatPtr(-15000.00),
		}
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.Transaction(context.Background(), "abc123def456", 1001)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int64(1001), *result.ActivityID)
	assert.Equal(t, "BUY 100 AAPL", *result.Description)
	assert.Equal(t, -15000.00, *result.NetAmount)
}

func TestTransaction_WithComplexData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := models.Transaction{
			ActivityID:     int64Ptr(1001),
			Time:           strPtr("2024-01-15T10:30:00Z"),
			Type:           transactionTypePtr(models.TransactionTypeTrade),
			Status:         strPtr("FILLED"),
			Description:    strPtr("BUY 100 AAPL"),
			NetAmount:      floatPtr(-15000.00),
			AccountNumber:  strPtr("123456789"),
			SubAccount:     strPtr("CASH"),
			ActivityType:   strPtr("ORDER"),
			OrderID:        int64Ptr(5001),
			PositionID:     int64Ptr(9001),
			TradeDate:      strPtr("2024-01-15"),
			SettlementDate: strPtr("2024-01-17"),
			TransferItems: []models.TransferItem{
				{
					Instrument: &models.TransferInstrument{
						AssetType:   strPtr("EQUITY"),
						Symbol:      strPtr("AAPL"),
						Description: strPtr("Apple Inc"),
					},
					Position:         floatPtr(100),
					PositionEffect:   strPtr("OPENING"),
					PositionQuantity: floatPtr(100),
				},
			},
		}
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.Transaction(context.Background(), "abc123def456", 1001)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "123456789", *result.AccountNumber)
	assert.Equal(t, int64(5001), *result.OrderID)
	assert.Equal(t, "2024-01-17", *result.SettlementDate)
	require.Len(t, result.TransferItems, 1)
	assert.Equal(t, "AAPL", *result.TransferItems[0].Instrument.Symbol)
	assert.Equal(t, 100.0, *result.TransferItems[0].Position)
}

func TestTransaction_BearerTokenAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer my-secret-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(models.Transaction{}))
	}))
	defer srv.Close()

	c := NewClient("my-secret-token", WithBaseURL(srv.URL))
	_, err := c.Transaction(context.Background(), "hash123", 1001)

	require.NoError(t, err)
}

func TestTransactions_BearerTokenAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer my-secret-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode([]models.Transaction{}))
	}))
	defer srv.Close()

	c := NewClient("my-secret-token", WithBaseURL(srv.URL))
	_, err := c.Transactions(context.Background(), "hash123", TransactionParams{})

	require.NoError(t, err)
}

// Helper function for TransactionType pointer creation
func transactionTypePtr(t models.TransactionType) *models.TransactionType {
	return &t
}
