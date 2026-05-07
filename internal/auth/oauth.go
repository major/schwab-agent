package auth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	schwabauth "github.com/major/schwab-go/schwab/auth"
	"github.com/pkg/browser"

	"github.com/major/schwab-agent/internal/apperr"
)

const (
	// oauthHTTPTimeout is the timeout for OAuth token exchange and refresh requests.
	oauthHTTPTimeout = 30 * time.Second

	// callbackServerTimeout bounds how long the local HTTPS callback server waits.
	callbackServerTimeout = 300 * time.Second
)

// AuthorizeURL builds the Schwab authorization URL and returns it with a random state value.
func AuthorizeURL(cfg *Config) (string, string, error) {
	schwabCfg, err := cfg.schwabAuthConfig("")
	if err != nil {
		return "", "", mapSchwabAuthError("failed to build OAuth authorization URL", err)
	}

	authorizeURL, state, err := schwabauth.AuthorizeURL(schwabCfg)
	if err != nil {
		return "", "", mapSchwabAuthError("failed to build OAuth authorization URL", err)
	}

	return authorizeURL, state, nil
}

// ExchangeCode exchanges an OAuth authorization code for a token file.
//
// The now parameter is retained for the test seam this package already exposed.
// schwab-go owns the exchange request and response parsing, then this adapter
// normalizes the saved timestamps when tests pass a deterministic clock.
func ExchangeCode(cfg *Config, code, tokenEndpoint string, now time.Time) (*TokenFile, error) {
	schwabCfg, err := cfg.schwabAuthConfig(tokenEndpoint)
	if err != nil {
		return nil, mapSchwabAuthError("failed to prepare token exchange", err)
	}

	tokenFile, err := schwabauth.ExchangeCode(context.Background(), schwabCfg, code, cfg.oauthHTTPClient())
	if err != nil {
		return nil, apperr.NewAuthCallbackError("token exchange failed", err)
	}

	if !now.IsZero() {
		nowUnix := now.Unix()
		tokenFile.CreationTimestamp = nowUnix
		if tokenFile.Token.ExpiresIn > 0 {
			tokenFile.Token.ExpiresAt = nowUnix + int64(tokenFile.Token.ExpiresIn)
		}
	}

	return &tokenFile, nil
}

// StartCallbackServer starts a loopback-only HTTPS callback server and waits for one callback.
func StartCallbackServer(addr, expectedState string) (string, error) {
	callbackURL, err := normalizeLoopbackCallbackURL(addr)
	if err != nil {
		return "", apperr.NewAuthCallbackError("invalid OAuth callback URL", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), callbackServerTimeout)
	defer cancel()

	results, errs, shutdown, err := schwabauth.StartCallbackServer(ctx, callbackURL)
	if err != nil {
		return "", mapSchwabAuthError("failed to start OAuth callback server", err)
	}
	defer shutdown()

	select {
	case result := <-results:
		if result.State != expectedState {
			return "", apperr.NewAuthCallbackError(
				"OAuth callback state mismatch",
				fmt.Errorf("expected state %q, got %q", expectedState, result.State),
			)
		}
		return result.Code, nil
	case callbackErr := <-errs:
		return "", mapSchwabAuthError("OAuth callback failed", callbackErr)
	case <-ctx.Done():
		return "", apperr.NewAuthCallbackError("timed out waiting for OAuth callback after 300 seconds", ctx.Err())
	}
}

func normalizeLoopbackCallbackURL(addr string) (string, error) {
	callbackURL := strings.TrimSpace(addr)
	if !strings.Contains(callbackURL, "://") {
		callbackURL = "https://" + callbackURL
	}

	parsedURL, err := url.Parse(callbackURL)
	if err != nil {
		return "", fmt.Errorf("parse callback URL: %w", err)
	}
	if parsedURL.Scheme != "https" {
		return "", errors.New("callback URL must use https")
	}
	if parsedURL.Hostname() != "127.0.0.1" {
		return "", fmt.Errorf("callback server must bind to 127.0.0.1 only, got %q", parsedURL.Hostname())
	}
	if parsedURL.Port() == "" {
		return "", errors.New("callback URL must include an explicit port")
	}
	if parsedURL.Path == "" {
		parsedURL.Path = "/"
	}

	return parsedURL.String(), nil
}

// RunLogin performs the full OAuth login flow and persists the resulting token.
func RunLogin(cfg *Config, tokenPath, tokenEndpoint string, openBrowser bool, w io.Writer) error {
	schwabCfg, err := cfg.schwabAuthConfig(tokenEndpoint)
	if err != nil {
		return mapSchwabAuthError("failed to prepare OAuth login", err)
	}

	urlHandler := func(authorizeURL string) error {
		if openBrowser {
			if openErr := browser.OpenURL(authorizeURL); openErr != nil {
				return apperr.NewAuthCallbackError("failed to open browser", openErr)
			}
			return nil
		}

		if _, writeErr := fmt.Fprintln(w, authorizeURL); writeErr != nil {
			return fmt.Errorf("failed to write authorization URL: %w", writeErr)
		}
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), callbackServerTimeout)
	defer cancel()

	_, err = schwabauth.Login(
		ctx,
		schwabCfg,
		schwabauth.NewFileTokenStore(tokenPath),
		urlHandler,
		schwabauth.WithLoginHTTPClient(cfg.oauthHTTPClient()),
	)
	if err != nil {
		return mapSchwabAuthError("OAuth login failed", err)
	}

	return nil
}

func mapSchwabAuthError(message string, err error) error {
	if err == nil {
		return nil
	}
	if schwabauth.IsCallback(err) {
		return apperr.NewAuthCallbackError(message, err)
	}
	if schwabauth.IsExpired(err) {
		return apperr.NewAuthExpiredError(message, err)
	}
	if schwabauth.IsRequired(err) {
		return apperr.NewAuthRequiredError(message, err)
	}

	return fmt.Errorf("%s: %w", message, err)
}
