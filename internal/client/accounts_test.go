package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/models"
)

func TestAccountNumbers_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/trader/v1/accounts/accountNumbers", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		response := []models.AccountNumber{
			{AccountNumber: "123456789", HashValue: "abc123def456"},
			{AccountNumber: "987654321", HashValue: "xyz789uvw012"},
		}
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.AccountNumbers(context.Background())

	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, "123456789", result[0].AccountNumber)
	assert.Equal(t, "abc123def456", result[0].HashValue)
	assert.Equal(t, "987654321", result[1].AccountNumber)
	assert.Equal(t, "xyz789uvw012", result[1].HashValue)
}

func TestAccountNumbers_404_ReturnsAccountNotFoundError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.AccountNumbers(context.Background())

	require.Error(t, err)
	assert.Nil(t, result)

	var accountErr *apperr.AccountNotFoundError
	require.ErrorAs(t, err, &accountErr)
	assert.Contains(t, accountErr.Error(), "account numbers not found")
}

func TestAccountNumbers_EmptyList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.AccountNumbers(context.Background())

	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestAccounts_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/trader/v1/accounts", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		response := []models.Account{
			{
				SecuritiesAccount: &models.SecuritiesAccount{
					Type:          new("MARGIN"),
					AccountNumber: new("123456789"),
				},
			},
			{
				SecuritiesAccount: &models.SecuritiesAccount{
					Type:          new("CASH"),
					AccountNumber: new("987654321"),
				},
			},
		}
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.Accounts(context.Background())

	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, "MARGIN", *result[0].SecuritiesAccount.Type)
	assert.Equal(t, "123456789", *result[0].SecuritiesAccount.AccountNumber)
	assert.Equal(t, "CASH", *result[1].SecuritiesAccount.Type)
	assert.Equal(t, "987654321", *result[1].SecuritiesAccount.AccountNumber)
}

func TestAccounts_404_ReturnsAccountNotFoundError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.Accounts(context.Background())

	require.Error(t, err)
	assert.Nil(t, result)

	var accountErr *apperr.AccountNotFoundError
	require.ErrorAs(t, err, &accountErr)
	assert.Contains(t, accountErr.Error(), "accounts not found")
}

func TestAccounts_EmptyList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.Accounts(context.Background())

	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestAccount_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/trader/v1/accounts/abc123def456", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		response := models.Account{
			SecuritiesAccount: &models.SecuritiesAccount{
				Type:          new("MARGIN"),
				AccountNumber: new("123456789"),
			},
		}
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.Account(context.Background(), "abc123def456")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "MARGIN", *result.SecuritiesAccount.Type)
	assert.Equal(t, "123456789", *result.SecuritiesAccount.AccountNumber)
}

func TestAccount_404_ReturnsAccountNotFoundError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"account not found"}`))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.Account(context.Background(), "invalid-hash")

	require.Error(t, err)
	assert.Nil(t, result)

	var accountErr *apperr.AccountNotFoundError
	require.ErrorAs(t, err, &accountErr)
	assert.Contains(t, accountErr.Error(), "account invalid-hash not found")
}

func TestAccounts_WithPositionsField(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the fields query parameter is sent to the API.
		assert.Equal(t, "positions", r.URL.Query().Get("fields"))

		w.Header().Set("Content-Type", "application/json")
		response := []models.Account{
			{
				SecuritiesAccount: &models.SecuritiesAccount{
					Type:          new("MARGIN"),
					AccountNumber: new("123456789"),
					Positions: []models.Position{
						{
							LongQuantity: new(float64(100)),
							MarketValue:  new(15000.00),
							Instrument: &models.AccountsInstrument{
								Symbol:    new("AAPL"),
								AssetType: new("EQUITY"),
							},
						},
					},
				},
			},
		}
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.Accounts(context.Background(), "positions")

	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Len(t, result[0].SecuritiesAccount.Positions, 1)
	assert.Equal(t, "AAPL", *result[0].SecuritiesAccount.Positions[0].Instrument.Symbol)
	assert.Equal(t, 100.0, *result[0].SecuritiesAccount.Positions[0].LongQuantity)
}

func TestAccounts_WithoutFields_NoQueryParam(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No fields param should be sent when none are requested.
		assert.Empty(t, r.URL.Query().Get("fields"))

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode([]models.Account{}))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	_, err := c.Accounts(context.Background())

	require.NoError(t, err)
}

func TestAccount_WithPositionsField(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "positions", r.URL.Query().Get("fields"))
		assert.Equal(t, "/trader/v1/accounts/hash123", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		response := models.Account{
			SecuritiesAccount: &models.SecuritiesAccount{
				Type:          new("MARGIN"),
				AccountNumber: new("123456789"),
				Positions: []models.Position{
					{
						LongQuantity:  new(float64(50)),
						ShortQuantity: new(float64(0)),
						MarketValue:   new(8500.00),
						Instrument: &models.AccountsInstrument{
							Symbol:    new("MSFT"),
							AssetType: new("EQUITY"),
						},
					},
				},
			},
		}
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.Account(context.Background(), "hash123", "positions")

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.SecuritiesAccount.Positions, 1)
	assert.Equal(t, "MSFT", *result.SecuritiesAccount.Positions[0].Instrument.Symbol)
	assert.Equal(t, 50.0, *result.SecuritiesAccount.Positions[0].LongQuantity)
}

func TestAccount_WithComplexData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := models.Account{
			SecuritiesAccount: &models.SecuritiesAccount{
				Type:          new("MARGIN"),
				AccountNumber: new("123456789"),
				RoundTrips:    new(5),
				IsForeign:     new(false),
				CurrentBalances: &models.MarginBalance{
					CashBalance:      new(50000.00),
					BuyingPower:      new(100000.00),
					EquityPercentage: new(0.75),
				},
				Positions: []models.Position{
					{
						LongQuantity: new(float64(100)),
						MarketValue:  new(15000.00),
						Instrument: &models.AccountsInstrument{
							Symbol:    new("AAPL"),
							AssetType: new("EQUITY"),
						},
					},
				},
			},
		}
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.Account(context.Background(), "hash123")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "MARGIN", *result.SecuritiesAccount.Type)
	assert.Equal(t, 5, *result.SecuritiesAccount.RoundTrips)
	assert.False(t, *result.SecuritiesAccount.IsForeign)
	assert.Equal(t, 50000.00, *result.SecuritiesAccount.CurrentBalances.CashBalance)
	assert.Equal(t, 100000.00, *result.SecuritiesAccount.CurrentBalances.BuyingPower)
	require.Len(t, result.SecuritiesAccount.Positions, 1)
	assert.Equal(t, "AAPL", *result.SecuritiesAccount.Positions[0].Instrument.Symbol)
	assert.Equal(t, 100.0, *result.SecuritiesAccount.Positions[0].LongQuantity)
}

func TestAccount_401_ReturnsAuthExpiredError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"token expired"}`))
	}))
	defer srv.Close()

	c := NewClient("bad-token", WithBaseURL(srv.URL))
	result, err := c.Account(context.Background(), "hash123")

	require.Error(t, err)
	assert.Nil(t, result)

	var authErr *apperr.AuthExpiredError
	require.ErrorAs(t, err, &authErr)
}

func TestAccountNumbers_BearerTokenAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer my-secret-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode([]models.AccountNumber{}))
	}))
	defer srv.Close()

	c := NewClient("my-secret-token", WithBaseURL(srv.URL))
	_, err := c.AccountNumbers(context.Background())

	require.NoError(t, err)
}

func TestAccounts_BearerTokenAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer my-secret-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode([]models.Account{}))
	}))
	defer srv.Close()

	c := NewClient("my-secret-token", WithBaseURL(srv.URL))
	_, err := c.Accounts(context.Background())

	require.NoError(t, err)
}

func TestAccount_BearerTokenAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer my-secret-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(models.Account{}))
	}))
	defer srv.Close()

	c := NewClient("my-secret-token", WithBaseURL(srv.URL))
	_, err := c.Account(context.Background(), "hash123")

	require.NoError(t, err)
}
