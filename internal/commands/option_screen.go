package commands

import (
	"cmp"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/major/schwab-go/schwab/marketdata"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/models"
	"github.com/major/schwab-agent/internal/output"
)

const (
	// fieldSpreadPct is the computed bid-ask spread percentage field name.
	fieldSpreadPct = "spreadPct"

	// fieldDTE is the computed days-to-expiration field name.
	fieldDTE = "dte"

	// sortDirAsc is the ascending sort direction.
	sortDirAsc = "asc"

	// sortDirDesc is the descending sort direction.
	sortDirDesc = "desc"

	// sortPartCount is the expected number of parts in a "field:direction" sort flag.
	sortPartCount = 2
)

// screenDefaultColumns is the default column order for the screen view.
// Includes spreadPct and dte beyond the chain defaults since these are
// important for screening workflows.
//
//nolint:gochecknoglobals,goconst // package-level constant slice; field names are clearer inline
var screenDefaultColumns = []string{
	"expiry", fieldDTE, "strike", "cp", "symbol",
	"bid", "ask", "mark", fieldSpreadPct,
	"delta", "iv", "oi", "volume",
}

// screenValidFields is the allowlist of field names accepted by the screen
// command. Extends the chain valid fields with the computed spreadPct field.
//
//nolint:gochecknoglobals,goconst // package-level constant slice; field names are clearer inline
var screenValidFields = []string{
	"expiry", "strike", "cp", "symbol",
	"bid", "ask", "mark", "last",
	"delta", "gamma", "theta", "vega", "rho",
	"iv", "oi", "volume", "itm", fieldDTE,
	fieldSpreadPct,
}

// screenRow holds a contract's data along with sort keys and computed fields
// used by the screening pipeline. Embedding chainRow would couple us to its
// layout, so we keep the fields flat for clarity.
type screenRow struct {
	expiry    string
	strike    float64
	putCall   string
	dte       int
	contract  *models.OptionContract
	spreadPct float64
	values    []any
}

// newOptionScreenCmd returns the porcelain command for option contract screening.
// It fetches raw chain data, applies client-side filters for delta, premium,
// volume, open interest, and bid-ask spread, then emits matching contracts.
func newOptionScreenCmd(ref *client.Ref, w io.Writer) *cobra.Command {
	opts := &optionScreenOpts{}
	cmd := &cobra.Command{
		Use:   "screen <symbol>",
		Short: "Screen option contracts by delta, premium, volume, and spread",
		Long: `Screen option contracts for a single underlying symbol. Fetches the full
chain from the Schwab API, then applies client-side filters for delta range,
premium thresholds, volume, open interest, and bid-ask spread width. Use
--dte-min and --dte-max to narrow expiration cycles. All premium values are
per-share (multiply by 100 for standard contract cost). Delta values use raw
API conventions: calls are positive, puts are negative.`,
		Example: `  # Short put candidates: 30-45 DTE, delta -0.30 to -0.20, min $1.00 credit
  schwab-agent option screen AAPL --type PUT --dte-min 30 --dte-max 45 --delta-min -0.30 --delta-max -0.20 --min-bid 1.00

  # Long call candidates: reasonable spread, decent volume
  schwab-agent option screen AAPL --type CALL --dte-min 20 --dte-max 60 --max-ask 5.00 --max-spread-pct 10 --min-volume 100

  # High-IV puts with liquidity
  schwab-agent option screen SPY --type PUT --min-oi 500 --min-volume 200 --sort iv:desc --limit 20

  # All contracts with custom fields
  schwab-agent option screen TSLA --fields strike,bid,ask,delta,theta,iv,dte`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOptionScreen(cmd, ref, w, opts, args)
		},
	}

	defineCobraFlags(cmd, opts)
	cmd.SetFlagErrorFunc(normalizeFlagValidationErrorFunc)

	return cmd
}

// runOptionScreen is the RunE body for the option screen command.
func runOptionScreen(
	cmd *cobra.Command,
	ref *client.Ref,
	w io.Writer,
	opts *optionScreenOpts,
	args []string,
) error {
	// Record which zero-sensitive flags were explicitly provided so filters
	// and server-side params honor --dte-min 0 and --delta-min 0 correctly.
	opts.dteMinSet = cmd.Flags().Changed("dte-min")
	opts.dteMaxSet = cmd.Flags().Changed("dte-max")
	opts.deltaMinSet = cmd.Flags().Changed("delta-min")
	opts.deltaMaxSet = cmd.Flags().Changed("delta-max")

	if err := validateCobraOptions(cmd.Context(), opts); err != nil {
		return err
	}

	symbol := strings.ToUpper(strings.TrimSpace(args[0]))
	if err := requireArg(symbol, "symbol"); err != nil {
		return err
	}

	// Build API params, pushing server-side filters where possible.
	params := screenChainParams(symbol, opts)

	chain, err := ref.OptionChain(cmd.Context(), params)
	if err != nil {
		return err
	}

	if chain == nil || (len(chain.CallExpDateMap) == 0 && len(chain.PutExpDateMap) == 0) {
		return apperr.NewSymbolNotFoundError("no option chain data for "+symbol, nil)
	}

	// Resolve column set: custom fields or screen defaults.
	columns, err := resolveScreenColumns(opts.Fields)
	if err != nil {
		return err
	}

	// Build screen rows from all contracts, computing DTE and spreadPct.
	rows := buildScreenRows(chain, columns, time.Now())

	// Count total before filtering for metadata.
	totalScanned := len(rows)

	// Apply the client-side filter pipeline.
	rows = applyScreenFilters(rows, opts)

	// Apply sorting if requested.
	if opts.Sort != "" {
		rows = sortScreenRows(rows, opts.Sort, columns)
	}

	// Apply limit if requested.
	if opts.Limit > 0 && len(rows) > opts.Limit {
		rows = rows[:opts.Limit]
	}

	// Build the filters-applied list for metadata.
	filtersApplied := buildFiltersApplied(opts)

	// Extract row values for JSON output.
	result := make([][]any, len(rows))
	for i, r := range rows {
		result[i] = r.values
	}

	// Determine the underlying price for the output envelope.
	var underlyingPrice any
	if chain.UnderlyingPrice != nil {
		underlyingPrice = *chain.UnderlyingPrice
	}

	meta := output.NewMetadata()
	meta.Returned = len(result)

	return output.WriteSuccess(w, map[string]any{
		keyUnderlying:     symbol,
		"underlyingPrice": underlyingPrice,
		keyColumns:        columns,
		keyRows:           result,
		keyRowCount:       len(result),
		"filtersApplied":  filtersApplied,
		"totalScanned":    totalScanned,
	}, meta)
}

// screenChainParams builds API parameters for the screen command, pushing
// server-side filters where Schwab supports them.
func screenChainParams(symbol string, opts *optionScreenOpts) *marketdata.OptionChainParams {
	params := &marketdata.OptionChainParams{
		Symbol:                 symbol,
		ContractType:           marketdata.OptionChainContractType(opts.Type),
		Strategy:               marketdata.OptionChainStrategySingle,
		IncludeUnderlyingQuote: true,
	}

	// Push DTE range to server-side fromDate/toDate. This reduces the
	// payload size when a DTE window is specified. Uses dteMinSet/dteMaxSet
	// so --dte-min 0 (0DTE screening) still generates a fromDate.
	now := time.Now()
	if opts.dteMinSet {
		fromDate := now.AddDate(0, 0, opts.DTEMin)
		params.FromDate = fromDate.Format("2006-01-02")
	}
	if opts.dteMaxSet {
		toDate := now.AddDate(0, 0, opts.DTEMax)
		params.ToDate = toDate.Format("2006-01-02")
	}

	return params
}

// resolveScreenColumns parses a comma-separated --fields value into the
// column list, falling back to screen defaults when empty.
func resolveScreenColumns(fieldsFlag string) ([]string, error) {
	if fieldsFlag == "" {
		return screenDefaultColumns, nil
	}

	parts := strings.Split(fieldsFlag, ",")
	fields := make([]string, 0, len(parts))
	for _, f := range parts {
		trimmed := strings.TrimSpace(f)
		if trimmed != "" {
			fields = append(fields, trimmed)
		}
	}

	// Validate against the screen-specific field allowlist.
	validSet := make(map[string]bool, len(screenValidFields))
	for _, f := range screenValidFields {
		validSet[f] = true
	}
	for _, f := range fields {
		if !validSet[f] {
			return nil, apperr.NewValidationError(
				"unknown field: "+f+"; valid fields: "+strings.Join(screenValidFields, ", "),
				nil,
			)
		}
	}

	return fields, nil
}

// buildScreenRows collects all contracts from a chain, computing DTE and
// spreadPct for each. Returns unsorted rows.
func buildScreenRows(chain *models.OptionChain, columns []string, now time.Time) []screenRow {
	var rows []screenRow
	rows = collectScreenExpDateMap(rows, chain.CallExpDateMap, columns, now)
	rows = collectScreenExpDateMap(rows, chain.PutExpDateMap, columns, now)
	// Deterministic sort: expiry ASC, strike ASC, CALL before PUT.
	slices.SortFunc(rows, compareScreenRows)
	return rows
}

// collectScreenExpDateMap iterates an expiration-date map and appends
// screenRows for every contract found.
func collectScreenExpDateMap(
	rows []screenRow,
	expDateMap map[string]map[string][]*models.OptionContract,
	columns []string,
	now time.Time,
) []screenRow {
	for expKey, strikes := range expDateMap {
		expiry, _, _ := strings.Cut(expKey, ":")

		for _, contracts := range strikes {
			for _, c := range contracts {
				row := buildScreenRow(c, expiry, columns, now)
				rows = append(rows, row)
			}
		}
	}
	return rows
}

// buildScreenRow constructs a screenRow from a single OptionContract,
// computing DTE and bid-ask spread percentage.
func buildScreenRow(c *models.OptionContract, expiry string, columns []string, now time.Time) screenRow {
	strike := derefFloat64(c.StrikePrice)
	putCall := derefString(c.PutCall)

	// Compute DTE as integer days.
	dteVal := computeDTEInt(expiry, now)

	// Compute bid-ask spread as percentage of mark.
	spreadPct := computeSpreadPct(c)

	values := make([]any, len(columns))
	for i, col := range columns {
		values[i] = screenFieldValue(c, col, expiry, now, spreadPct)
	}

	return screenRow{
		expiry:    expiry,
		strike:    strike,
		putCall:   putCall,
		dte:       dteVal,
		contract:  c,
		spreadPct: spreadPct,
		values:    values,
	}
}

// screenFieldValue returns the value for a single column from an
// OptionContract, extending contractFieldValue with spreadPct.
func screenFieldValue(c *models.OptionContract, field, expiry string, now time.Time, spread float64) any {
	switch field {
	case fieldSpreadPct:
		if c.Bid == nil || c.Ask == nil || c.Mark == nil {
			return nil
		}
		return math.Round(spread*100) / 100 //nolint:mnd // round to 2 decimal places
	case fieldDTE:
		return computeDTE(expiry, now)
	default:
		return contractFieldValue(c, field, expiry)
	}
}

// computeDTEInt returns DTE as a plain int, or 0 on parse failure.
// Used for filter comparisons where nil is not needed.
func computeDTEInt(expiry string, now time.Time) int {
	raw := computeDTE(expiry, now)
	if v, ok := raw.(int); ok {
		return v
	}
	return 0
}

// computeSpreadPct computes bid-ask spread as a percentage of mark price.
// Returns 0 when any input is nil or mark is zero/negative.
func computeSpreadPct(c *models.OptionContract) float64 {
	if c.Bid == nil || c.Ask == nil || c.Mark == nil {
		return 0
	}
	mark := *c.Mark
	if mark <= 0 {
		return 0
	}
	return (*c.Ask - *c.Bid) / mark * 100 //nolint:mnd // percentage conversion
}

// compareScreenRows provides deterministic ordering: expiry ASC, strike ASC,
// CALL before PUT.
func compareScreenRows(a, b screenRow) int {
	if c := strings.Compare(a.expiry, b.expiry); c != 0 {
		return c
	}
	if a.strike != b.strike {
		if a.strike < b.strike {
			return -1
		}
		return 1
	}
	return strings.Compare(a.putCall, b.putCall)
}

// applyScreenFilters runs the client-side filter pipeline on screen rows.
// Each filter is applied independently; a contract must pass all filters.
func applyScreenFilters(rows []screenRow, opts *optionScreenOpts) []screenRow {
	filtered := make([]screenRow, 0, len(rows))
	for _, r := range rows {
		if passesScreenFilters(r, opts) {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// passesScreenFilters checks whether a single row passes all active filters.
func passesScreenFilters(r screenRow, opts *optionScreenOpts) bool {
	return passesDTEFilter(r, opts) &&
		passesDeltaFilter(r, opts) &&
		passesPremiumFilter(r, opts) &&
		passesLiquidityFilter(r, opts) &&
		passesSpreadFilter(r, opts)
}

// passesDTEFilter checks the DTE range bounds. Uses dteMinSet/dteMaxSet so
// --dte-min 0 and --dte-max 0 are honored for 0DTE screening.
func passesDTEFilter(r screenRow, opts *optionScreenOpts) bool {
	if opts.dteMinSet && r.dte < opts.DTEMin {
		return false
	}
	if opts.dteMaxSet && r.dte > opts.DTEMax {
		return false
	}
	return true
}

// passesDeltaFilter checks the delta range bounds. Uses deltaMinSet/deltaMaxSet
// so --delta-min 0 and --delta-max 0 are honored (delta=0 is meaningful for
// deep OTM options).
func passesDeltaFilter(r screenRow, opts *optionScreenOpts) bool {
	if opts.deltaMinSet {
		if r.contract.Delta == nil || *r.contract.Delta < opts.DeltaMin {
			return false
		}
	}
	if opts.deltaMaxSet {
		if r.contract.Delta == nil || *r.contract.Delta > opts.DeltaMax {
			return false
		}
	}
	return true
}

// passesPremiumFilter checks minimum bid (short premium floor) and maximum
// ask (long premium cap).
func passesPremiumFilter(r screenRow, opts *optionScreenOpts) bool {
	if opts.MinBid > 0 {
		if r.contract.Bid == nil || *r.contract.Bid < opts.MinBid {
			return false
		}
	}
	if opts.MaxAsk > 0 {
		if r.contract.Ask == nil || *r.contract.Ask > opts.MaxAsk {
			return false
		}
	}
	return true
}

// passesLiquidityFilter checks minimum volume and open interest.
func passesLiquidityFilter(r screenRow, opts *optionScreenOpts) bool {
	if opts.MinVolume > 0 {
		if r.contract.TotalVolume == nil || *r.contract.TotalVolume < int64(opts.MinVolume) {
			return false
		}
	}
	if opts.MinOI > 0 {
		if r.contract.OpenInterest == nil || *r.contract.OpenInterest < int64(opts.MinOI) {
			return false
		}
	}
	return true
}

// passesSpreadFilter checks the maximum bid-ask spread percentage. Contracts
// with nil bid/ask/mark fail since spread width cannot be computed.
func passesSpreadFilter(r screenRow, opts *optionScreenOpts) bool {
	if opts.MaxSpreadPct > 0 {
		if r.contract.Bid == nil || r.contract.Ask == nil || r.contract.Mark == nil {
			return false
		}
		if r.spreadPct > opts.MaxSpreadPct {
			return false
		}
	}
	return true
}

// sortScreenRows sorts rows by a field:direction specification.
func sortScreenRows(rows []screenRow, sortSpec string, columns []string) []screenRow {
	parts := splitSortFlag(sortSpec)
	if len(parts) != sortPartCount {
		return rows
	}

	field, direction := parts[0], parts[1]
	ascending := direction == sortDirAsc

	// Find the column index for the sort field.
	colIdx := -1
	for i, col := range columns {
		if col == field {
			colIdx = i
			break
		}
	}

	// If the sort field is not in the column projection, sort by the raw
	// contract data instead.
	if colIdx >= 0 {
		sortByColumnIndex(rows, colIdx, ascending)
	} else {
		sortByFieldName(rows, field, ascending)
	}

	return rows
}

// sortByColumnIndex sorts screen rows by the value at a specific column index.
func sortByColumnIndex(rows []screenRow, colIdx int, ascending bool) {
	slices.SortStableFunc(rows, func(a, b screenRow) int {
		aVal := a.values[colIdx]
		bVal := b.values[colIdx]
		cmp := compareAnyValues(aVal, bVal)
		if !ascending {
			cmp = -cmp
		}
		return cmp
	})
}

// sortByFieldName sorts screen rows by extracting the field value directly.
// Captures [time.Now] once so DTE computation is consistent across all comparisons.
func sortByFieldName(rows []screenRow, field string, ascending bool) {
	now := time.Now()
	slices.SortStableFunc(rows, func(a, b screenRow) int {
		aVal := screenFieldValue(a.contract, field, a.expiry, now, a.spreadPct)
		bVal := screenFieldValue(b.contract, field, b.expiry, now, b.spreadPct)
		cmp := compareAnyValues(aVal, bVal)
		if !ascending {
			cmp = -cmp
		}
		return cmp
	})
}

// compareAnyValues compares two any values for sorting. Nil values sort last.
func compareAnyValues(a, b any) int {
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return 1
	}
	if b == nil {
		return -1
	}

	switch av := a.(type) {
	case float64:
		if bv, ok := b.(float64); ok {
			return cmp.Compare(av, bv)
		}
	case int64:
		if bv, ok := b.(int64); ok {
			return cmp.Compare(av, bv)
		}
	case int:
		if bv, ok := b.(int); ok {
			return cmp.Compare(av, bv)
		}
	case string:
		if bv, ok := b.(string); ok {
			return cmp.Compare(av, bv)
		}
	}
	return 0
}

// buildFiltersApplied returns a list of human-readable filter descriptions
// for the metadata envelope.
func buildFiltersApplied(opts *optionScreenOpts) []string {
	var filters []string

	if opts.Type != "" && opts.Type != chainContractTypeAll {
		filters = append(filters, "type="+string(opts.Type))
	}
	if opts.dteMinSet {
		filters = append(filters, "dte-min="+intToStr(opts.DTEMin))
	}
	if opts.dteMaxSet {
		filters = append(filters, "dte-max="+intToStr(opts.DTEMax))
	}
	if opts.deltaMinSet {
		filters = append(filters, "delta-min="+floatToStr(opts.DeltaMin))
	}
	if opts.deltaMaxSet {
		filters = append(filters, "delta-max="+floatToStr(opts.DeltaMax))
	}
	if opts.MinBid > 0 {
		filters = append(filters, "min-bid="+floatToStr(opts.MinBid))
	}
	if opts.MaxAsk > 0 {
		filters = append(filters, "max-ask="+floatToStr(opts.MaxAsk))
	}
	if opts.MinVolume > 0 {
		filters = append(filters, "min-volume="+intToStr(opts.MinVolume))
	}
	if opts.MinOI > 0 {
		filters = append(filters, "min-oi="+intToStr(opts.MinOI))
	}
	if opts.MaxSpreadPct > 0 {
		filters = append(filters, "max-spread-pct="+floatToStr(opts.MaxSpreadPct))
	}

	return filters
}

// intToStr converts an int to its string representation.
func intToStr(v int) string {
	return strconv.Itoa(v)
}

// floatToStr formats a float64 with no trailing zeros.
func floatToStr(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
