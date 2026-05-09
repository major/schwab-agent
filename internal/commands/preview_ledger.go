package commands

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/models"
)

const (
	previewLedgerSchemaVersion = 1
	previewLedgerOperation     = "order.place"
	previewLedgerEndpoint      = "/trader/v1/accounts/{accountHash}/orders"
	previewLedgerTTL           = 15 * time.Minute
)

// previewDigestData is the small machine-readable reference returned from
// `order preview --save-preview`. The digest is intentionally just a local
// safety reference, not an authorization token or Schwab-side reservation.
type previewDigestData struct {
	Digest    string `json:"digest"`
	ExpiresAt string `json:"expiresAt"`
	Account   string `json:"account"`
	Operation string `json:"operation"`
}

// savedOrderPreview is the on-disk ledger entry used by `order place --from-preview`.
// The canonical order JSON is stored alongside the decoded request so load can
// prove the file still matches its digest before submitting anything mutable.
type savedOrderPreview struct {
	Version        int                  `json:"version"`
	Digest         string               `json:"digest"`
	Operation      string               `json:"operation"`
	Endpoint       string               `json:"endpoint"`
	Account        string               `json:"account"`
	CreatedAt      time.Time            `json:"createdAt"`
	ExpiresAt      time.Time            `json:"expiresAt"`
	CanonicalOrder json.RawMessage      `json:"canonicalOrder"`
	Order          *models.OrderRequest `json:"order"`
	Preview        *models.PreviewOrder `json:"preview,omitempty"`
	OrderID        *int64               `json:"orderId,omitempty"`
	SafetyCheck    *previewSafetyCheck  `json:"safetyCheck,omitempty"`
}

// previewSafetyCheck records strategy-specific preflight checks that must be
// repeated when a saved preview is converted into a mutable order placement.
// It is included in the preview digest payload so local tampering fails closed.
type previewSafetyCheck struct {
	Type       string  `json:"type"`
	Underlying string  `json:"underlying,omitempty"`
	Contracts  float64 `json:"contracts,omitempty"`
}

// saveOrderPreview stores a previewed order in the local state ledger and
// returns the digest metadata callers should include in the preview envelope.
func saveOrderPreview(
	account string,
	order *models.OrderRequest,
	preview *models.PreviewOrder,
	safetyCheck *previewSafetyCheck,
) (*previewDigestData, error) {
	createdAt := time.Now().UTC()
	expiresAt := createdAt.Add(previewLedgerTTL)
	canonicalOrder, digest, err := previewDigestFor(account, order, safetyCheck)
	if err != nil {
		return nil, err
	}

	entry := savedOrderPreview{
		Version:        previewLedgerSchemaVersion,
		Digest:         digest,
		Operation:      previewLedgerOperation,
		Endpoint:       previewLedgerEndpoint,
		Account:        account,
		CreatedAt:      createdAt,
		ExpiresAt:      expiresAt,
		CanonicalOrder: canonicalOrder,
		Order:          order,
		Preview:        preview,
		SafetyCheck:    safetyCheck,
	}
	if preview != nil {
		entry.OrderID = preview.OrderID
	}

	ledgerDir, err := previewLedgerDir()
	if err != nil {
		return nil, err
	}
	if mkdirErr := os.MkdirAll(ledgerDir, 0o700); mkdirErr != nil {
		return nil, fmt.Errorf("create preview ledger directory: %w", mkdirErr)
	}
	// MkdirAll only applies the requested mode when it creates a directory.
	// Tighten existing directories too, because preview files contain exact order
	// payloads and the command help promises private ledger storage.
	//nolint:gosec // G302: directories need execute bits; 0700 keeps the ledger private.
	if chmodErr := os.Chmod(
		ledgerDir,
		0o700,
	); chmodErr != nil {
		return nil, fmt.Errorf("secure preview ledger directory: %w", chmodErr)
	}

	encoded, err := json.MarshalIndent(&entry, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal preview ledger entry: %w", err)
	}

	path := filepath.Join(ledgerDir, digest+".json")
	if writeErr := os.WriteFile(path, encoded, 0o600); writeErr != nil {
		return nil, fmt.Errorf("write preview ledger entry: %w", writeErr)
	}

	return &previewDigestData{
		Digest:    digest,
		ExpiresAt: expiresAt.Format(time.RFC3339),
		Account:   account,
		Operation: previewLedgerOperation,
	}, nil
}

// loadOrderPreview validates and returns a saved preview entry. Digest checking
// happens before expiry/account decisions so local tampering always fails closed.
func loadOrderPreview(digest string) (*savedOrderPreview, error) {
	digest = strings.TrimSpace(digest)
	if !isPreviewDigest(digest) {
		return nil, apperr.NewValidationError("preview digest must be a 64-character lowercase SHA-256 hex string", nil)
	}

	ledgerDir, err := previewLedgerDir()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(ledgerDir, digest+".json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, apperr.NewValidationError("preview digest was not found or has already been cleaned up", err)
		}
		return nil, fmt.Errorf("read preview ledger entry: %w", err)
	}

	var entry savedOrderPreview
	if unmarshalErr := json.Unmarshal(data, &entry); unmarshalErr != nil {
		return nil, apperr.NewValidationError("preview ledger entry is not valid JSON", unmarshalErr)
	}
	if entry.Version != previewLedgerSchemaVersion || entry.Operation != previewLedgerOperation ||
		entry.Endpoint != previewLedgerEndpoint {
		return nil, apperr.NewValidationError("preview ledger entry uses an unsupported schema or operation", nil)
	}
	if entry.Order == nil {
		return nil, apperr.NewValidationError("preview ledger entry is missing its order payload", nil)
	}

	canonicalOrder, expectedDigest, err := previewDigestFor(entry.Account, entry.Order, entry.SafetyCheck)
	if err != nil {
		return nil, err
	}
	var compactedLedgerOrder bytes.Buffer
	if compactErr := json.Compact(&compactedLedgerOrder, entry.CanonicalOrder); compactErr != nil {
		return nil, apperr.NewValidationError("preview ledger entry has invalid canonical order JSON", compactErr)
	}
	if expectedDigest != digest || entry.Digest != digest || string(canonicalOrder) != compactedLedgerOrder.String() {
		return nil, apperr.NewValidationError("preview ledger entry no longer matches its digest", nil)
	}
	if time.Now().UTC().After(entry.ExpiresAt) {
		return nil, apperr.NewValidationError("preview digest expired; run order preview --save-preview again", nil)
	}

	return &entry, nil
}

// previewDigestFor returns the canonical order JSON and the account-bound digest
// used as the immutable handoff between preview and place.
func previewDigestFor(
	account string,
	order *models.OrderRequest,
	safetyCheck *previewSafetyCheck,
) (json.RawMessage, string, error) {
	canonicalOrder, err := json.Marshal(order)
	if err != nil {
		return nil, "", fmt.Errorf("marshal canonical order: %w", err)
	}

	boundPayload := struct {
		Version        int                 `json:"version"`
		Operation      string              `json:"operation"`
		Endpoint       string              `json:"endpoint"`
		Account        string              `json:"account"`
		CanonicalOrder json.RawMessage     `json:"canonicalOrder"`
		SafetyCheck    *previewSafetyCheck `json:"safetyCheck,omitempty"`
	}{
		Version:        previewLedgerSchemaVersion,
		Operation:      previewLedgerOperation,
		Endpoint:       previewLedgerEndpoint,
		Account:        account,
		CanonicalOrder: canonicalOrder,
		SafetyCheck:    safetyCheck,
	}
	encoded, err := json.Marshal(boundPayload)
	if err != nil {
		return nil, "", fmt.Errorf("marshal preview digest payload: %w", err)
	}

	sum := sha256.Sum256(encoded)
	return canonicalOrder, hex.EncodeToString(sum[:]), nil
}

// previewLedgerDir resolves the local state directory for saved previews.
func previewLedgerDir() (string, error) {
	if stateDir := strings.TrimSpace(os.Getenv("SCHWAB_AGENT_STATE_DIR")); stateDir != "" {
		return filepath.Join(stateDir, "previews"), nil
	}
	if xdgStateHome := strings.TrimSpace(os.Getenv("XDG_STATE_HOME")); xdgStateHome != "" {
		return filepath.Join(xdgStateHome, "schwab-agent", "previews"), nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory for preview ledger: %w", err)
	}
	return filepath.Join(homeDir, ".local", "state", "schwab-agent", "previews"), nil
}

// isPreviewDigest rejects path traversal and non-canonical hex before any file
// path is built from user input.
func isPreviewDigest(digest string) bool {
	if len(digest) != sha256.Size*2 {
		return false
	}
	for _, r := range digest {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}
