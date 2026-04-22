package commands

import (
	schwabErrors "github.com/major/schwab-agent/internal/errors"
)

// requireArg returns a ValidationError if value is empty.
// Use this for required positional arguments and flag values.
func requireArg(value, name string) error {
	if value == "" {
		return schwabErrors.NewValidationError(name+" is required", nil)
	}

	return nil
}
