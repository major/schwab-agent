package commands

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/client"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type defineAndConstrainTestOpts struct {
	Call bool `flag:"call" flagdescr:"Call option"`
	Put  bool `flag:"put" flagdescr:"Put option"`
}

// Attach implements structcli.Options for defineAndConstrainTestOpts.
func (o *defineAndConstrainTestOpts) Attach(_ *cobra.Command) error { return nil }

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

// runTestCommand configures the command to capture output and runs it with the given args.
func runTestCommand(t *testing.T, cmd *cobra.Command, args ...string) (string, error) {
	t.Helper()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func TestCobraRequireSubcommand(t *testing.T) {
	// Arrange - build a parent command with two subcommands
	parent := &cobra.Command{
		Use:   "parent",
		Short: "parent command",
		RunE:  requireSubcommand,
	}
	parent.AddCommand(&cobra.Command{
		Use:   "alpha",
		Short: "first subcommand",
	})
	parent.AddCommand(&cobra.Command{
		Use:   "beta",
		Short: "second subcommand",
	})

	t.Run("no argument produces validation error", func(t *testing.T) {
		// Act
		_, err := runTestCommand(t, parent)

		// Assert
		var valErr *apperr.ValidationError
		require.ErrorAs(t, err, &valErr)
		assert.Contains(t, valErr.Error(), `"parent" requires a subcommand`)
		assert.Contains(t, valErr.Error(), "alpha, beta")
	})

	t.Run("valid subcommand is not rejected", func(t *testing.T) {
		// Act
		_, err := runTestCommand(t, parent, "alpha")

		// Assert - no ValidationError means requireSubcommand didn't fire
		_, ok := errors.AsType[*apperr.ValidationError](err)
		assert.False(t, ok, "valid subcommand should not produce ValidationError")
	})

	t.Run("requireSubcommand directly with unknown arg", func(t *testing.T) {
		// Act - call the function directly with args
		err := requireSubcommand(parent, []string{"bogus"})

		// Assert
		var valErr *apperr.ValidationError
		require.ErrorAs(t, err, &valErr)
		assert.Contains(t, valErr.Error(), `unknown command "bogus" for "parent"`)
		assert.Contains(t, valErr.Error(), "alpha, beta")
	})

	t.Run("requireSubcommand directly with no args", func(t *testing.T) {
		// Act - call the function directly with no args
		err := requireSubcommand(parent, []string{})

		// Assert
		var valErr *apperr.ValidationError
		require.ErrorAs(t, err, &valErr)
		assert.Contains(t, valErr.Error(), `"parent" requires a subcommand`)
		assert.Contains(t, valErr.Error(), "alpha, beta")
	})
}

func TestCobraVisibleSubcommandNames(t *testing.T) {
	// Arrange
	parent := &cobra.Command{
		Use: "parent",
	}
	parent.AddCommand(&cobra.Command{Use: "visible1"})
	parent.AddCommand(&cobra.Command{Use: "visible2"})
	hidden := &cobra.Command{Use: "hidden"}
	hidden.Hidden = true
	parent.AddCommand(hidden)

	// Act
	names := visibleSubcommandNames(parent)

	// Assert
	assert.Equal(t, []string{"visible1", "visible2"}, names)
	assert.NotContains(t, names, "hidden")
}

func TestDefineAndConstrain(t *testing.T) {
	// Arrange
	cmd := &cobra.Command{
		Use:  "option",
		RunE: func(_ *cobra.Command, _ []string) error { return nil },
	}
	opts := &defineAndConstrainTestOpts{}

	// Act
	defineAndConstrain(cmd, opts, []string{"call", "put"})

	// Assert
	require.NotNil(t, cmd.Flags().Lookup("call"))
	require.NotNil(t, cmd.Flags().Lookup("put"))

	_, err := runTestCommand(t, cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one of the flags in the group [call put] is required")

	_, err = runTestCommand(t, cmd, "--call", "--put")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "if any flags in the group [call put] are set none of the others can be")
}
