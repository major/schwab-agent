package commands

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/models"
)

// registerOrderFlagAliases registers --instruction and --order-type aliases
// for their primary counterparts (--action and --type) on the given command.
// Aliases are only registered if their primary flags exist on the command.
// Both aliases are visible in --help output.
func registerOrderFlagAliases(cmd *cobra.Command) {
	// Register --instruction alias only if --action exists and --instruction
	// is not already registered (idempotent for tree-walker safety).
	// Keep the usage text explicit so generated docs and help output make the
	// relationship to the primary flag obvious.
	actionFlag := cmd.Flags().Lookup("action")
	if actionFlag != nil && cmd.Flags().Lookup("instruction") == nil {
		var instructionAlias string
		cmd.Flags().StringVar(&instructionAlias, "instruction", "", "alias for --action (order instruction)")

		// Replace the single-flag required constraint on --action with
		// a Cobra group constraint: exactly one of --action or --instruction
		// must be set. Without this, Cobra rejects --instruction usage because
		// --action is still marked individually required.
		if wasRequired := clearRequiredAnnotation(actionFlag); wasRequired {
			cmd.MarkFlagsOneRequired("action", "instruction")
		}
		cmd.MarkFlagsMutuallyExclusive("action", "instruction")
	}

	// Register --order-type alias only if --type exists and --order-type
	// is not already registered. --type is typically optional (not required),
	// so no required annotation transfer is needed.
	if cmd.Flags().Lookup("type") != nil && cmd.Flags().Lookup("order-type") == nil {
		var orderTypeAlias string
		cmd.Flags().StringVar(&orderTypeAlias, "order-type", "", "alias for --type (order type)")
		cmd.MarkFlagsMutuallyExclusive("type", "order-type")
	}
}

// clearRequiredAnnotation removes Cobra's required annotation from a flag,
// returning true if it was previously required.
func clearRequiredAnnotation(f *pflag.Flag) bool {
	if f.Annotations == nil {
		return false
	}

	vals, ok := f.Annotations[cobra.BashCompOneRequiredFlag]
	if !ok || len(vals) == 0 || vals[0] != "true" {
		return false
	}

	delete(f.Annotations, cobra.BashCompOneRequiredFlag)

	return true
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

// resolveOrderFlagAliasesViaFlags resolves --instruction and --order-type alias
// flag values by copying them to their primary Cobra flags (--action and --type)
// before RunE reads the bound opts struct. This works with generic factory
// patterns where the opts struct type isn't accessible. The function is a safe
// no-op for commands that don't have the alias flags registered.
func resolveOrderFlagAliasesViaFlags(cmd *cobra.Command) error {
	// Resolve --instruction -> --action
	instructionFlag := cmd.Flags().Lookup("instruction")
	if instructionFlag != nil && instructionFlag.Changed {
		actionFlag := cmd.Flags().Lookup("action")
		if actionFlag != nil && actionFlag.Changed {
			return apperr.NewValidationError(
				"cannot use both --action and --instruction flags",
				nil,
			)
		}

		if err := cmd.Flags().Set("action", instructionFlag.Value.String()); err != nil {
			return err
		}
	}

	// Resolve --order-type -> --type
	orderTypeFlag := cmd.Flags().Lookup("order-type")
	if orderTypeFlag != nil && orderTypeFlag.Changed {
		typeFlag := cmd.Flags().Lookup("type")
		if typeFlag != nil && typeFlag.Changed {
			return apperr.NewValidationError(
				"cannot use both --type and --order-type flags",
				nil,
			)
		}

		if err := cmd.Flags().Set("type", orderTypeFlag.Value.String()); err != nil {
			return err
		}
	}

	return nil
}

// RegisterOrderFlagAliasesOnTree walks the command tree and registers
// --instruction and --order-type aliases on all commands that have --action
// and/or --type flags. Call this after the command tree is fully built so aliases
// are added after all command setup is complete.
func RegisterOrderFlagAliasesOnTree(root *cobra.Command) {
	walkCommandTree(root, func(cmd *cobra.Command) {
		registerOrderFlagAliases(cmd)
	})
}

// walkCommandTree recursively visits every command in the tree.
func walkCommandTree(cmd *cobra.Command, fn func(*cobra.Command)) {
	fn(cmd)
	for _, child := range cmd.Commands() {
		walkCommandTree(child, fn)
	}
}
