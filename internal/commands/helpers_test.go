package commands

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/models"
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

type cobraFlagCoverageOpts struct {
	Symbol   string           `flag:"symbol" flagdescr:"Symbol" flagshort:"s" default:"AAPL"`
	Count    int              `flag:"count" flagdescr:"Count" default:"3"`
	Price    float64          `flag:"price" flagdescr:"Price" default:"1.25"`
	Enabled  bool             `flag:"enabled" flagdescr:"Enabled" default:"true"`
	Fields   []string         `flag:"field" flagdescr:"Field"`
	Duration models.Duration  `flag:"duration" flagdescr:"Duration" default:"DAY"`
	Type     models.OrderType `flag:"type" flagdescr:"Order type"`
}

type requiredCobraFlagOpts struct {
	Symbol string `flag:"symbol" flagdescr:"Symbol" flagrequired:"true"`
}

type unsupportedSliceFlagOpts struct {
	Values []int `flag:"value" flagdescr:"Value"`
}

type unsupportedMapFlagOpts struct {
	Values map[string]string `flag:"value" flagdescr:"Value"`
}

type unsupportedDefaultOpts struct {
	Values []string `flag:"value" flagdescr:"Value" default:"bad"`
}

type invalidIntDefaultOpts struct {
	Count int `flag:"count" flagdescr:"Count" default:"bad"`
}

type invalidFloatDefaultOpts struct {
	Price float64 `flag:"price" flagdescr:"Price" default:"bad"`
}

type invalidBoolDefaultOpts struct {
	Enabled bool `flag:"enabled" flagdescr:"Enabled" default:"bad"`
}

type validationCoverageOpts struct {
	errs []error
}

func (o *validationCoverageOpts) Validate(_ context.Context) []error {
	return o.errs
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

func TestDefineCobraFlags(t *testing.T) {
	t.Run("binds supported flag types defaults enums and aliases", func(t *testing.T) {
		// Arrange
		opts := &cobraFlagCoverageOpts{}
		cmd := &cobra.Command{
			Use: "bind",
			RunE: func(_ *cobra.Command, _ []string) error {
				return nil
			},
		}
		defineCobraFlags(cmd, opts)

		// Assert defaults are applied before Cobra executes so handlers can read opts directly.
		assert.Equal(t, "AAPL", opts.Symbol)
		assert.Equal(t, 3, opts.Count)
		assert.Equal(t, 1.25, opts.Price)
		assert.True(t, opts.Enabled)
		assert.Equal(t, models.DurationDay, opts.Duration)
		require.NotNil(t, cmd.Flags().Lookup("duration"))
		assert.Equal(t, []string{"DAY", "END_OF_MONTH", "END_OF_WEEK", "FILL_OR_KILL", "GOOD_TILL_CANCEL", "IMMEDIATE_OR_CANCEL", "NEXT_END_OF_MONTH"}, cmd.Flags().Lookup("duration").Annotations[structcliFlagEnumAnnotation])

		// Act
		_, err := runTestCommand(
			t,
			cmd,
			"--symbol", "MSFT",
			"--count", "7",
			"--price", "2.50",
			"--enabled=false",
			"--field", "quote",
			"--field", "fundamental",
			"--duration", "gtc",
			"--type", "MOC",
		)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "MSFT", opts.Symbol)
		assert.Equal(t, 7, opts.Count)
		assert.Equal(t, 2.50, opts.Price)
		assert.False(t, opts.Enabled)
		assert.Equal(t, []string{"quote", "fundamental"}, opts.Fields)
		assert.Equal(t, models.DurationGoodTillCancel, opts.Duration)
		assert.Equal(t, models.OrderTypeMarketOnClose, opts.Type)
	})

	t.Run("required tagged flag is enforced", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "required", RunE: func(_ *cobra.Command, _ []string) error { return nil }}
		defineCobraFlags(cmd, &requiredCobraFlagOpts{})

		// Act
		_, err := runTestCommand(t, cmd)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), `required flag(s) "symbol" not set`)
	})

	t.Run("rejects invalid options argument", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "invalid"}

		// Act / Assert
		assert.PanicsWithValue(t, "defineCobraFlags requires a pointer to an options struct", func() {
			defineCobraFlags(cmd, cobraFlagCoverageOpts{})
		})
		assert.PanicsWithValue(t, "defineCobraFlags requires a pointer to an options struct", func() {
			defineCobraFlags(cmd, (*cobraFlagCoverageOpts)(nil))
		})
	})

	t.Run("rejects invalid defaults and unsupported flag types", func(t *testing.T) {
		for name, opts := range map[string]any{
			"invalid int default":      &invalidIntDefaultOpts{},
			"invalid float default":    &invalidFloatDefaultOpts{},
			"invalid bool default":     &invalidBoolDefaultOpts{},
			"unsupported default type": &unsupportedDefaultOpts{},
			"unsupported slice flag":   &unsupportedSliceFlagOpts{},
			"unsupported map flag":     &unsupportedMapFlagOpts{},
		} {
			t.Run(name, func(t *testing.T) {
				// Arrange
				cmd := &cobra.Command{Use: name}

				// Act / Assert
				assert.Panics(t, func() {
					defineCobraFlags(cmd, opts)
				})
			})
		}
	})
}

func TestCobraStringEnumValue(t *testing.T) {
	t.Run("handles nil and invalid reflect values", func(t *testing.T) {
		assert.Empty(t, (*cobraStringEnumValue)(nil).String())
		assert.Empty(t, (&cobraStringEnumValue{}).String())
		assert.Equal(t, "string", (&cobraStringEnumValue{}).Type())
	})

	t.Run("accepts blank canonical and alias values", func(t *testing.T) {
		// Arrange
		opts := &cobraFlagCoverageOpts{}
		cmd := &cobra.Command{Use: "enum", RunE: func(_ *cobra.Command, _ []string) error { return nil }}
		defineCobraFlags(cmd, opts)

		// Act / Assert
		_, err := runTestCommand(t, cmd, "--type", "limit")
		require.NoError(t, err)
		assert.Equal(t, models.OrderTypeLimit, opts.Type)

		value := cmd.Flags().Lookup("type").Value
		require.NoError(t, value.Set(""))
		assert.Empty(t, opts.Type)
		assert.Equal(t, `invalid value "bogus" (allowed: LIMIT, LIMIT_ON_CLOSE, MARKET, MARKET_ON_CLOSE, NET_CREDIT, NET_DEBIT, NET_ZERO, STOP, STOP_LIMIT, TRAILING_STOP, TRAILING_STOP_LIMIT)`, value.Set("bogus").Error())
	})
}

func TestValidateCobraOptions(t *testing.T) {
	t.Run("ignores structs without validation hooks", func(t *testing.T) {
		assert.NoError(t, validateCobraOptions(context.Background(), &cobraFlagCoverageOpts{}))
	})

	t.Run("accepts empty validation result", func(t *testing.T) {
		assert.NoError(t, validateCobraOptions(context.Background(), &validationCoverageOpts{}))
	})

	t.Run("combines validation messages", func(t *testing.T) {
		// Arrange
		opts := &validationCoverageOpts{errs: []error{errors.New("first"), errors.New("second")}}

		// Act
		err := validateCobraOptions(context.Background(), opts)

		// Assert
		var valErr *apperr.ValidationError
		require.ErrorAs(t, err, &valErr)
		assert.Equal(t, "first; second", valErr.Error())
	})
}

func TestNormalizeFlagValidationError(t *testing.T) {
	t.Run("invalid enum includes bad value and valid values", func(t *testing.T) {
		// Arrange - Cobra enum registration annotates flags with the canonical
		// values that should appear in remediation text.
		cmd := &cobra.Command{
			Use:  "indicator",
			RunE: func(_ *cobra.Command, _ []string) error { return nil },
		}
		cmd.SetFlagErrorFunc(normalizeFlagValidationErrorFunc)
		defineCobraFlags(cmd, &flagValidationTestOpts{})

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
