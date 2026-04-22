package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/major/schwab-agent/internal/apperr"
)

func TestLoadConfig_MissingFile_ReturnsValidationError(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nonexistent.json")

	// Act
	cfg, err := LoadConfig(configPath)

	// Assert
	assert.Nil(t, cfg)
	require.Error(t, err)

	var valErr *apperr.ValidationError
	assert.ErrorAs(t, err, &valErr)
	assert.Contains(t, err.Error(), "SCHWAB_CLIENT_ID")
	assert.Contains(t, err.Error(), "SCHWAB_CLIENT_SECRET")
}

func TestLoadConfig_ExistingFile_LoadsValues(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := Config{
		ClientID:       "test-client-id",
		ClientSecret:   "test-secret",
		CallbackURL:    "https://example.com/callback",
		DefaultAccount: "account-123",
	}

	data, err := json.Marshal(configData)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0o600))

	// Act
	cfg, err := LoadConfig(configPath)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "test-client-id", cfg.ClientID)
	assert.Equal(t, "test-secret", cfg.ClientSecret)
	assert.Equal(t, "https://example.com/callback", cfg.CallbackURL)
	assert.Equal(t, "account-123", cfg.DefaultAccount)
}

func TestLoadConfig_EnvVarOverridesClientID(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := Config{
		ClientID:     "file-client-id",
		ClientSecret: "file-secret",
	}

	data, err := json.Marshal(configData)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0o600))

	t.Setenv("SCHWAB_CLIENT_ID", "env-client-id")

	// Act
	cfg, err := LoadConfig(configPath)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "env-client-id", cfg.ClientID)
	assert.Equal(t, "file-secret", cfg.ClientSecret)
}

func TestLoadConfig_EnvVarOverridesClientSecret(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := Config{
		ClientID:     "file-client-id",
		ClientSecret: "file-secret",
	}

	data, err := json.Marshal(configData)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0o600))

	t.Setenv("SCHWAB_CLIENT_SECRET", "env-secret")

	// Act
	cfg, err := LoadConfig(configPath)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "file-client-id", cfg.ClientID)
	assert.Equal(t, "env-secret", cfg.ClientSecret)
}

func TestLoadConfig_EnvVarOverridesCallbackURL(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := Config{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
		CallbackURL:  "https://file.example.com/callback",
	}

	data, err := json.Marshal(configData)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0o600))

	t.Setenv("SCHWAB_CALLBACK_URL", "https://env.example.com/callback")

	// Act
	cfg, err := LoadConfig(configPath)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "https://env.example.com/callback", cfg.CallbackURL)
}

func TestLoadConfig_DefaultCallbackURL(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := Config{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
	}

	data, err := json.Marshal(configData)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0o600))

	// Act
	cfg, err := LoadConfig(configPath)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "https://127.0.0.1:8182", cfg.CallbackURL)
}

func TestLoadConfig_MissingClientID_ReturnsValidationError(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := Config{
		ClientSecret: "test-secret",
	}

	data, err := json.Marshal(configData)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0o600))

	// Act
	cfg, err := LoadConfig(configPath)

	// Assert
	assert.Nil(t, cfg)
	require.Error(t, err)

	var valErr *apperr.ValidationError
	assert.ErrorAs(t, err, &valErr)
	assert.Contains(t, err.Error(), "SCHWAB_CLIENT_ID")
	assert.Contains(t, err.Error(), "config file")
}

func TestLoadConfig_MissingClientSecret_ReturnsValidationError(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := Config{
		ClientID: "test-id",
	}

	data, err := json.Marshal(configData)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0o600))

	// Act
	cfg, err := LoadConfig(configPath)

	// Assert
	assert.Nil(t, cfg)
	require.Error(t, err)

	var valErr *apperr.ValidationError
	assert.ErrorAs(t, err, &valErr)
	assert.Contains(t, err.Error(), "SCHWAB_CLIENT_SECRET")
	assert.Contains(t, err.Error(), "config file")
}

func TestLoadConfig_BothMissing_ReturnsValidationError(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := Config{}

	data, err := json.Marshal(configData)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0o600))

	// Act
	cfg, err := LoadConfig(configPath)

	// Assert
	assert.Nil(t, cfg)
	require.Error(t, err)

	var valErr *apperr.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

func TestLoadConfig_EnvVarPriority(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := Config{
		ClientID:     "file-id",
		ClientSecret: "file-secret",
		CallbackURL:  "https://file.example.com",
	}

	data, err := json.Marshal(configData)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0o600))

	t.Setenv("SCHWAB_CLIENT_ID", "env-id")
	t.Setenv("SCHWAB_CLIENT_SECRET", "env-secret")
	t.Setenv("SCHWAB_CALLBACK_URL", "https://env.example.com")

	// Act
	cfg, err := LoadConfig(configPath)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "env-id", cfg.ClientID)
	assert.Equal(t, "env-secret", cfg.ClientSecret)
	assert.Equal(t, "https://env.example.com", cfg.CallbackURL)
}

func TestSaveConfig_CreatesFileWithCorrectPermissions(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	cfg := &Config{
		ClientID:       "test-id",
		ClientSecret:   "test-secret",
		CallbackURL:    "https://example.com",
		DefaultAccount: "account-123",
	}

	// Act
	err := SaveConfig(configPath, cfg)

	// Assert
	require.NoError(t, err)

	// Verify file exists
	assert.FileExists(t, configPath)

	// Verify file permissions are 0600
	info, err := os.Stat(configPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	// Verify content
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var loaded Config
	err = json.Unmarshal(data, &loaded)
	require.NoError(t, err)

	assert.Equal(t, cfg.ClientID, loaded.ClientID)
	assert.Equal(t, cfg.ClientSecret, loaded.ClientSecret)
	assert.Equal(t, cfg.CallbackURL, loaded.CallbackURL)
	assert.Equal(t, cfg.DefaultAccount, loaded.DefaultAccount)
}

func TestSaveConfig_CreatesParentDirWithCorrectPermissions(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "subdir", "config.json")

	cfg := &Config{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
	}

	// Act
	err := SaveConfig(configPath, cfg)

	// Assert
	require.NoError(t, err)

	// Verify parent directory exists
	parentDir := filepath.Dir(configPath)
	assert.DirExists(t, parentDir)

	// Verify parent directory permissions are 0700
	info, err := os.Stat(parentDir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())

	// Verify file exists
	assert.FileExists(t, configPath)
}

func TestSaveConfig_OverwritesExistingFile(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	oldCfg := &Config{
		ClientID:     "old-id",
		ClientSecret: "old-secret",
	}

	err := SaveConfig(configPath, oldCfg)
	require.NoError(t, err)

	newCfg := &Config{
		ClientID:     "new-id",
		ClientSecret: "new-secret",
	}

	// Act
	err = SaveConfig(configPath, newCfg)

	// Assert
	require.NoError(t, err)

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var loaded Config
	err = json.Unmarshal(data, &loaded)
	require.NoError(t, err)

	assert.Equal(t, "new-id", loaded.ClientID)
	assert.Equal(t, "new-secret", loaded.ClientSecret)
}

func TestSetDefaultAccount_LoadsConfigSetsAndSaves(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	initialCfg := &Config{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
	}

	err := SaveConfig(configPath, initialCfg)
	require.NoError(t, err)

	// Act
	err = SetDefaultAccount(configPath, "new-account-hash")

	// Assert
	require.NoError(t, err)

	// Verify the config was updated
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var loaded Config
	err = json.Unmarshal(data, &loaded)
	require.NoError(t, err)

	assert.Equal(t, "new-account-hash", loaded.DefaultAccount)
	assert.Equal(t, "test-id", loaded.ClientID)
	assert.Equal(t, "test-secret", loaded.ClientSecret)
}

func TestSetDefaultAccount_CreatesConfigIfMissing(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Act
	err := SetDefaultAccount(configPath, "account-hash")

	// Assert
	require.NoError(t, err)

	// Verify file was created
	assert.FileExists(t, configPath)

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var loaded Config
	err = json.Unmarshal(data, &loaded)
	require.NoError(t, err)

	assert.Equal(t, "account-hash", loaded.DefaultAccount)
}

func TestLoadConfig_InvalidJSON_ReturnsError(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	require.NoError(t, os.WriteFile(configPath, []byte("invalid json"), 0o600))

	// Act
	cfg, err := LoadConfig(configPath)

	// Assert
	assert.Nil(t, cfg)
	require.Error(t, err)
}

func TestLoadConfig_EmptyFile_ReturnsValidationError(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	require.NoError(t, os.WriteFile(configPath, []byte("{}"), 0o600))

	// Act
	cfg, err := LoadConfig(configPath)

	// Assert
	assert.Nil(t, cfg)
	require.Error(t, err)

	var valErr *apperr.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

func TestLoadConfig_IAlsoLikeToLiveDangerously_DefaultsFalse(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configData := Config{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
	}

	data, err := json.Marshal(configData)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0o600))

	// Act
	cfg, err := LoadConfig(configPath)

	// Assert
	require.NoError(t, err)
	assert.False(t, cfg.IAlsoLikeToLiveDangerously)
}

func TestLoadConfig_IAlsoLikeToLiveDangerously_LoadsTrue(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	rawJSON := `{"client_id":"test-id","client_secret":"test-secret","i-also-like-to-live-dangerously":true}`
	require.NoError(t, os.WriteFile(configPath, []byte(rawJSON), 0o600))

	// Act
	cfg, err := LoadConfig(configPath)

	// Assert
	require.NoError(t, err)
	assert.True(t, cfg.IAlsoLikeToLiveDangerously)
}

func TestLoadConfig_IAlsoLikeToLiveDangerously_LoadsFalse(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	rawJSON := `{"client_id":"test-id","client_secret":"test-secret","i-also-like-to-live-dangerously":false}`
	require.NoError(t, os.WriteFile(configPath, []byte(rawJSON), 0o600))

	// Act
	cfg, err := LoadConfig(configPath)

	// Assert
	require.NoError(t, err)
	assert.False(t, cfg.IAlsoLikeToLiveDangerously)
}

func TestSaveConfig_MkdirAllFailure(t *testing.T) {
	// Arrange: use a path under /dev/null which is not a directory,
	// so MkdirAll will fail trying to create the parent.
	badPath := "/dev/null/impossible/config.json"
	cfg := &Config{ClientID: "id", ClientSecret: "secret"}

	// Act
	err := SaveConfig(badPath, cfg)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create config directory")
}

func TestSaveConfig_WriteFileFailure(t *testing.T) {
	// Arrange: create a read-only directory so WriteFile fails.
	tmpDir := t.TempDir()
	readOnlyDir := filepath.Join(tmpDir, "readonly")
	require.NoError(t, os.MkdirAll(readOnlyDir, 0o500))
	configPath := filepath.Join(readOnlyDir, "config.json")
	cfg := &Config{ClientID: "id", ClientSecret: "secret"}

	// Act
	err := SaveConfig(configPath, cfg)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write config file")
}

func TestSaveConfig_PreservesIAlsoLikeToLiveDangerously(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	cfg := &Config{
		ClientID:                   "test-id",
		ClientSecret:               "test-secret",
		IAlsoLikeToLiveDangerously: true,
	}

	// Act
	err := SaveConfig(configPath, cfg)

	// Assert
	require.NoError(t, err)

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var loaded Config
	err = json.Unmarshal(data, &loaded)
	require.NoError(t, err)

	assert.True(t, loaded.IAlsoLikeToLiveDangerously)
}
