package commands

import (
	"context"
	"fmt"

	"github.com/major/schwab-agent/internal/apperr"
)

// optionChainOpts holds the flags for the porcelain option chain command.
// Struct tags drive defineCobraFlags registration. Type and StrikeRange use
// named enum types so Cobra validates allowed values at flag-parse time.
type optionChainOpts struct {
	Type        chainContractType `flag:"type"         flagdescr:"Contract type: CALL, PUT, or ALL"                    default:"ALL"`
	DTE         int               `flag:"dte"          flagdescr:"Target days to expiration"`
	Expiration  string            `flag:"expiration"   flagdescr:"Exact expiration date (YYYY-MM-DD)"`
	Fields      string            `flag:"fields"       flagdescr:"Comma-separated field names for column projection"`
	StrikeCount int               `flag:"strike-count" flagdescr:"Number of strikes around ATM"`
	Strike      float64           `flag:"strike"       flagdescr:"Exact strike price"`
	StrikeMin   float64           `flag:"strike-min"   flagdescr:"Minimum strike price"`
	StrikeMax   float64           `flag:"strike-max"   flagdescr:"Maximum strike price"`
	StrikeRange strikeRange       `flag:"strike-range" flagdescr:"Moneyness filter: ITM, NTM, OTM, SAK, SBK, SNK, ALL"`
}

// Validate implements cobraValidatable so validateCobraOptions can delegate
// to the existing constraint checks.
func (o *optionChainOpts) Validate(_ context.Context) []error {
	return validateOptionChainOpts(o)
}

// optionContractOpts holds the flags for porcelain option contract commands.
type optionContractOpts struct {
	Expiration string  `flag:"expiration" flagdescr:"Option expiration date (YYYY-MM-DD)"`
	Strike     float64 `flag:"strike"     flagdescr:"Option strike price"`
	Call       bool    `flag:"call"       flagdescr:"Select the call contract"`
	Put        bool    `flag:"put"        flagdescr:"Select the put contract"`
}

// Validate implements cobraValidatable so validateCobraOptions can delegate
// to the existing constraint checks.
func (o *optionContractOpts) Validate(_ context.Context) []error {
	return validateOptionContractOpts(o)
}

// validOptionChainTypes lists the accepted values for optionChainOpts.Type.
//
//nolint:gochecknoglobals // package-level constant map; Go cannot use const for maps
var validOptionChainTypes = map[string]bool{
	string(chainContractTypeCall): true,
	string(chainContractTypePut):  true,
	string(chainContractTypeAll):  true,
}

// validateOptionChainOpts checks optionChainOpts for constraint violations.
// Returns a slice of errors (matching the cobraValidatable pattern).
func validateOptionChainOpts(opts *optionChainOpts) []error {
	var errs []error

	// Type must be CALL, PUT, or ALL. Cobra enum validation catches most
	// invalid values at parse time, but direct callers (tests) may bypass
	// flag parsing and set the field directly.
	if opts.Type != "" && !validOptionChainTypes[string(opts.Type)] {
		errs = append(errs, apperr.NewValidationError(
			fmt.Sprintf("invalid type: %s; valid: CALL, PUT, ALL", opts.Type), nil,
		))
	}

	// DTE and Expiration are mutually exclusive.
	if opts.DTE > 0 && opts.Expiration != "" {
		errs = append(errs, apperr.NewValidationError(
			"dte and expiration are mutually exclusive; use one or the other", nil,
		))
	}

	// StrikeCount must be non-negative.
	if opts.StrikeCount < 0 {
		errs = append(errs, apperr.NewValidationError(
			fmt.Sprintf("strike-count must be >= 0, got %d", opts.StrikeCount), nil,
		))
	}

	// StrikeCount cannot be combined with a strike range filter.
	if opts.StrikeCount > 0 && (opts.StrikeMin > 0 || opts.StrikeMax > 0) {
		errs = append(errs, apperr.NewValidationError(
			"strike-count is mutually exclusive with strike-min and strike-max", nil,
		))
	}

	// Strike selector mutual exclusivity: --strike is exclusive with count, min/max, range.
	hasStrikeRange := string(opts.StrikeRange) != ""
	if opts.Strike > 0 && (opts.StrikeCount > 0 || opts.StrikeMin > 0 || opts.StrikeMax > 0 || hasStrikeRange) {
		errs = append(errs, apperr.NewValidationError(
			"strike is mutually exclusive with strike-count, strike-min/max, and strike-range", nil,
		))
	}

	// StrikeCount and StrikeRange are mutually exclusive.
	if opts.StrikeCount > 0 && hasStrikeRange {
		errs = append(errs, apperr.NewValidationError(
			"strike-count and strike-range are mutually exclusive", nil,
		))
	}

	// StrikeMin/Max pair and StrikeRange are mutually exclusive.
	if (opts.StrikeMin > 0 || opts.StrikeMax > 0) && hasStrikeRange {
		errs = append(errs, apperr.NewValidationError(
			"strike-min/max and strike-range are mutually exclusive", nil,
		))
	}

	// StrikeMin must not exceed StrikeMax (when both are set).
	if opts.StrikeMin > 0 && opts.StrikeMax > 0 && opts.StrikeMin > opts.StrikeMax {
		errs = append(errs, apperr.NewValidationError(
			fmt.Sprintf("strike-min (%g) must be <= strike-max (%g)", opts.StrikeMin, opts.StrikeMax), nil,
		))
	}

	return errs
}

// validateOptionContractOpts checks optionContractOpts for constraint
// violations. Returns a slice of errors (matching the cobraValidatable pattern).
func validateOptionContractOpts(opts *optionContractOpts) []error {
	var errs []error

	// Expiration is required.
	if opts.Expiration == "" {
		errs = append(errs, apperr.NewValidationError(
			"expiration is required", nil,
		))
	}

	// Strike must be > 0.
	if opts.Strike <= 0 {
		errs = append(errs, apperr.NewValidationError(
			fmt.Sprintf("strike must be > 0, got %g", opts.Strike), nil,
		))
	}

	// Call and Put are mutually exclusive, and exactly one is required.
	if opts.Call && opts.Put {
		errs = append(errs, apperr.NewValidationError(
			"call and put are mutually exclusive; specify one", nil,
		))
	} else if !opts.Call && !opts.Put {
		errs = append(errs, apperr.NewValidationError(
			"call or put is required; specify one", nil,
		))
	}

	return errs
}
