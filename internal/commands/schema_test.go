package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

// testApp builds a small CLI app for schema testing with nested commands and various flag types.
func testApp() *cli.Command {
	return &cli.Command{
		Name:  "test-app",
		Usage: "A test application",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "verbose",
				Usage: "Enable verbose output",
			},
			&cli.StringFlag{
				Name:  "output",
				Usage: "Output format",
				Value: "json",
			},
		},
		Commands: []*cli.Command{
			{
				Name:  "account",
				Usage: "Account operations",
				Commands: []*cli.Command{
					{
						Name:  "list",
						Usage: "List all accounts",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "format",
								Usage: "Output format",
								Value: "table",
							},
							&cli.BoolFlag{
								Name:     "all",
								Usage:    "Show all accounts",
								Required: true,
							},
						},
					},
				},
			},
			{
				Name:  "quote",
				Usage: "Get stock quotes",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "symbol",
						Usage:    "Stock symbol",
						Required: true,
					},
					&cli.IntFlag{
						Name:  "count",
						Usage: "Number of quotes",
						Value: 10,
					},
					&cli.Float64Flag{
						Name:  "threshold",
						Usage: "Price threshold",
						Value: 0.5,
					},
				},
			},
		},
	}
}

func TestSchemaCommand_FullOutput(t *testing.T) {
	app := testApp()
	var buf bytes.Buffer
	schemaCmd := SchemaCommand(app, &buf)

	err := schemaCmd.Run(context.Background(), []string{"schema"})
	require.NoError(t, err)

	var schema SchemaOutput
	err = json.Unmarshal(buf.Bytes(), &schema)
	require.NoError(t, err)

	// All commands present (parent and leaf nodes).
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

func TestSchemaCommand_FlagTypes(t *testing.T) {
	app := testApp()
	var buf bytes.Buffer
	schemaCmd := SchemaCommand(app, &buf)

	err := schemaCmd.Run(context.Background(), []string{"schema"})
	require.NoError(t, err)

	var schema SchemaOutput
	err = json.Unmarshal(buf.Bytes(), &schema)
	require.NoError(t, err)

	// String flag (required, empty default).
	symbolFlag := schema.Commands["quote"].Flags["--symbol"]
	assert.Equal(t, "string", symbolFlag.Type)
	assert.True(t, symbolFlag.Required)
	assert.Equal(t, "", symbolFlag.Default)
	assert.Equal(t, "Stock symbol", symbolFlag.Description)

	// Int flag (optional, non-zero default). JSON numbers decode as float64.
	countFlag := schema.Commands["quote"].Flags["--count"]
	assert.Equal(t, "int", countFlag.Type)
	assert.False(t, countFlag.Required)
	assert.Equal(t, float64(10), countFlag.Default)
	assert.Equal(t, "Number of quotes", countFlag.Description)

	// Float flag (optional, fractional default).
	thresholdFlag := schema.Commands["quote"].Flags["--threshold"]
	assert.Equal(t, "float", thresholdFlag.Type)
	assert.False(t, thresholdFlag.Required)
	assert.Equal(t, 0.5, thresholdFlag.Default)
	assert.Equal(t, "Price threshold", thresholdFlag.Description)

	// Bool flag (required, false default).
	allFlag := schema.Commands["account list"].Flags["--all"]
	assert.Equal(t, "bool", allFlag.Type)
	assert.True(t, allFlag.Required)
	assert.Equal(t, false, allFlag.Default)
	assert.Equal(t, "Show all accounts", allFlag.Description)
}

func TestSchemaCommand_FilterByCommand(t *testing.T) {
	app := testApp()
	var buf bytes.Buffer
	schemaCmd := SchemaCommand(app, &buf)

	err := schemaCmd.Run(context.Background(), []string{"schema", "--command", "account list"})
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

func TestSchemaCommand_FilterNotFound(t *testing.T) {
	app := testApp()
	var buf bytes.Buffer
	schemaCmd := SchemaCommand(app, &buf)

	err := schemaCmd.Run(context.Background(), []string{"schema", "--command", "nonexistent"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")
	assert.Contains(t, err.Error(), "not found")
}

func TestSchemaCommand_EmptyApp(t *testing.T) {
	app := &cli.Command{
		Name:  "empty",
		Usage: "An empty application",
	}
	var buf bytes.Buffer
	schemaCmd := SchemaCommand(app, &buf)

	err := schemaCmd.Run(context.Background(), []string{"schema"})
	require.NoError(t, err)

	var schema SchemaOutput
	err = json.Unmarshal(buf.Bytes(), &schema)
	require.NoError(t, err)

	assert.Empty(t, schema.Commands)
	assert.Empty(t, schema.GlobalFlags)
}

func TestSchemaCommand_NestedCommandPath(t *testing.T) {
	app := &cli.Command{
		Name: "app",
		Commands: []*cli.Command{
			{
				Name:  "order",
				Usage: "Order operations",
				Commands: []*cli.Command{
					{
						Name:  "place",
						Usage: "Place an order",
						Commands: []*cli.Command{
							{
								Name:  "equity",
								Usage: "Place an equity order",
								Flags: []cli.Flag{
									&cli.StringFlag{
										Name:     "symbol",
										Usage:    "Stock symbol",
										Required: true,
									},
								},
							},
						},
					},
				},
			},
		},
	}
	var buf bytes.Buffer
	schemaCmd := SchemaCommand(app, &buf)

	err := schemaCmd.Run(context.Background(), []string{"schema"})
	require.NoError(t, err)

	var schema SchemaOutput
	err = json.Unmarshal(buf.Bytes(), &schema)
	require.NoError(t, err)

	// All levels appear with space-separated paths.
	assert.Contains(t, schema.Commands, "order")
	assert.Contains(t, schema.Commands, "order place")
	assert.Contains(t, schema.Commands, "order place equity")

	// Deepest command has the flag.
	equityCmd := schema.Commands["order place equity"]
	assert.Equal(t, "Place an equity order", equityCmd.Description)
	assert.Contains(t, equityCmd.Flags, "--symbol")
	assert.True(t, equityCmd.Flags["--symbol"].Required)
}

func TestClassifyFlag_UnknownType_FallsBackToString(t *testing.T) {
	// The default branch handles any flag type not explicitly matched
	// (String, Int, Float64, Bool). UintFlag triggers this path.
	t.Run("with name", func(t *testing.T) {
		f := &cli.UintFlag{Name: "retries", Usage: "retry count"}
		name, schema := classifyFlag(f)
		assert.Equal(t, "retries", name)
		assert.Equal(t, "string", schema.Type, "unknown flag types fall back to string")
	})
}

func TestSchemaCommand_RawJSONOutput(t *testing.T) {
	// Verify schema outputs raw JSON, not wrapped in the standard envelope.
	app := testApp()
	var buf bytes.Buffer
	schemaCmd := SchemaCommand(app, &buf)

	err := schemaCmd.Run(context.Background(), []string{"schema"})
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
