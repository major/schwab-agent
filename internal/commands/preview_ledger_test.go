package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-agent/internal/models"
	"github.com/major/schwab-agent/internal/orderbuilder"
)

func testPreviewLedgerOrder(t *testing.T) *models.OrderRequest {
	t.Helper()

	order, err := orderbuilder.BuildEquityOrder(&orderbuilder.EquityParams{
		Symbol:    "AAPL",
		Action:    models.InstructionBuy,
		Quantity:  10,
		OrderType: models.OrderTypeLimit,
		Price:     185.25,
	})
	require.NoError(t, err)
	return order
}

func testSavedPreviewEntry(t *testing.T, stateDir, digest string) savedOrderPreview {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(stateDir, "previews", digest+".json"))
	require.NoError(t, err)

	var entry savedOrderPreview
	require.NoError(t, json.Unmarshal(data, &entry))
	return entry
}

func writeSavedPreviewEntry(t *testing.T, stateDir string, entry *savedOrderPreview) {
	t.Helper()

	data, err := json.MarshalIndent(&entry, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, "previews", entry.Digest+".json"), data, 0o600))
}

func TestPreviewLedgerRejectsUnsafeDigestInput(t *testing.T) {
	t.Setenv("SCHWAB_AGENT_STATE_DIR", t.TempDir())

	for _, digest := range []string{
		"",
		"../" + strings.Repeat("a", 64),
		strings.Repeat("A", 64),
		strings.Repeat("g", 64),
		strings.Repeat("a", 63),
	} {
		t.Run(digest, func(t *testing.T) {
			_, err := loadOrderPreview(digest)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "64-character lowercase SHA-256")
		})
	}
}

func TestPreviewLedgerRejectsMissingDigest(t *testing.T) {
	t.Setenv("SCHWAB_AGENT_STATE_DIR", t.TempDir())

	_, err := loadOrderPreview(strings.Repeat("a", 64))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestPreviewLedgerRejectsTamperedOrderPayload(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("SCHWAB_AGENT_STATE_DIR", stateDir)

	digestData, err := saveOrderPreview("hash123", testPreviewLedgerOrder(t), nil)
	require.NoError(t, err)
	entry := testSavedPreviewEntry(t, stateDir, digestData.Digest)
	entry.Order.OrderLegCollection[0].Instrument.Symbol = "MSFT"
	writeSavedPreviewEntry(t, stateDir, &entry)

	_, err = loadOrderPreview(digestData.Digest)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no longer matches")
}

func TestPreviewLedgerRejectsTamperedCanonicalOrder(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("SCHWAB_AGENT_STATE_DIR", stateDir)

	digestData, err := saveOrderPreview("hash123", testPreviewLedgerOrder(t), nil)
	require.NoError(t, err)
	entry := testSavedPreviewEntry(t, stateDir, digestData.Digest)
	entry.CanonicalOrder = json.RawMessage(`{"tampered":true}`)
	writeSavedPreviewEntry(t, stateDir, &entry)

	_, err = loadOrderPreview(digestData.Digest)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no longer matches")
}

func TestPreviewLedgerRejectsTamperedSafetyCheck(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("SCHWAB_AGENT_STATE_DIR", stateDir)

	digestData, err := saveOrderPreview(
		"hash123",
		testPreviewLedgerOrder(t),
		nil,
		previewSafetyCheck{Type: previewSafetyCoveredCall, Underlying: "F", Contracts: 1},
	)
	require.NoError(t, err)
	entry := testSavedPreviewEntry(t, stateDir, digestData.Digest)
	entry.SafetyCheck = nil
	writeSavedPreviewEntry(t, stateDir, &entry)

	_, err = loadOrderPreview(digestData.Digest)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no longer matches")
}

func TestPreviewLedgerRejectsExpiredEntry(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("SCHWAB_AGENT_STATE_DIR", stateDir)

	digestData, err := saveOrderPreview("hash123", testPreviewLedgerOrder(t), nil)
	require.NoError(t, err)
	entry := testSavedPreviewEntry(t, stateDir, digestData.Digest)
	entry.ExpiresAt = time.Now().UTC().Add(-time.Minute)
	writeSavedPreviewEntry(t, stateDir, &entry)

	_, err = loadOrderPreview(digestData.Digest)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

func TestPreviewLedgerSecuresExistingDirectory(t *testing.T) {
	stateDir := t.TempDir()
	ledgerDir := filepath.Join(stateDir, "previews")
	require.NoError(t, os.MkdirAll(ledgerDir, 0o755))
	t.Setenv("SCHWAB_AGENT_STATE_DIR", stateDir)

	digestData, err := saveOrderPreview("hash123", testPreviewLedgerOrder(t), nil)
	require.NoError(t, err)

	dirInfo, err := os.Stat(ledgerDir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), dirInfo.Mode().Perm())

	fileInfo, err := os.Stat(filepath.Join(ledgerDir, digestData.Digest+".json"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), fileInfo.Mode().Perm())
}
