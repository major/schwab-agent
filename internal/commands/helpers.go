package commands

import (
	"github.com/major/schwab-agent/internal/apperr"
)

// requireArg returns a ValidationError if value is empty.
// Use this for required positional arguments and flag values.
func requireArg(value, name string) error {
	if value == "" {
		return apperr.NewValidationError(name+" is required", nil)
	}

	return nil
}
