package commands

import (
	"context"
	"fmt"

	"github.com/major/schwab-agent/internal/apperr"
)

// optionScreenOpts holds the flags for the option screen command. Struct tags
// drive defineCobraFlags registration. Type reuses chainContractType so Cobra
// validates allowed values at flag-parse time.
//
// The *Set fields are not flags; they track whether the corresponding flag was
// explicitly provided via cmd.Flags().Changed(). This distinguishes "user
// passed 0" from "default zero value" for DTE and delta bounds where 0 is a
// valid filter value (e.g., 0DTE screening, delta=0 for deep OTM).
type optionScreenOpts struct {
	Type         chainContractType `flag:"type"           flagdescr:"Contract type: CALL, PUT, or ALL"                             default:"ALL"`
	DTEMin       int               `flag:"dte-min"        flagdescr:"Minimum days to expiration (inclusive)"`
	DTEMax       int               `flag:"dte-max"        flagdescr:"Maximum days to expiration (inclusive)"`
	DeltaMin     float64           `flag:"delta-min"      flagdescr:"Minimum delta (raw API value; puts are negative)"`
	DeltaMax     float64           `flag:"delta-max"      flagdescr:"Maximum delta (raw API value; puts are negative)"`
	MinBid       float64           `flag:"min-bid"        flagdescr:"Minimum bid price per share (short premium floor)"`
	MaxAsk       float64           `flag:"max-ask"        flagdescr:"Maximum ask price per share (long premium cap)"`
	MinVolume    int               `flag:"min-volume"     flagdescr:"Minimum total volume"`
	MinOI        int               `flag:"min-oi"         flagdescr:"Minimum open interest"`
	MaxSpreadPct float64           `flag:"max-spread-pct" flagdescr:"Maximum bid-ask spread as percentage of mark price"`
	Fields       string            `flag:"fields"         flagdescr:"Comma-separated field names for column projection"`
	Sort         string            `flag:"sort"           flagdescr:"Sort output by field:direction (e.g. delta:asc, volume:desc)"`
	Limit        int               `flag:"limit"          flagdescr:"Maximum number of rows to return"`

	// Populated by runOptionScreen from cmd.Flags().Changed(), not by defineCobraFlags.
	dteMinSet   bool
	dteMaxSet   bool
	deltaMinSet bool
	deltaMaxSet bool
}

// Validate implements cobraValidatable so validateCobraOptions can delegate
// to the constraint checks below.
func (o *optionScreenOpts) Validate(_ context.Context) []error {
	return validateOptionScreenOpts(o)
}

// validateOptionScreenOpts checks optionScreenOpts for constraint violations.
// Returns a slice of errors (matching the cobraValidatable pattern).
func validateOptionScreenOpts(opts *optionScreenOpts) []error {
	var errs []error

	// Type must be CALL, PUT, or ALL.
	if opts.Type != "" && !validOptionChainTypes[string(opts.Type)] {
		errs = append(errs, apperr.NewValidationError(
			fmt.Sprintf("invalid type: %s; valid: CALL, PUT, ALL", opts.Type), nil,
		))
	}

	// DTE bounds must be non-negative.
	if opts.DTEMin < 0 {
		errs = append(errs, apperr.NewValidationError(
			fmt.Sprintf("dte-min must be >= 0, got %d", opts.DTEMin), nil,
		))
	}
	if opts.DTEMax < 0 {
		errs = append(errs, apperr.NewValidationError(
			fmt.Sprintf("dte-max must be >= 0, got %d", opts.DTEMax), nil,
		))
	}

	// DTEMin must not exceed DTEMax (when both are explicitly set).
	if opts.dteMinSet && opts.dteMaxSet && opts.DTEMin > opts.DTEMax {
		errs = append(errs, apperr.NewValidationError(
			fmt.Sprintf("dte-min (%d) must be <= dte-max (%d)", opts.DTEMin, opts.DTEMax), nil,
		))
	}

	// DeltaMin must not exceed DeltaMax when both bounds are explicitly set.
	// For puts, delta values are negative: delta-min=-0.30, delta-max=-0.20 is valid.
	if opts.deltaMinSet && opts.deltaMaxSet && opts.DeltaMin > opts.DeltaMax {
		errs = append(errs, apperr.NewValidationError(
			fmt.Sprintf("delta-min (%g) must be <= delta-max (%g)", opts.DeltaMin, opts.DeltaMax), nil,
		))
	}

	// Price filters must be non-negative.
	if opts.MinBid < 0 {
		errs = append(errs, apperr.NewValidationError(
			fmt.Sprintf("min-bid must be >= 0, got %g", opts.MinBid), nil,
		))
	}
	if opts.MaxAsk < 0 {
		errs = append(errs, apperr.NewValidationError(
			fmt.Sprintf("max-ask must be >= 0, got %g", opts.MaxAsk), nil,
		))
	}

	// Volume and OI minimums must be non-negative.
	if opts.MinVolume < 0 {
		errs = append(errs, apperr.NewValidationError(
			fmt.Sprintf("min-volume must be >= 0, got %d", opts.MinVolume), nil,
		))
	}
	if opts.MinOI < 0 {
		errs = append(errs, apperr.NewValidationError(
			fmt.Sprintf("min-oi must be >= 0, got %d", opts.MinOI), nil,
		))
	}

	// Spread percentage must be non-negative.
	if opts.MaxSpreadPct < 0 {
		errs = append(errs, apperr.NewValidationError(
			fmt.Sprintf("max-spread-pct must be >= 0, got %g", opts.MaxSpreadPct), nil,
		))
	}

	// Limit must be non-negative.
	if opts.Limit < 0 {
		errs = append(errs, apperr.NewValidationError(
			fmt.Sprintf("limit must be >= 0, got %d", opts.Limit), nil,
		))
	}

	// Sort must be field:direction format when set.
	if opts.Sort != "" {
		if err := validateScreenSort(opts.Sort); err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}

// screenSortableFields lists the fields that support sorting in the screen command.
//
//nolint:gochecknoglobals,goconst // package-level constant map; Go cannot use const for maps
var screenSortableFields = map[string]bool{
	"expiry": true, "strike": true, "bid": true, "ask": true, "mark": true,
	"last": true, "delta": true, "gamma": true, "theta": true, "vega": true,
	"rho": true, "iv": true, "oi": true, "volume": true, fieldDTE: true,
	fieldSpreadPct: true,
}

// validateScreenSort validates the --sort flag format (field:direction).
func validateScreenSort(sortFlag string) error {
	parts := splitSortFlag(sortFlag)
	if len(parts) != sortPartCount {
		return apperr.NewValidationError(
			fmt.Sprintf("sort must be field:direction (e.g. delta:asc), got %q", sortFlag), nil,
		)
	}

	field, direction := parts[0], parts[1]
	if !screenSortableFields[field] {
		return apperr.NewValidationError(
			fmt.Sprintf("unsupported sort field: %q", field), nil,
		)
	}
	if direction != sortDirAsc && direction != sortDirDesc {
		return apperr.NewValidationError(
			fmt.Sprintf("sort direction must be asc or desc, got %q", direction), nil,
		)
	}

	return nil
}

// splitSortFlag splits a "field:direction" string into its components.
func splitSortFlag(s string) []string {
	idx := -1
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == ':' {
			idx = i
			break
		}
	}
	if idx < 0 {
		return []string{s}
	}
	return []string{s[:idx], s[idx+1:]}
}
