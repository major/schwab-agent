package commands

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHelpTopicCmds(t *testing.T) {
	cmds := NewHelpTopicCmds(&bytes.Buffer{})
	require.Len(t, cmds, 2)

	byUse := map[string]*cobra.Command{}
	for _, cmd := range cmds {
		byUse[cmd.Use] = cmd
		assert.Equal(t, annotationValueTrue, cmd.Annotations[annotationSkipAuth])
		assert.Equal(t, groupIDTools, cmd.GroupID)
	}

	assert.NotNil(t, byUse["env-vars"])
	assert.NotNil(t, byUse["config-keys"])
}

func TestEnvVarsCmdOutput(t *testing.T) {
	var out bytes.Buffer
	cmd := newEnvVarsCmd(&out)

	_, err := runTestCommand(t, cmd)

	require.NoError(t, err)
	text := out.String()
	assert.Contains(t, text, "Environment Variables")
	assert.Contains(t, text, "SCHWAB_CLIENT_ID")
	assert.Contains(t, text, "SCHWAB_AGENT_STATE_DIR")
}

func TestConfigKeysCmdOutput(t *testing.T) {
	var out bytes.Buffer
	root := &cobra.Command{Use: "schwab-agent"}
	root.AddGroup(&cobra.Group{ID: groupIDTools, Title: "Tool Commands"})
	root.PersistentFlags().String("config", "", "config path")
	root.AddCommand(newConfigKeysCmd(&out))

	_, err := runTestCommand(t, root, "config-keys")

	require.NoError(t, err)
	text := out.String()
	assert.Contains(t, text, "Configuration Keys")
	assert.Contains(t, text, "schwab-agent (global)")
	assert.Contains(t, text, "config")
	assert.Contains(t, text, "Keys can be nested")
}

func TestWriteConfigKeysFiltersAndFormatsFlags(t *testing.T) {
	var out bytes.Buffer
	root := &cobra.Command{Use: "root"}
	root.PersistentFlags().String("config", "/tmp/config.json", "config path")

	visible := &cobra.Command{Use: "visible"}
	visible.Flags().Int("strike-count", 0, "number of strikes")
	visible.Flags().String("hidden-flag", "secret", "hidden flag")
	require.NoError(t, visible.Flags().MarkHidden("hidden-flag"))

	hidden := &cobra.Command{Use: "hidden", Hidden: true}
	hidden.Flags().String("leaked", "nope", "hidden command flag")

	root.AddCommand(visible, hidden)

	err := writeConfigKeys(&out, root)

	require.NoError(t, err)
	text := out.String()
	assert.Contains(t, text, "Configuration Keys")
	assert.Contains(t, text, "root (global)")
	assert.Contains(t, text, "config")
	assert.Contains(t, text, "visible")
	assert.Contains(t, text, "strikecount")
	assert.Contains(t, text, "--strike-count")
	assert.Contains(t, text, "Keys can be nested")
	assert.NotContains(t, text, "hidden-flag")
	assert.NotContains(t, text, "leaked")
}

func TestFlagRowsSkipsHelpAndHiddenFlags(t *testing.T) {
	cmd := &cobra.Command{Use: "flags"}
	cmd.Flags().Bool("keep-me", true, "visible flag")
	cmd.Flags().String("hide-me", "x", "hidden flag")
	require.NoError(t, cmd.Flags().MarkHidden("hide-me"))

	rows := flagRows(cmd.Flags())

	require.Len(t, rows, 1)
	assert.Equal(t, flagRow{
		key:          "keepme",
		flag:         "keep-me",
		valueType:    "bool",
		defaultValue: "true",
	}, rows[0])
}
