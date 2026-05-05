package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingDebugWriter struct{}

func (failingDebugWriter) Write(_ []byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestInstallDebugOptionsJSONSkipsHandlerAndReportsSources(t *testing.T) {
	// Arrange
	var output bytes.Buffer
	called := false
	root := &cobra.Command{Use: "root"}
	child := &cobra.Command{
		Use: "child",
		RunE: func(_ *cobra.Command, _ []string) error {
			called = true
			return nil
		},
	}
	child.Flags().String("symbol", "AAPL", "symbol to inspect")
	child.Flags().String("hidden", "secret", "hidden test flag")
	require.NoError(t, child.Flags().MarkHidden("hidden"))
	root.AddCommand(child)
	InstallDebugOptions(root)
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs([]string{"child", "--symbol", "MSFT", "--debug-options=json"})

	// Act
	err := root.Execute()

	// Assert
	require.NoError(t, err)
	assert.False(t, called)

	var envelope debugOptionsEnvelope
	require.NoError(t, json.Unmarshal(output.Bytes(), &envelope))
	assert.Equal(t, "root child", envelope.Command)
	assert.Equal(t, []debugFlag{
		{Name: "debug-options", Value: "json", Source: "argv", Changed: true},
		{Name: "help", Value: "false", Source: "default", Changed: false},
		{Name: "symbol", Value: "MSFT", Source: "argv", Changed: true},
	}, envelope.Flags)
}

func TestInstallDebugOptionsBareFlagWritesText(t *testing.T) {
	// Arrange
	var output bytes.Buffer
	root := &cobra.Command{Use: "root"}
	child := &cobra.Command{Use: "child", RunE: func(_ *cobra.Command, _ []string) error { return nil }}
	child.Flags().Bool("verbose", false, "show verbose output")
	root.AddCommand(child)
	InstallDebugOptions(root)
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs([]string{"child", "--debug-options"})

	// Act
	err := root.Execute()

	// Assert
	require.NoError(t, err)
	assert.Contains(t, output.String(), "Command: root child")
	assert.Contains(t, output.String(), "--debug-options=text (argv)")
	assert.Contains(t, output.String(), "--verbose=false (default)")
}

func TestInstallDebugOptionsPreservesRunnableCommandsWhenInactive(t *testing.T) {
	// Arrange
	runCalled := false
	runECalled := false
	root := &cobra.Command{Use: "root"}
	runChild := &cobra.Command{Use: "run", Run: func(_ *cobra.Command, _ []string) { runCalled = true }}
	runEChild := &cobra.Command{Use: "rune", RunE: func(_ *cobra.Command, _ []string) error {
		runECalled = true
		return nil
	}}
	root.AddCommand(runChild, runEChild)
	InstallDebugOptions(root)

	// Act and assert
	root.SetArgs([]string{"run"})
	require.NoError(t, root.Execute())
	root.SetArgs([]string{"rune"})
	require.NoError(t, root.Execute())
	assert.True(t, runCalled)
	assert.True(t, runECalled)
}

func TestIsDebugOptionsActive(t *testing.T) {
	t.Run("false when flag is absent", func(t *testing.T) {
		cmd := &cobra.Command{Use: "bare"}

		assert.False(t, IsDebugOptionsActive(cmd))
	})

	t.Run("false when flag is installed but unchanged", func(t *testing.T) {
		cmd := &cobra.Command{Use: "root", RunE: func(_ *cobra.Command, _ []string) error { return nil }}
		InstallDebugOptions(cmd)

		assert.False(t, IsDebugOptionsActive(cmd))
	})

	t.Run("true when flag is changed", func(t *testing.T) {
		cmd := &cobra.Command{Use: "root", RunE: func(_ *cobra.Command, _ []string) error { return nil }}
		InstallDebugOptions(cmd)
		require.NoError(t, cmd.ParseFlags([]string{"--debug-options"}))

		assert.True(t, IsDebugOptionsActive(cmd))
	})
}

func TestWriteDebugOptionsReturnsWriterErrors(t *testing.T) {
	// Arrange
	cmd := &cobra.Command{Use: "root", RunE: func(_ *cobra.Command, _ []string) error { return nil }}
	InstallDebugOptions(cmd)
	cmd.SetArgs([]string{"--debug-options"})
	require.NoError(t, cmd.ParseFlags([]string{"--debug-options"}))

	// Act
	err := writeDebugOptions(cmd, failingDebugWriter{})

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "write failed")
}

func TestWriteDebugOptionsReturnsJSONWriterErrors(t *testing.T) {
	// Arrange
	cmd := &cobra.Command{Use: "root", RunE: func(_ *cobra.Command, _ []string) error { return nil }}
	InstallDebugOptions(cmd)
	require.NoError(t, cmd.ParseFlags([]string{"--debug-options=json"}))

	// Act
	err := writeDebugOptions(cmd, failingDebugWriter{})

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "write failed")
}
