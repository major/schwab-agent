package commands

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildTestRoot creates a small Cobra command tree for schema testing.
func buildTestRoot(w io.Writer) *cobra.Command {
	root := &cobra.Command{
		Use:           "test-app",
		Short:         "A test application",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.CompletionOptions.DisableDefaultCmd = true
	root.SetOut(w)
	root.SetErr(w)
	root.AddGroup(&cobra.Group{ID: "tools", Title: "Tool Commands"})
	root.PersistentFlags().Bool("verbose", false, "Enable verbose output")
	root.PersistentFlags().String("output", "json", "Output format")

	accountCmd := &cobra.Command{Use: "account", Short: "Account operations"}
	listCmd := &cobra.Command{Use: "list", Short: "List all accounts"}
	listCmd.Flags().String("format", "table", "Output format")
	listCmd.Flags().Bool("all", false, "Show all accounts")
	accountCmd.AddCommand(listCmd)

	quoteCmd := &cobra.Command{Use: "quote", Short: "Get stock quotes"}
	quoteCmd.Flags().String("symbol", "", "Stock symbol")
	quoteCmd.Flags().Int("count", 10, "Number of quotes")
	quoteCmd.Flags().Float64("threshold", 0.5, "Price threshold")

	root.AddCommand(accountCmd, quoteCmd)
	return root
}

func runSchemaCommand(t *testing.T, root *cobra.Command, w *bytes.Buffer, args ...string) error {
	t.Helper()

	schemaCmd := NewSchemaCmd(root, w)
	root.AddCommand(schemaCmd)
	_, err := runCobraCommand(t, root, args...)
	return err
}

func TestNewSchemaCmd_FullOutput(t *testing.T) {
	var buf bytes.Buffer
	root := buildTestRoot(&buf)

	err := runSchemaCommand(t, root, &buf, "schema")
	require.NoError(t, err)

	var schema SchemaOutput
	err = json.Unmarshal(buf.Bytes(), &schema)
	require.NoError(t, err)

	// All application commands are present (parent and leaf nodes).
	assert.Len(t, schema.Commands, 3)
	assert.Contains(t, schema.Commands, "account")
	assert.Contains(t, schema.Commands, "account list")
	assert.Contains(t, schema.Commands, "quote")

	// Verify descriptions.
	assert.Equal(t, "Account operations", schema.Commands["account"].Description)
	assert.Equal(t, "List all accounts", schema.Commands["account list"].Description)
	assert.Equal(t, "Get stock quotes", schema.Commands["quote"].Description)

	// Verify global flags.
	assert.Len(t, schema.GlobalFlags, 2)
	assert.Contains(t, schema.GlobalFlags, "--verbose")
	assert.Contains(t, schema.GlobalFlags, "--output")
	assert.Equal(t, "bool", schema.GlobalFlags["--verbose"].Type)
	assert.Equal(t, "string", schema.GlobalFlags["--output"].Type)
	assert.Equal(t, "json", schema.GlobalFlags["--output"].Default)
}

func TestNewSchemaCmd_FlagTypes(t *testing.T) {
	var buf bytes.Buffer
	root := buildTestRoot(&buf)

	err := runSchemaCommand(t, root, &buf, "schema")
	require.NoError(t, err)

	var schema SchemaOutput
	err = json.Unmarshal(buf.Bytes(), &schema)
	require.NoError(t, err)

	// String flag with empty default.
	symbolFlag := schema.Commands["quote"].Flags["--symbol"]
	assert.Equal(t, "string", symbolFlag.Type)
	assert.Equal(t, "", symbolFlag.Default)
	assert.Equal(t, "Stock symbol", symbolFlag.Description)

	// Int flag with non-zero default. JSON numbers decode as float64.
	countFlag := schema.Commands["quote"].Flags["--count"]
	assert.Equal(t, "int", countFlag.Type)
	assert.Equal(t, float64(10), countFlag.Default)
	assert.Equal(t, "Number of quotes", countFlag.Description)

	// Float flag with fractional default.
	thresholdFlag := schema.Commands["quote"].Flags["--threshold"]
	assert.Equal(t, "float64", thresholdFlag.Type)
	assert.Equal(t, 0.5, thresholdFlag.Default)
	assert.Equal(t, "Price threshold", thresholdFlag.Description)

	// Bool flag with false default.
	allFlag := schema.Commands["account list"].Flags["--all"]
	assert.Equal(t, "bool", allFlag.Type)
	assert.Equal(t, false, allFlag.Default)
	assert.Equal(t, "Show all accounts", allFlag.Description)
}

func TestNewSchemaCmd_FilterByCommand(t *testing.T) {
	var buf bytes.Buffer
	root := buildTestRoot(&buf)

	err := runSchemaCommand(t, root, &buf, "schema", "--command", "account list")
	require.NoError(t, err)

	var schema SchemaOutput
	err = json.Unmarshal(buf.Bytes(), &schema)
	require.NoError(t, err)

	// Only the filtered command appears.
	assert.Len(t, schema.Commands, 1)
	assert.Contains(t, schema.Commands, "account list")

	cmd := schema.Commands["account list"]
	assert.Equal(t, "List all accounts", cmd.Description)
	assert.Len(t, cmd.Flags, 2)
	assert.Contains(t, cmd.Flags, "--format")
	assert.Contains(t, cmd.Flags, "--all")

	// Global flags still present.
	assert.Len(t, schema.GlobalFlags, 2)
}

func TestNewSchemaCmd_FilterNotFound(t *testing.T) {
	var buf bytes.Buffer
	root := buildTestRoot(&buf)

	err := runSchemaCommand(t, root, &buf, "schema", "--command", "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")
	assert.Contains(t, err.Error(), "not found")
}

func TestNewSchemaCmd_EmptyRoot(t *testing.T) {
	var buf bytes.Buffer
	root := &cobra.Command{
		Use:           "empty",
		Short:         "An empty application",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.CompletionOptions.DisableDefaultCmd = true
	root.AddGroup(&cobra.Group{ID: "tools", Title: "Tool Commands"})

	err := runSchemaCommand(t, root, &buf, "schema")
	require.NoError(t, err)

	var schema SchemaOutput
	err = json.Unmarshal(buf.Bytes(), &schema)
	require.NoError(t, err)

	assert.Empty(t, schema.Commands)
	assert.Empty(t, schema.GlobalFlags)
}

func TestNewSchemaCmd_NestedCommandPath(t *testing.T) {
	var buf bytes.Buffer
	root := &cobra.Command{
		Use:           "app",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.CompletionOptions.DisableDefaultCmd = true
	root.AddGroup(&cobra.Group{ID: "tools", Title: "Tool Commands"})
	orderCmd := &cobra.Command{Use: "order", Short: "Order operations"}
	placeCmd := &cobra.Command{Use: "place", Short: "Place an order"}
	equityCmd := &cobra.Command{Use: "equity", Short: "Place an equity order"}
	equityCmd.Flags().String("symbol", "", "Stock symbol")
	placeCmd.AddCommand(equityCmd)
	orderCmd.AddCommand(placeCmd)
	root.AddCommand(orderCmd)

	err := runSchemaCommand(t, root, &buf, "schema")
	require.NoError(t, err)

	var schema SchemaOutput
	err = json.Unmarshal(buf.Bytes(), &schema)
	require.NoError(t, err)

	// All levels appear with space-separated paths.
	assert.Contains(t, schema.Commands, "order")
	assert.Contains(t, schema.Commands, "order place")
	assert.Contains(t, schema.Commands, "order place equity")

	// Deepest command has the flag.
	equitySchema := schema.Commands["order place equity"]
	assert.Equal(t, "Place an equity order", equitySchema.Description)
	assert.Contains(t, equitySchema.Flags, "--symbol")
}

func TestNewSchemaCmd_RawJSONOutput(t *testing.T) {
	// Verify schema outputs raw JSON, not wrapped in the standard envelope.
	var buf bytes.Buffer
	root := buildTestRoot(&buf)

	err := runSchemaCommand(t, root, &buf, "schema")
	require.NoError(t, err)

	// Parse raw JSON and verify top-level keys.
	var raw map[string]json.RawMessage
	err = json.Unmarshal(buf.Bytes(), &raw)
	require.NoError(t, err)

	assert.Contains(t, raw, "commands")
	assert.Contains(t, raw, "global_flags")
	// Must NOT have envelope keys.
	assert.NotContains(t, raw, "data")
	assert.NotContains(t, raw, "metadata")
	assert.NotContains(t, raw, "error")
}

func TestNewSchemaCmd_HiddenFlagsExcluded(t *testing.T) {
	var buf bytes.Buffer
	root := buildTestRoot(&buf)
	quoteCmd, _, err := root.Find([]string{"quote"})
	require.NoError(t, err)
	hiddenFlag := quoteCmd.Flags().Lookup("symbol")
	require.NotNil(t, hiddenFlag)
	require.NoError(t, quoteCmd.Flags().MarkHidden(hiddenFlag.Name))

	err = runSchemaCommand(t, root, &buf, "schema")
	require.NoError(t, err)

	var schema SchemaOutput
	err = json.Unmarshal(buf.Bytes(), &schema)
	require.NoError(t, err)

	assert.NotContains(t, schema.Commands["quote"].Flags, "--symbol")
	assert.Contains(t, schema.Commands["quote"].Flags, "--count")
}
