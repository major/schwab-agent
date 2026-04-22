package commands

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/major/schwab-agent/internal/client"
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
