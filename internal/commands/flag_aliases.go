package commands

import (
	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/models"
	"github.com/spf13/cobra"
)

// registerOrderFlagAliases registers --instruction and --order-type aliases
// for their primary counterparts (--action and --type) on the given command.
// Aliases are only registered if their primary flags exist on the command.
// Both aliases are visible in --help output.
func registerOrderFlagAliases(cmd *cobra.Command) {
	// Register --instruction alias only if --action exists
	if cmd.Flags().Lookup("action") != nil {
		var instructionAlias string
		cmd.Flags().StringVar(&instructionAlias, "instruction", "", "Alias for --action")
	}

	// Register --order-type alias only if --type exists
	if cmd.Flags().Lookup("type") != nil {
		var orderTypeAlias string
		cmd.Flags().StringVar(&orderTypeAlias, "order-type", "", "Alias for --type")
	}
}

// resolveOrderFlagAliases handles conflicts and copies alias values to their
// primary counterparts. It returns a ValidationError if both a primary flag
// and its alias are set. If only the alias is set, its value is copied to
// the primary field. If orderType is nil, --order-type resolution is skipped.
func resolveOrderFlagAliases(cmd *cobra.Command, action *models.Instruction, orderType *models.OrderType) error {
	// Handle --action / --instruction conflict and resolution
	actionFlag := cmd.Flags().Lookup("action")
	if actionFlag != nil {
		instructionFlag := cmd.Flags().Lookup("instruction")
		if instructionFlag != nil {
			actionChanged := actionFlag.Changed
			instructionChanged := instructionFlag.Changed

			// Both flags set is a conflict
			if actionChanged && instructionChanged {
				return apperr.NewValidationError(
					"cannot use both --action and --instruction flags",
					nil,
				)
			}

			// Only alias set: copy to primary
			if instructionChanged && !actionChanged {
				instructionValue, _ := cmd.Flags().GetString("instruction")
				*action = models.Instruction(instructionValue)
			}
		}
	}

	// Handle --type / --order-type conflict and resolution (skip if orderType is nil)
	if orderType != nil {
		typeFlag := cmd.Flags().Lookup("type")
		if typeFlag != nil {
			orderTypeFlag := cmd.Flags().Lookup("order-type")
			if orderTypeFlag != nil {
				typeChanged := typeFlag.Changed
				orderTypeChanged := orderTypeFlag.Changed

				// Both flags set is a conflict
				if typeChanged && orderTypeChanged {
					return apperr.NewValidationError(
						"cannot use both --type and --order-type flags",
						nil,
					)
				}

				// Only alias set: copy to primary
				if orderTypeChanged && !typeChanged {
					orderTypeValue, _ := cmd.Flags().GetString("order-type")
					*orderType = models.OrderType(orderTypeValue)
				}
			}
		}
	}

	return nil
}
