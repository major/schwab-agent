package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/pkg/browser"
	"resty.dev/v3"

	"github.com/major/schwab-agent/internal/apperr"
)

const (
	// defaultCallbackAddr limits the local callback server to loopback.
	defaultCallbackAddr = "127.0.0.1:8182"

	// callbackSuccessHTML is the browser response shown after login completes.
	callbackSuccessHTML = "<html><body><p>Authentication successful! You can close this tab.</p></body></html>"
)

const (
	// oauthHTTPTimeout is the timeout for OAuth token exchange and refresh requests.
	// Separate from the API client timeout since these hit a different endpoint and
	// are critical path for authentication. 30 seconds is generous but prevents
	// indefinite hangs if Schwab's OAuth endpoint is unresponsive.
	oauthHTTPTimeout = 30 * time.Second

	// callbackServerTimeout bounds how long the local HTTPS callback server waits.
	callbackServerTimeout = 300 * time.Second
)

// newOAuthClient creates a resty client configured for OAuth token requests.
// The caller must defer client.Close() to release idle connections.
func newOAuthClient(cfg *Config, timeout time.Duration) *resty.Client {
	c := resty.New().SetTimeout(timeout)
	if tlsCfg := cfg.TLSConfig(); tlsCfg != nil {
		c.SetTLSClientConfig(tlsCfg)
	}
	return c
}

// AuthorizeURL builds the Schwab authorization URL and returns it with a random state value.
func AuthorizeURL(cfg *Config) (string, string, error) {
	state, err := randomOAuthState()
	if err != nil {
		return "", "", apperr.NewAuthCallbackError("failed to generate OAuth state", err)
	}

	query := url.Values{}
	query.Set("client_id", cfg.ClientID)
	query.Set("redirect_uri", cfg.CallbackURL)
	query.Set("response_type", "code")
	query.Set("scope", "api")
	query.Set("state", state)

	return cfg.OAuthAuthorizeURL() + "?" + query.Encode(), state, nil
}

// ExchangeCode exchanges an OAuth authorization code for a token file.
// The now parameter controls the timestamp used for token expiry calculation,
// making the function deterministic for tests.
func ExchangeCode(cfg *Config, code, tokenEndpoint string, now time.Time) (*TokenFile, error) {
	if tokenEndpoint == "" {
		tokenEndpoint = cfg.OAuthTokenURL()
	}

	client := newOAuthClient(cfg, oauthHTTPTimeout)
	defer client.Close()

	resp, err := client.R().
		SetBasicAuth(cfg.ClientID, cfg.ClientSecret).
		SetFormData(map[string]string{
			"grant_type":   "authorization_code",
			"code":         code,
			"redirect_uri": cfg.CallbackURL,
		}).
		Post(tokenEndpoint)
	if err != nil {
		return nil, apperr.NewAuthCallbackError("token exchange request failed", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, apperr.NewAuthCallbackError(
			fmt.Sprintf("token exchange failed with status %d", resp.StatusCode()),
			fmt.Errorf("response body: %s", strings.TrimSpace(string(resp.Bytes()))),
		)
	}

	var token TokenData
	if err := json.Unmarshal(resp.Bytes(), &token); err != nil {
		return nil, apperr.NewAuthCallbackError("failed to parse token exchange response", err)
	}

	nowUnix := now.Unix()
	token.ExpiresAt = float64(nowUnix) + float64(token.ExpiresIn)

	return &TokenFile{
		CreationTimestamp: nowUnix,
		Token:             token,
	}, nil
}

// StartCallbackServer starts a loopback-only HTTPS callback server and waits for one callback.
func StartCallbackServer(addr, expectedState string) (string, error) {
	return startCallbackServer(addr, expectedState, nil)
}

// RunLogin performs the full OAuth login flow and persists the resulting token.
func RunLogin(cfg *Config, tokenPath, tokenEndpoint string, openBrowser bool, w io.Writer) error {
	authURL, state, err := AuthorizeURL(cfg)
	if err != nil {
		return err
	}

	callbackAddr, err := callbackAddress(cfg)
	if err != nil {
		return err
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	readyCh := make(chan struct{})
	resultCh := make(chan callbackResult, 1)

	go func() {
		code, callbackErr := startCallbackServer(callbackAddr, state, readyCh)
		resultCh <- callbackResult{code: code, err: callbackErr}
	}()

	<-readyCh
	logger.Info("OAuth callback server listening", "addr", callbackAddr)

	if openBrowser {
		logger.Info("Opening browser for Schwab login")
		if err := browser.OpenURL(authURL); err != nil {
			return apperr.NewAuthCallbackError("failed to open browser", err)
		}
	} else {
		logger.Info("Open this URL to authenticate", "url", authURL)
		if _, err := fmt.Fprintln(w, authURL); err != nil {
			return fmt.Errorf("failed to write authorization URL: %w", err)
		}
	}

	logger.Info("Waiting for OAuth callback")
	result := <-resultCh
	if result.err != nil {
		return result.err
	}

	logger.Info("Exchanging authorization code for token")
	tokenFile, err := ExchangeCode(cfg, result.code, tokenEndpoint, time.Now())
	if err != nil {
		return err
	}

	if err := SaveToken(tokenPath, tokenFile); err != nil {
		return fmt.Errorf("failed to save token: %w", err)
	}

	logger.Info("Saved OAuth token", "path", tokenPath)
	return nil
}

// callbackResult carries the callback server outcome.
type callbackResult struct {
	code string
	err  error
}

// randomOAuthState creates a 32-byte random hex state value.
// Returns an error if the system CSPRNG is unavailable instead of panicking,
// since this runs in a CLI tool where a clean error message beats a stack trace.
func randomOAuthState() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("failed to generate OAuth state: %w", err)
	}

	return hex.EncodeToString(buf), nil
}

// callbackAddress extracts and validates the callback server address from config.
func callbackAddress(cfg *Config) (string, error) {
	parsedURL, err := url.Parse(cfg.CallbackURL)
	if err != nil {
		return "", apperr.NewAuthCallbackError("invalid callback URL", err)
	}

	host := parsedURL.Host
	if host == "" {
		return "", apperr.NewAuthCallbackError("callback URL must include a host and port", nil)
	}

	validatedAddr, err := validateCallbackAddr(host)
	if err != nil {
		return "", err
	}

	return validatedAddr, nil
}

// startCallbackServer is the implementation behind StartCallbackServer with an optional readiness signal.
func startCallbackServer(addr, expectedState string, readyCh chan<- struct{}) (string, error) {
	validatedAddr, err := validateCallbackAddr(addr)
	if err != nil {
		if readyCh != nil {
			close(readyCh)
		}
		return "", err
	}

	certificate, err := generateSelfSignedCertificate()
	if err != nil {
		if readyCh != nil {
			close(readyCh)
		}
		return "", apperr.NewAuthCallbackError("failed to generate callback TLS certificate", err)
	}

	listenConfig := &net.ListenConfig{}
	listener, err := listenConfig.Listen(context.Background(), "tcp", validatedAddr)
	if err != nil {
		if readyCh != nil {
			close(readyCh)
		}
		return "", apperr.NewAuthCallbackError("failed to listen for OAuth callback", err)
	}

	tlsListener := tls.NewListener(listener, &tls.Config{Certificates: []tls.Certificate{certificate}})
	defer tlsListener.Close()

	resultCh := make(chan callbackResult, 1)
	var once sync.Once

	server := &http.Server{
		Handler:           callbackHandler(expectedState, resultCh, &once),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		serveErr := server.Serve(tlsListener)
		if serveErr != nil && serveErr != http.ErrServerClosed {
			once.Do(func() {
				resultCh <- callbackResult{
					err: apperr.NewAuthCallbackError("OAuth callback server failed", serveErr),
				}
			})
		}
	}()

	if readyCh != nil {
		close(readyCh)
	}

	timer := time.NewTimer(callbackServerTimeout)
	defer timer.Stop()

	select {
	case result := <-resultCh:
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		return result.code, result.err
	case <-timer.C:
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		return "", apperr.NewAuthCallbackError("timed out waiting for OAuth callback after 300 seconds", nil)
	}
}

// callbackHandler validates the callback request and forwards the result.
func callbackHandler(expectedState string, resultCh chan<- callbackResult, once *sync.Once) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		state := r.URL.Query().Get("state")

		if state != expectedState {
			http.Error(w, "state mismatch", http.StatusBadRequest)
			once.Do(func() {
				resultCh <- callbackResult{
					err: apperr.NewAuthCallbackError(
						"OAuth callback state mismatch",
						fmt.Errorf("expected state %q, got %q", expectedState, state),
					),
				}
			})
			return
		}

		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			once.Do(func() {
				resultCh <- callbackResult{
					err: apperr.NewAuthCallbackError("OAuth callback missing code", nil),
				}
			})
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, callbackSuccessHTML)

		once.Do(func() {
			resultCh <- callbackResult{code: code}
		})
	})
}

// validateCallbackAddr ensures the callback server binds only to 127.0.0.1.
func validateCallbackAddr(addr string) (string, error) {
	if addr == "" {
		return defaultCallbackAddr, nil
	}

	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return "", apperr.NewAuthCallbackError("callback address must include host and port", err)
	}

	if host != "127.0.0.1" {
		return "", apperr.NewAuthCallbackError("callback server must bind to 127.0.0.1 only", nil)
	}

	return addr, nil
}

// generateSelfSignedCertificate creates an in-memory TLS certificate for the loopback callback server.
func generateSelfSignedCertificate() (tls.Certificate, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to generate private key: %w", err)
	}

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: big.NewInt(now.UnixNano()),
		Subject: pkix.Name{
			CommonName: "127.0.0.1",
		},
		NotBefore:             now.Add(-1 * time.Minute),
		NotAfter:              now.Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to create certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})

	certificate, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to load TLS key pair: %w", err)
	}

	return certificate, nil
}
