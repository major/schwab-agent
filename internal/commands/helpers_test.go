package commands

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

// runTestCommand configures the command to suppress os.Exit and runs it with the given args.
func runTestCommand(t *testing.T, cmd *cli.Command, args ...string) error {
	t.Helper()
	cmd.ExitErrHandler = func(_ context.Context, _ *cli.Command, _ error) {}
	return cmd.Run(context.Background(), args)
}

// testClient creates a *client.Ref backed by the given httptest server.
func testClient(t *testing.T, server *httptest.Server) *client.Ref {
	t.Helper()
	return &client.Ref{Client: client.NewClient("test-token", client.WithBaseURL(server.URL))}
}

// jsonServer returns an httptest.Server that always responds with the given JSON body.
func jsonServer(body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
}

func TestRequireSubcommand(t *testing.T) {
	// Arrange - build a parent command with two subcommands
	parent := &cli.Command{
		Name:   "parent",
		Action: requireSubcommand(),
		Commands: []*cli.Command{
			{Name: "alpha", Usage: "first subcommand"},
			{Name: "beta", Usage: "second subcommand"},
		},
	}

	t.Run("unknown argument produces validation error", func(t *testing.T) {
		// Act
		err := runTestCommand(t, parent, "parent", "bogus")

		// Assert
		var valErr *apperr.ValidationError
		require.ErrorAs(t, err, &valErr)
		assert.Contains(t, valErr.Error(), `unknown command "bogus" for "parent"`)
		assert.Contains(t, valErr.Error(), "alpha, beta")
	})

	t.Run("no argument produces validation error", func(t *testing.T) {
		// Act
		err := runTestCommand(t, parent, "parent")

		// Assert
		var valErr *apperr.ValidationError
		require.ErrorAs(t, err, &valErr)
		assert.Contains(t, valErr.Error(), `"parent" requires a subcommand`)
		assert.Contains(t, valErr.Error(), "alpha, beta")
	})

	t.Run("valid subcommand is not rejected", func(t *testing.T) {
		// Act
		err := runTestCommand(t, parent, "parent", "alpha")

		// Assert - no ValidationError means requireSubcommand didn't fire
		var valErr *apperr.ValidationError
		assert.False(t, errors.As(err, &valErr), "valid subcommand should not produce ValidationError")
	})
}
