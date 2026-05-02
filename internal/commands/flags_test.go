package commands

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlagString(t *testing.T) {
	t.Run("returns string value when flag is defined", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.Flags().String("name", "default", "test flag")
		cmd.SetArgs([]string{"--name", "hello"})
		_ = cmd.Execute()

		val := flagString(cmd, "name")
		assert.Equal(t, "hello", val)
	})

	t.Run("returns default value when flag not provided", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.Flags().String("name", "default", "test flag")
		cmd.SetArgs([]string{})
		_ = cmd.Execute()

		val := flagString(cmd, "name")
		assert.Equal(t, "default", val)
	})

	t.Run("panics when flag not defined", func(t *testing.T) {
		cmd := &cobra.Command{}
		assert.Panics(t, func() {
			flagString(cmd, "undefined")
		})
	})
}

func TestFlagFloat64(t *testing.T) {
	t.Run("returns float64 value when flag is defined", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.Flags().Float64("price", 0.0, "test flag")
		cmd.SetArgs([]string{"--price", "123.45"})
		_ = cmd.Execute()

		val := flagFloat64(cmd, "price")
		assert.Equal(t, 123.45, val)
	})

	t.Run("panics when flag not defined", func(t *testing.T) {
		cmd := &cobra.Command{}
		assert.Panics(t, func() {
			flagFloat64(cmd, "undefined")
		})
	})
}

func TestFlagBool(t *testing.T) {
	t.Run("returns bool value when flag is defined", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.Flags().Bool("verbose", false, "test flag")
		cmd.SetArgs([]string{"--verbose"})
		_ = cmd.Execute()

		val := flagBool(cmd, "verbose")
		assert.True(t, val)
	})

	t.Run("returns false when flag not provided", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.Flags().Bool("verbose", false, "test flag")
		cmd.SetArgs([]string{})
		_ = cmd.Execute()

		val := flagBool(cmd, "verbose")
		assert.False(t, val)
	})

	t.Run("panics when flag not defined", func(t *testing.T) {
		cmd := &cobra.Command{}
		assert.Panics(t, func() {
			flagBool(cmd, "undefined")
		})
	})
}

func TestFlagInt(t *testing.T) {
	t.Run("returns int value when flag is defined", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.Flags().Int("count", 0, "test flag")
		cmd.SetArgs([]string{"--count", "42"})
		_ = cmd.Execute()

		val := flagInt(cmd, "count")
		assert.Equal(t, 42, val)
	})

	t.Run("panics when flag not defined", func(t *testing.T) {
		cmd := &cobra.Command{}
		assert.Panics(t, func() {
			flagInt(cmd, "undefined")
		})
	})
}

func TestFlagStringSlice(t *testing.T) {
	t.Run("returns string slice value when flag is defined", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.Flags().StringSlice("items", []string{}, "test flag")
		cmd.SetArgs([]string{"--items", "a", "--items", "b"})
		_ = cmd.Execute()

		val := flagStringSlice(cmd, "items")
		assert.Equal(t, []string{"a", "b"}, val)
	})

	t.Run("returns empty slice when flag not provided", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.Flags().StringSlice("items", []string{}, "test flag")
		cmd.SetArgs([]string{})
		_ = cmd.Execute()

		val := flagStringSlice(cmd, "items")
		assert.Equal(t, []string{}, val)
	})

	t.Run("panics when flag not defined", func(t *testing.T) {
		cmd := &cobra.Command{}
		assert.Panics(t, func() {
			flagStringSlice(cmd, "undefined")
		})
	})
}

func TestFlagIntSlice(t *testing.T) {
	t.Run("returns int slice value when flag is defined", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.Flags().IntSlice("numbers", []int{}, "test flag")
		cmd.SetArgs([]string{"--numbers", "1", "--numbers", "2"})
		_ = cmd.Execute()

		val := flagIntSlice(cmd, "numbers")
		assert.Equal(t, []int{1, 2}, val)
	})

	t.Run("panics when flag not defined", func(t *testing.T) {
		cmd := &cobra.Command{}
		assert.Panics(t, func() {
			flagIntSlice(cmd, "undefined")
		})
	})
}

func TestFlagPanic(t *testing.T) {
	t.Run("panic message includes flag name for string", func(t *testing.T) {
		cmd := &cobra.Command{}
		defer func() {
			r := recover()
			require.NotNil(t, r)
			msg := r.(string)
			assert.Contains(t, msg, "flagString")
			assert.Contains(t, msg, "missing_flag")
		}()
		flagString(cmd, "missing_flag")
	})

	t.Run("panic message includes flag name for float64", func(t *testing.T) {
		cmd := &cobra.Command{}
		defer func() {
			r := recover()
			require.NotNil(t, r)
			msg := r.(string)
			assert.Contains(t, msg, "flagFloat64")
			assert.Contains(t, msg, "missing_flag")
		}()
		flagFloat64(cmd, "missing_flag")
	})

	t.Run("panic message includes flag name for bool", func(t *testing.T) {
		cmd := &cobra.Command{}
		defer func() {
			r := recover()
			require.NotNil(t, r)
			msg := r.(string)
			assert.Contains(t, msg, "flagBool")
			assert.Contains(t, msg, "missing_flag")
		}()
		flagBool(cmd, "missing_flag")
	})

	t.Run("panic message includes flag name for int", func(t *testing.T) {
		cmd := &cobra.Command{}
		defer func() {
			r := recover()
			require.NotNil(t, r)
			msg := r.(string)
			assert.Contains(t, msg, "flagInt")
			assert.Contains(t, msg, "missing_flag")
		}()
		flagInt(cmd, "missing_flag")
	})

	t.Run("panic message includes flag name for string slice", func(t *testing.T) {
		cmd := &cobra.Command{}
		defer func() {
			r := recover()
			require.NotNil(t, r)
			msg := r.(string)
			assert.Contains(t, msg, "flagStringSlice")
			assert.Contains(t, msg, "missing_flag")
		}()
		flagStringSlice(cmd, "missing_flag")
	})

	t.Run("panic message includes flag name for int slice", func(t *testing.T) {
		cmd := &cobra.Command{}
		defer func() {
			r := recover()
			require.NotNil(t, r)
			msg := r.(string)
			assert.Contains(t, msg, "flagIntSlice")
			assert.Contains(t, msg, "missing_flag")
		}()
		flagIntSlice(cmd, "missing_flag")
	})
}
