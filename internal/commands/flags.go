package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

// flagString returns the value of a string flag. Panics if flag not defined (programmer error).
func flagString(cmd *cobra.Command, name string) string {
	val, err := cmd.Flags().GetString(name)
	if err != nil {
		panic(fmt.Sprintf("flagString: flag %q not defined: %v", name, err))
	}
	return val
}

// flagFloat64 returns the value of a float64 flag. Panics if flag not defined (programmer error).
func flagFloat64(cmd *cobra.Command, name string) float64 {
	val, err := cmd.Flags().GetFloat64(name)
	if err != nil {
		panic(fmt.Sprintf("flagFloat64: flag %q not defined: %v", name, err))
	}
	return val
}

// flagBool returns the value of a bool flag. Panics if flag not defined (programmer error).
func flagBool(cmd *cobra.Command, name string) bool {
	val, err := cmd.Flags().GetBool(name)
	if err != nil {
		panic(fmt.Sprintf("flagBool: flag %q not defined: %v", name, err))
	}
	return val
}

// flagInt returns the value of an int flag. Panics if flag not defined (programmer error).
func flagInt(cmd *cobra.Command, name string) int {
	val, err := cmd.Flags().GetInt(name)
	if err != nil {
		panic(fmt.Sprintf("flagInt: flag %q not defined: %v", name, err))
	}
	return val
}

// flagStringSlice returns the value of a string slice flag. Panics if flag not defined (programmer error).
func flagStringSlice(cmd *cobra.Command, name string) []string {
	val, err := cmd.Flags().GetStringSlice(name)
	if err != nil {
		panic(fmt.Sprintf("flagStringSlice: flag %q not defined: %v", name, err))
	}
	return val
}

// flagIntSlice returns the value of an int slice flag. Panics if flag not defined (programmer error).
func flagIntSlice(cmd *cobra.Command, name string) []int {
	val, err := cmd.Flags().GetIntSlice(name)
	if err != nil {
		panic(fmt.Sprintf("flagIntSlice: flag %q not defined: %v", name, err))
	}
	return val
}
