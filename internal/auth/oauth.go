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

	"github.com/pkg/browser"
	"sync"
	"time"

	schwabErrors "github.com/major/schwab-agent/internal/errors"
)

const (
	// authorizeEndpoint is Schwab's OAuth authorization endpoint.
	authorizeEndpoint = "https://api.schwabapi.com/v1/oauth/authorize"

	// oauthTokenEndpoint is Schwab's OAuth token endpoint.
	oauthTokenEndpoint = "https://api.schwabapi.com/v1/oauth/token"

	// defaultCallbackAddr limits the local callback server to loopback.
	defaultCallbackAddr = "127.0.0.1:8182"

	// callbackSuccessHTML is the browser response shown after login completes.
	callbackSuccessHTML = "<html><body><p>Authentication successful! You can close this tab.</p></body></html>"
)

var (
	// callbackServerTimeout bounds how long the local HTTPS callback server waits.
	callbackServerTimeout = 300 * time.Second

	// oauthNow returns the current time. Tests override it for deterministic assertions.
	oauthNow = time.Now

	// browserOpenFunc opens the authorization URL in the user's browser.
	browserOpenFunc = browser.OpenURL
)

// AuthorizeURL builds the Schwab authorization URL and returns it with a random state value.
// It panics if the system CSPRNG is unavailable.
func AuthorizeURL(cfg *Config) (authURL, state string) {
	state = randomOAuthState()

	query := url.Values{}
	query.Set("client_id", cfg.ClientID)
	query.Set("redirect_uri", cfg.CallbackURL)
	query.Set("response_type", "code")
	query.Set("scope", "api")
	query.Set("state", state)

	return authorizeEndpoint + "?" + query.Encode(), state
}

// ExchangeCode exchanges an OAuth authorization code for a token file.
func ExchangeCode(cfg *Config, code, tokenEndpoint string) (*TokenFile, error) {
	if tokenEndpoint == "" {
		tokenEndpoint = oauthTokenEndpoint
	}

	formData := url.Values{}
	formData.Set("grant_type", "authorization_code")
	formData.Set("code", code)
	formData.Set("redirect_uri", cfg.CallbackURL)

	req, err := http.NewRequest(http.MethodPost, tokenEndpoint, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, schwabErrors.NewAuthCallbackError("failed to build token exchange request", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(cfg.ClientID, cfg.ClientSecret)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, schwabErrors.NewAuthCallbackError("token exchange request failed", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, schwabErrors.NewAuthCallbackError("failed to read token exchange response", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, schwabErrors.NewAuthCallbackError(
			fmt.Sprintf("token exchange failed with status %d", resp.StatusCode),
			fmt.Errorf("response body: %s", strings.TrimSpace(string(body))),
		)
	}

	var token TokenData
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, schwabErrors.NewAuthCallbackError("failed to parse token exchange response", err)
	}

	nowUnix := oauthNow().Unix()
	token.ExpiresAt = float64(nowUnix) + float64(token.ExpiresIn)

	return &TokenFile{
		CreationTimestamp: nowUnix,
		Token:             token,
	}, nil
}

// StartCallbackServer starts a loopback-only HTTPS callback server and waits for one callback.
func StartCallbackServer(addr, expectedState string) (code string, err error) {
	return startCallbackServer(addr, expectedState, nil)
}

// RunLogin performs the full OAuth login flow and persists the resulting token.
func RunLogin(cfg *Config, tokenPath, tokenEndpoint string, openBrowser bool, w io.Writer) error {
	authURL, state := AuthorizeURL(cfg)
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
		if err := browserOpenFunc(authURL); err != nil {
			return schwabErrors.NewAuthCallbackError("failed to open browser", err)
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
	tokenFile, err := ExchangeCode(cfg, result.code, tokenEndpoint)
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
func randomOAuthState() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		panic(fmt.Errorf("failed to generate OAuth state: %w", err))
	}

	return hex.EncodeToString(buf)
}

// callbackAddress extracts and validates the callback server address from config.
func callbackAddress(cfg *Config) (string, error) {
	parsedURL, err := url.Parse(cfg.CallbackURL)
	if err != nil {
		return "", schwabErrors.NewAuthCallbackError("invalid callback URL", err)
	}

	host := parsedURL.Host
	if host == "" {
		return "", schwabErrors.NewAuthCallbackError("callback URL must include a host and port", nil)
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
		return "", schwabErrors.NewAuthCallbackError("failed to generate callback TLS certificate", err)
	}

	listener, err := net.Listen("tcp", validatedAddr)
	if err != nil {
		if readyCh != nil {
			close(readyCh)
		}
		return "", schwabErrors.NewAuthCallbackError("failed to listen for OAuth callback", err)
	}

	tlsListener := tls.NewListener(listener, &tls.Config{Certificates: []tls.Certificate{certificate}})
	defer tlsListener.Close()

	resultCh := make(chan callbackResult, 1)
	var once sync.Once

	server := &http.Server{Handler: callbackHandler(expectedState, resultCh, &once)}

	go func() {
		serveErr := server.Serve(tlsListener)
		if serveErr != nil && serveErr != http.ErrServerClosed {
			once.Do(func() {
				resultCh <- callbackResult{
					err: schwabErrors.NewAuthCallbackError("OAuth callback server failed", serveErr),
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
		return "", schwabErrors.NewAuthCallbackError("timed out waiting for OAuth callback after 300 seconds", nil)
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
					err: schwabErrors.NewAuthCallbackError(
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
					err: schwabErrors.NewAuthCallbackError("OAuth callback missing code", nil),
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
		return "", schwabErrors.NewAuthCallbackError("callback address must include host and port", err)
	}

	if host != "127.0.0.1" {
		return "", schwabErrors.NewAuthCallbackError("callback server must bind to 127.0.0.1 only", nil)
	}

	return addr, nil
}

// generateSelfSignedCertificate creates an in-memory TLS certificate for the loopback callback server.
func generateSelfSignedCertificate() (tls.Certificate, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to generate private key: %w", err)
	}

	now := oauthNow()
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


