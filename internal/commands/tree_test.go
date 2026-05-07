package commands

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildCommandTreeRegistersTopLevelCommands(t *testing.T) {
	root := BuildCommandTree(&bytes.Buffer{}, "config.json", "token.json", "test", RootDeps{}, AuthDeps{})

	require.NotNil(t, root.PersistentPreRunE)
	require.NotNil(t, root.PersistentPostRunE)
	for _, commandName := range []string{
		"auth",
		"account",
		"position",
		"quote",
		"order",
		"option",
		"history",
		"instrument",
		"market",
		"symbol",
		"ta",
		"indicators",
		"analyze",
		"completion",
		"env-vars",
		"config-keys",
	} {
		found, _, err := root.Find([]string{commandName})
		require.NoError(t, err)
		require.NotNil(t, found, "BuildCommandTree() missing %q command", commandName)
		assert.Equal(t, commandName, found.Name())
	}
}
