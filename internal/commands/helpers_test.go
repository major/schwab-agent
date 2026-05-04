package commands

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/leodido/structcli"
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

type flagValidationTestOpts struct {
	Interval taInterval `flag:"interval" flagdescr:"Data interval" default:"daily"`
}

// Attach implements structcli.Options for defineAndConstrainTestOpts.
func (o *defineAndConstrainTestOpts) Attach(_ *cobra.Command) error { return nil }

// Attach implements structcli.Options for flagValidationTestOpts.
func (o *flagValidationTestOpts) Attach(_ *cobra.Command) error { return nil }

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

func TestDefaultSubcommand(t *testing.T) {
	newParent := func(t *testing.T, called *bool, gotArgs *[]string) *cobra.Command {
		t.Helper()

		defaultCmd := &cobra.Command{
			Use:   "get SYMBOL",
			Short: "default subcommand",
			RunE: func(_ *cobra.Command, args []string) error {
				*called = true
				*gotArgs = append((*gotArgs)[:0], args...)

				return nil
			},
		}
		parent := &cobra.Command{
			Use:  "parent",
			Args: cobra.ArbitraryArgs,
			RunE: defaultSubcommand(defaultCmd),
		}
		parent.AddCommand(defaultCmd)
		parent.AddCommand(&cobra.Command{
			Use:   "sma SYMBOL",
			Short: "named subcommand",
			RunE:  func(_ *cobra.Command, _ []string) error { return nil },
		})

		return parent
	}

	t.Run("args dispatch to default subcommand RunE", func(t *testing.T) {
		// Arrange
		called := false
		gotArgs := []string{}
		parent := newParent(t, &called, &gotArgs)

		// Act
		_, err := runTestCommand(t, parent, "AAPL")

		// Assert
		require.NoError(t, err)
		assert.True(t, called)
		assert.Equal(t, []string{"AAPL"}, gotArgs)
	})

	t.Run("no args returns requireSubcommand validation error", func(t *testing.T) {
		// Arrange
		called := false
		gotArgs := []string{}
		parent := newParent(t, &called, &gotArgs)

		// Act
		_, err := runTestCommand(t, parent)

		// Assert
		var valErr *apperr.ValidationError
		require.ErrorAs(t, err, &valErr)
		assert.Contains(t, valErr.Error(), `"parent" requires a subcommand`)
		assert.False(t, called)
		assert.Empty(t, gotArgs)
	})

	t.Run("subcommand name falls through to requireSubcommand", func(t *testing.T) {
		// Arrange
		called := false
		gotArgs := []string{}
		parent := newParent(t, &called, &gotArgs)

		// Act - Cobra normally routes this before parent RunE. Calling the RunE
		// directly verifies the defensive guard for future command wiring changes.
		err := parent.RunE(parent, []string{"sma"})

		// Assert
		var valErr *apperr.ValidationError
		require.ErrorAs(t, err, &valErr)
		assert.Contains(t, valErr.Error(), `unknown command "sma" for "parent"`)
		assert.False(t, called)
		assert.Empty(t, gotArgs)
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

func TestNormalizeFlagValidationError(t *testing.T) {
	t.Run("invalid enum includes bad value and valid values", func(t *testing.T) {
		// Arrange - structcli annotates registered enum flags with the canonical
		// values that should appear in remediation text.
		cmd := &cobra.Command{
			Use:  "indicator",
			RunE: func(_ *cobra.Command, _ []string) error { return nil },
		}
		cmd.SetFlagErrorFunc(normalizeFlagValidationErrorFunc)
		require.NoError(t, structcli.Define(cmd, &flagValidationTestOpts{}))

		// Act
		_, err := runTestCommand(t, cmd, "--interval", "bogus")

		// Assert
		var valErr *apperr.ValidationError
		require.ErrorAs(t, err, &valErr)
		assert.Equal(t, `invalid interval: "bogus" (valid: 15min, 1min, 30min, 5min, daily, weekly)`, valErr.Error())
	})

	t.Run("non-enum invalid value passes through unchanged", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "counter"}
		cmd.Flags().Int("count", 0, "Count")

		rawErr := errors.New(`invalid argument "bogus" for "--count" flag: invalid value`)

		// Act
		err := normalizeFlagValidationErrorFunc(cmd, rawErr)

		// Assert
		assert.Same(t, rawErr, err)
	})

	t.Run("unknown flag passes through unchanged", func(t *testing.T) {
		// Arrange
		rawErr := errors.New("unknown flag: --bogus")

		// Act
		err := normalizeFlagValidationError(rawErr)

		// Assert
		assert.Same(t, rawErr, err)
	})

	t.Run("missing flag argument passes through unchanged", func(t *testing.T) {
		// Arrange
		rawErr := errors.New("flag needs an argument: --symbol")

		// Act
		err := normalizeFlagValidationError(rawErr)

		// Assert
		assert.Same(t, rawErr, err)
	})
}
