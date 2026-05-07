package commands

import (
	"bytes"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/auth"
	"github.com/major/schwab-agent/internal/client"
)

func TestRootAuthPathsUsesDefaultsForEmptyFlags(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	root.Flags().String("config", "", "config path")
	root.Flags().String("token", "", "token path")

	configPath, tokenPath, err := rootAuthConfig{
		DefaultConfigPath: "default-config",
		DefaultTokenPath:  "default-token",
	}.authPaths(root)

	require.NoError(t, err)
	assert.Equal(t, "default-config", configPath)
	assert.Equal(t, "default-token", tokenPath)
}

func TestCompleteRootDepsFillsMissingDependencies(t *testing.T) {
	newClientCalled := false
	deps := completeRootDeps(RootDeps{
		NewClient: func(token string, opts ...client.Option) *client.Client {
			newClientCalled = true
			return client.NewClient(token, opts...)
		},
	})

	require.NotNil(t, deps.NewClient)
	require.NotNil(t, deps.TokenRefreshEndpoint)
	deps.NewClient("token")
	assert.True(t, newClientCalled)
}

func TestHasSkipAuthAnnotationChecksAncestors(t *testing.T) {
	root := &cobra.Command{
		Use:         "root",
		Annotations: map[string]string{annotationSkipAuth: annotationValueTrue},
	}
	child := &cobra.Command{Use: "child"}
	grandchild := &cobra.Command{Use: "grandchild"}
	root.AddCommand(child)
	child.AddCommand(grandchild)

	assert.True(t, hasSkipAuthAnnotation(grandchild))
	assert.False(t, hasSkipAuthAnnotation(&cobra.Command{Use: "plain"}))
}

func TestNewRootCmdCompletesDependenciesAndPostRunClosesClient(t *testing.T) {
	ref := &client.Ref{Client: client.NewClient("token")}
	root := NewRootCmd(ref, "config", "token", "test", &bytes.Buffer{}, RootDeps{})

	require.NotNil(t, root.PersistentPreRunE)
	require.NotNil(t, root.PersistentPostRunE)
	require.NoError(t, root.PersistentPostRunE(root, nil))
}

func TestDefaultRootDeps(t *testing.T) {
	deps := DefaultRootDeps()

	require.NotNil(t, deps.NewClient)
	require.NotNil(t, deps.TokenRefreshEndpoint)
	assert.Equal(t, "https://api.schwabapi.com/v1/oauth/token", deps.TokenRefreshEndpoint(&auth.Config{}))
}

func TestRootPreRunSkipsAuthForAnnotatedCommands(t *testing.T) {
	ref := &client.Ref{}
	cmd := &cobra.Command{
		Use:         "env-vars",
		Annotations: map[string]string{annotationSkipAuth: annotationValueTrue},
	}

	err := rootAuthConfig{Ref: ref}.preRun(cmd, nil)

	require.NoError(t, err)
	assert.Nil(t, ref.Client)
}

func TestRootPreRunLoadsAuthAndInitializesClients(t *testing.T) {
	configPath, tokenPath := writeRootAuthFiles(t)
	root := rootCommandWithAuthFlags(t, configPath, tokenPath)
	ref := &client.Ref{}
	var capturedToken string
	var capturedClient *client.Client

	err := rootAuthConfig{
		Version: "test-version",
		Deps: RootDeps{
			NewClient: func(token string, opts ...client.Option) *client.Client {
				capturedToken = token
				capturedClient = client.NewClient(token, opts...)
				return capturedClient
			},
			TokenRefreshEndpoint: func(cfg *auth.Config) string {
				return cfg.OAuthTokenURL()
			},
		},
		Ref: ref,
	}.preRun(root, nil)

	require.NoError(t, err)
	assert.Equal(t, "root-access-token", capturedToken)
	assert.Same(t, capturedClient, ref.Client)
	assert.NotNil(t, ref.MarketData)
}

func TestRootPreRunReturnsVerboseFlagError(t *testing.T) {
	cmd := &cobra.Command{Use: "root"}

	err := rootAuthConfig{Ref: &client.Ref{}}.preRun(cmd, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "verbose")
}

func TestLoadAuthFilesMapsMissingInputs(t *testing.T) {
	t.Setenv("SCHWAB_CLIENT_ID", "")
	t.Setenv("SCHWAB_CLIENT_SECRET", "")
	t.Setenv("SCHWAB_CALLBACK_URL", "")
	t.Setenv("SCHWAB_BASE_URL", "")
	t.Setenv("SCHWAB_BASE_URL_INSECURE", "false")

	t.Run("missing credentials", func(t *testing.T) {
		_, _, err := loadAuthFiles(filepath.Join(t.TempDir(), "missing-config.json"), "missing-token.json")

		require.Error(t, err)
		var authRequired *apperr.AuthRequiredError
		require.ErrorAs(t, err, &authRequired)
		assert.Contains(t, err.Error(), "Missing required credentials")
	})

	t.Run("missing token", func(t *testing.T) {
		configPath := filepath.Join(t.TempDir(), "config.json")
		require.NoError(t, auth.SaveConfig(configPath, &auth.Config{
			ClientID:     "client-id",
			ClientSecret: "client-secret",
		}))

		_, _, err := loadAuthFiles(configPath, filepath.Join(t.TempDir(), "missing-token.json"))

		require.Error(t, err)
		var authRequired *apperr.AuthRequiredError
		require.ErrorAs(t, err, &authRequired)
		assert.Contains(t, err.Error(), "No authentication token found")
	})
}

func TestRootClientOptionsAcceptsNilTLSConfig(t *testing.T) {
	opts := rootClientOptions(&auth.Config{BaseURL: "https://proxy.example.test/api"}, "1.2.3")

	assert.Len(t, opts, 3)
}

func rootCommandWithAuthFlags(t *testing.T, configPath, tokenPath string) *cobra.Command {
	t.Helper()

	cmd := &cobra.Command{Use: "root"}
	cmd.Flags().Bool("verbose", true, "verbose logging")
	cmd.Flags().String("config", configPath, "config path")
	cmd.Flags().String("token", tokenPath, "token path")
	return cmd
}

func writeRootAuthFiles(t *testing.T) (string, string) {
	t.Helper()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	tokenPath := filepath.Join(dir, "token.json")
	require.NoError(t, auth.SaveConfig(configPath, &auth.Config{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		BaseURL:      "https://api.schwabapi.com",
	}))
	require.NoError(t, auth.SaveToken(tokenPath, &auth.TokenFile{
		CreationTimestamp: time.Now().Add(-1 * time.Hour).Unix(),
		Token: auth.TokenData{
			AccessToken:  "root-access-token",
			TokenType:    "Bearer",
			ExpiresIn:    1800,
			RefreshToken: "root-refresh-token",
			Scope:        "api",
			ExpiresAt:    time.Now().Add(30 * time.Minute).Unix(),
		},
	}))

	return configPath, tokenPath
}
