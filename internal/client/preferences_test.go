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
	"github.com/major/schwab-agent/internal/ptr"
)

func TestUserPreference_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/trader/v1/userPreference", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		response := models.UserPreference{
			Accounts: []models.UserPreferenceAccount{
				{
					AccountNumber:      ptr.To("123456789"),
					PrimaryAccount:     ptr.To(true),
					Type:                ptr.To("MARGIN"),
					NickName:           ptr.To("Main Account"),
					AccountColor:       ptr.To("COLOR_BLUE"),
					DisplayAcctId:      ptr.To("X123"),
					AutoPositionEffect: ptr.To(true),
				},
				{
					AccountNumber:      ptr.To("987654321"),
					PrimaryAccount:     ptr.To(false),
					Type:                ptr.To("CASH"),
					NickName:           ptr.To("Savings Account"),
					AccountColor:       ptr.To("COLOR_GREEN"),
					DisplayAcctId:      ptr.To("X456"),
					AutoPositionEffect: ptr.To(false),
				},
			},
			Offers: []models.Offer{
				{
					ID:          ptr.To("offer-001"),
					Name:        ptr.To("Premium Features"),
					Description: ptr.To("Access to advanced tools"),
					Status:      ptr.To("ACTIVE"),
				},
			},
			StreamerInfo: []models.StreamerInfo{
				{
					StreamerURL: ptr.To("https://streamer.schwab.com"),
					Token:       ptr.To("streamer-token-xyz"),
					TokenExpTime: ptr.To(int64(1705363200)),
					AppID:       ptr.To("app-123"),
					ACL:         ptr.To("ACCT"),
				},
			},
		}
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.UserPreference(context.Background())

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Accounts, 2)
	assert.Equal(t, "123456789", *result.Accounts[0].AccountNumber)
	assert.True(t, *result.Accounts[0].PrimaryAccount)
	assert.Equal(t, "Main Account", *result.Accounts[0].NickName)
	assert.Equal(t, "987654321", *result.Accounts[1].AccountNumber)
	assert.False(t, *result.Accounts[1].PrimaryAccount)
	require.Len(t, result.Offers, 1)
	assert.Equal(t, "Premium Features", *result.Offers[0].Name)
	require.Len(t, result.StreamerInfo, 1)
	assert.Equal(t, "streamer-token-xyz", *result.StreamerInfo[0].Token)
}

func TestUserPreference_EmptyAccounts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := models.UserPreference{
			Accounts:     []models.UserPreferenceAccount{},
			Offers:       []models.Offer{},
			StreamerInfo: []models.StreamerInfo{},
		}
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.UserPreference(context.Background())

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.Accounts)
	assert.Empty(t, result.Offers)
	assert.Empty(t, result.StreamerInfo)
}

func TestUserPreference_WithMinimalData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := models.UserPreference{
			Accounts: []models.UserPreferenceAccount{
				{
					AccountNumber: ptr.To("123456789"),
				},
			},
		}
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.UserPreference(context.Background())

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Accounts, 1)
	assert.Equal(t, "123456789", *result.Accounts[0].AccountNumber)
	assert.Nil(t, result.Accounts[0].PrimaryAccount)
	assert.Nil(t, result.Accounts[0].NickName)
}

func TestUserPreference_BearerTokenAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer my-secret-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(models.UserPreference{}))
	}))
	defer srv.Close()

	c := NewClient("my-secret-token", WithBaseURL(srv.URL))
	_, err := c.UserPreference(context.Background())

	require.NoError(t, err)
}

func TestUserPreference_WithComplexStreamerInfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := models.UserPreference{
			Accounts: []models.UserPreferenceAccount{
				{
					AccountNumber:      ptr.To("123456789"),
					PrimaryAccount:     ptr.To(true),
					Type:                ptr.To("MARGIN"),
					NickName:           ptr.To("Trading Account"),
					AccountColor:       ptr.To("COLOR_RED"),
					DisplayAcctId:      ptr.To("X789"),
					AutoPositionEffect: ptr.To(true),
				},
			},
			StreamerInfo: []models.StreamerInfo{
				{
					StreamerURL:  ptr.To("https://streamer1.schwab.com"),
					Token:        ptr.To("token-abc123"),
					TokenExpTime: ptr.To(int64(1705363200)),
					AppID:        ptr.To("app-001"),
					ACL:          ptr.To("ACCT,QUOTE"),
				},
				{
					StreamerURL:  ptr.To("https://streamer2.schwab.com"),
					Token:        ptr.To("token-def456"),
					TokenExpTime: ptr.To(int64(1705449600)),
					AppID:        ptr.To("app-002"),
					ACL:          ptr.To("ACCT"),
				},
			},
		}
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.UserPreference(context.Background())

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.StreamerInfo, 2)
	assert.Equal(t, "token-abc123", *result.StreamerInfo[0].Token)
	assert.Equal(t, int64(1705363200), *result.StreamerInfo[0].TokenExpTime)
	assert.Equal(t, "token-def456", *result.StreamerInfo[1].Token)
	assert.Equal(t, int64(1705449600), *result.StreamerInfo[1].TokenExpTime)
}
