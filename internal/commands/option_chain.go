package commands

import (
	"fmt"
	"io"
	"math"
	"slices"
	"sort"
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
	// hoursPerDay converts [time.Duration] hours to calendar days.
	hoursPerDay = 24

	// thirdFridayMinDay and thirdFridayMaxDay bound the calendar days where
	// the third Friday of a month can fall.
	thirdFridayMinDay = 15
	thirdFridayMaxDay = 21

	// keyUnderlying is the JSON key for the underlying symbol in porcelain
	// command output envelopes.
	keyUnderlying = "underlying"

	// Envelope key constants shared by option commands.
	keyColumns  = "columns"
	keyRows     = "rows"
	keyRowCount = "rowCount"
)

// chainDefaultColumns is the default column order for the compact chain view.
// Callers get this set when no custom --fields are requested.
//
//nolint:gochecknoglobals,goconst // package-level constant slice; field names are clearer inline
var chainDefaultColumns = []string{
	"expiry", "strike", "cp", "symbol",
	"bid", "ask", "mark", "delta", "iv", "oi", "volume",
}

// chainValidFields is the allowlist of field names accepted by
// resolveChainColumns. Order matches the design doc.
//
//nolint:gochecknoglobals,goconst // package-level constant slice; field names are clearer inline
var chainValidFields = []string{
	"expiry", "strike", "cp", "symbol",
	"bid", "ask", "mark", "last",
	"delta", "gamma", "theta", "vega", "rho",
	"iv", "oi", "volume", "itm", "dte",
}

// chainRow holds the sort key and extracted field values for a single contract.
// Sorting happens on expiry+strike+putCall before the row values are emitted.
type chainRow struct {
	expiry  string
	strike  float64
	putCall string
	values  []any
}

// flattenChainRows collects all contracts from an OptionChain's CallExpDateMap
// and PutExpDateMap into sorted rows using the default column set. Rows are
// sorted by expiration ASC, strike ASC, then calls before puts.
//
//nolint:unparam // error return reserved for future field-extraction failures
func flattenChainRows(chain *models.OptionChain) ([]string, [][]any, error) {
	columns := chainDefaultColumns

	var rows []chainRow

	// Collect calls and puts from their respective maps.
	rows = collectExpDateMap(rows, chain.CallExpDateMap, columns)
	rows = collectExpDateMap(rows, chain.PutExpDateMap, columns)

	// Deterministic sort: expiry ASC, strike ASC, CALL before PUT.
	slices.SortFunc(rows, func(a, b chainRow) int {
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
	})

	// Extract just the values for the caller.
	result := make([][]any, len(rows))
	for i, r := range rows {
		result[i] = r.values
	}

	return columns, result, nil
}

// collectExpDateMap iterates an expiration-date map (call or put side) and
// appends chainRows for every contract found.
func collectExpDateMap(
	rows []chainRow,
	expDateMap map[string]map[string][]*models.OptionContract,
	columns []string,
) []chainRow {
	for expKey, strikes := range expDateMap {
		// Expiration key format: "YYYY-MM-DD:DTE" - take the date part.
		expiry, _, _ := strings.Cut(expKey, ":")

		for _, contracts := range strikes {
			for _, c := range contracts {
				row := extractContractRow(c, expiry, columns)
				rows = append(rows, row)
			}
		}
	}
	return rows
}

// extractContractRow builds a chainRow from a single OptionContract using the
// requested column list. Nil pointer fields emit nil in the []any slice, never
// zero values.
func extractContractRow(c *models.OptionContract, expiry string, columns []string) chainRow {
	// Dereference sort-key fields (these are always set in valid chain data).
	strike := derefFloat64(c.StrikePrice)
	putCall := derefString(c.PutCall)

	values := make([]any, len(columns))
	for i, col := range columns {
		values[i] = contractFieldValue(c, col, expiry)
	}

	return chainRow{
		expiry:  expiry,
		strike:  strike,
		putCall: putCall,
		values:  values,
	}
}

// contractFieldValue returns the value for a single column from an
// OptionContract. Pointer fields that are nil produce a nil any value.
func contractFieldValue(c *models.OptionContract, field, expiry string) any {
	switch field {
	case "expiry":
		return expiry
	case "strike":
		return derefFloat64(c.StrikePrice)
	case "cp":
		return derefString(c.PutCall)
	case "symbol":
		return derefString(c.Symbol)
	case "bid":
		return nilableFloat64(c.Bid)
	case "ask":
		return nilableFloat64(c.Ask)
	case "mark":
		return nilableFloat64(c.Mark)
	case "last":
		return nilableFloat64(c.Last)
	case "delta":
		return nilableFloat64(c.Delta)
	case "gamma":
		return nilableFloat64(c.Gamma)
	case "theta":
		return nilableFloat64(c.Theta)
	case "vega":
		return nilableFloat64(c.Vega)
	case "rho":
		return nilableFloat64(c.Rho)
	case "iv":
		return nilableFloat64(c.Volatility)
	case "oi":
		return nilableInt64(c.OpenInterest)
	case "volume":
		return nilableInt64(c.TotalVolume)
	case "itm":
		return nilableBool(c.InTheMoney)
	case "dte":
		// Compute DTE from expiry string and current time.
		return computeDTE(expiry, time.Now())
	default:
		return nil
	}
}

// resolveChainColumns returns the column list for a compact chain view. When
// requestedFields is nil or empty, the default columns are returned. Otherwise,
// each requested field is validated against the allowlist.
func resolveChainColumns(requestedFields []string) ([]string, error) {
	if len(requestedFields) == 0 {
		return chainDefaultColumns, nil
	}

	validSet := make(map[string]bool, len(chainValidFields))
	for _, f := range chainValidFields {
		validSet[f] = true
	}

	for _, f := range requestedFields {
		if !validSet[f] {
			return nil, apperr.NewValidationError(
				fmt.Sprintf("unknown field: %s; valid fields: %s", f, strings.Join(chainValidFields, ", ")),
				nil,
			)
		}
	}

	return requestedFields, nil
}

// selectNearestExpiration picks the expiration date closest to targetDTE
// calendar days from now. Ties go to the earlier date. DTE is computed using
// America/New_York calendar dates, not UTC.
func selectNearestExpiration(expirations []string, targetDTE int, now time.Time) (string, error) {
	if targetDTE <= 0 {
		return "", apperr.NewValidationError("dte must be > 0", nil)
	}
	if len(expirations) == 0 {
		return "", apperr.NewSymbolNotFoundError("no expirations available", nil)
	}

	nyLoc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return "", fmt.Errorf("loading America/New_York timezone: %w", err)
	}

	// Convert now to NY calendar date for consistent DTE calculation.
	nyNow := now.In(nyLoc)
	todayNY := time.Date(nyNow.Year(), nyNow.Month(), nyNow.Day(), 0, 0, 0, 0, nyLoc)

	bestExp := ""
	bestDist := math.MaxInt
	var bestDate time.Time

	for _, exp := range expirations {
		expDate, parseErr := time.Parse("2006-01-02", exp)
		if parseErr != nil {
			return "", apperr.NewValidationError(
				fmt.Sprintf("invalid expiration date format: %s", exp), parseErr,
			)
		}

		// Place the expiration in the NY timezone for calendar-day arithmetic.
		expNY := time.Date(expDate.Year(), expDate.Month(), expDate.Day(), 0, 0, 0, 0, nyLoc)
		dte := int(expNY.Sub(todayNY).Hours() / hoursPerDay)

		dist := abs(dte - targetDTE)

		// Pick minimum distance. On tie, pick the earlier date.
		if dist < bestDist || (dist == bestDist && expNY.Before(bestDate)) {
			bestDist = dist
			bestExp = exp
			bestDate = expNY
		}
	}

	return bestExp, nil
}

// expirationColumns is the column order for the compact expirations view.
//
//nolint:gochecknoglobals,goconst // package-level constant slice; field names are clearer inline
var expirationColumns = []string{
	"expiration", "dte", "expirationType", "standard",
}

// flattenExpirationRows converts an ExpirationChain into sorted compact rows.
// Returns the column names and row data. Nil or empty chains produce empty
// rows without error.
func flattenExpirationRows(chain *client.ExpirationChain, now time.Time) ([]string, [][]any, error) {
	if chain == nil || len(chain.ExpirationList) == 0 {
		return expirationColumns, [][]any{}, nil
	}

	nyLoc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return nil, nil, fmt.Errorf("loading America/New_York timezone: %w", err)
	}

	// Convert now to NY calendar date.
	nyNow := now.In(nyLoc)
	todayNY := time.Date(nyNow.Year(), nyNow.Month(), nyNow.Day(), 0, 0, 0, 0, nyLoc)

	// Copy and sort by date ascending.
	sorted := make([]client.ExpirationDate, len(chain.ExpirationList))
	copy(sorted, chain.ExpirationList)
	slices.SortFunc(sorted, func(a, b client.ExpirationDate) int {
		return strings.Compare(a.ExpirationDate, b.ExpirationDate)
	})

	rows := make([][]any, 0, len(sorted))
	for _, exp := range sorted {
		expDate, parseErr := time.Parse("2006-01-02", exp.ExpirationDate)
		if parseErr != nil {
			return nil, nil, apperr.NewValidationError(
				fmt.Sprintf("invalid expiration date format: %s", exp.ExpirationDate), parseErr,
			)
		}

		// DTE via NY calendar dates.
		expNY := time.Date(expDate.Year(), expDate.Month(), expDate.Day(), 0, 0, 0, 0, nyLoc)
		dte := int(expNY.Sub(todayNY).Hours() / hoursPerDay)

		// Third Friday detection: Friday where day is in [15, 21].
		isThirdFriday := expDate.Weekday() == time.Friday &&
			expDate.Day() >= thirdFridayMinDay && expDate.Day() <= thirdFridayMaxDay

		expType := "S" // weekly/special
		if isThirdFriday {
			expType = "R" // regular monthly
		}

		rows = append(rows, []any{
			exp.ExpirationDate,
			dte,
			expType,
			isThirdFriday,
		})
	}

	return expirationColumns, rows, nil
}

// computeDTE returns the calendar-day difference between the expiry date string
// and now, using America/New_York calendar dates. Returns nil on parse failure.
func computeDTE(expiry string, now time.Time) any {
	expDate, err := time.Parse("2006-01-02", expiry)
	if err != nil {
		return nil
	}

	nyLoc, locErr := time.LoadLocation("America/New_York")
	if locErr != nil {
		return nil
	}

	nyNow := now.In(nyLoc)
	todayNY := time.Date(nyNow.Year(), nyNow.Month(), nyNow.Day(), 0, 0, 0, 0, nyLoc)
	expNY := time.Date(expDate.Year(), expDate.Month(), expDate.Day(), 0, 0, 0, 0, nyLoc)

	return int(expNY.Sub(todayNY).Hours() / hoursPerDay)
}

// --- Pointer dereference helpers ---

// derefFloat64 returns the value behind a *float64, or 0.0 if nil.
func derefFloat64(p *float64) float64 {
	if p == nil {
		return 0.0
	}
	return *p
}

// derefString returns the value behind a *string, or "" if nil.
func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// nilableFloat64 returns *p as any if non-nil, or nil if the pointer is nil.
// This preserves the nil/present distinction in []any row output.
func nilableFloat64(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

// nilableInt64 returns *p as any if non-nil, or nil if the pointer is nil.
func nilableInt64(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}

// nilableBool returns *p as any if non-nil, or nil if the pointer is nil.
func nilableBool(p *bool) any {
	if p == nil {
		return nil
	}
	return *p
}

// abs returns the absolute value of x. Inlined to avoid a math import for ints.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// --- Porcelain option chain command ---

// newOptionChainCmd returns the porcelain command for compact option chain
// discovery. It fetches raw chain data, transforms it to compact rows, applies
// field projection and strike selectors, and emits the result.
func newOptionChainCmd(ref *client.Ref, w io.Writer) *cobra.Command {
	opts := &optionChainOpts{}
	cmd := &cobra.Command{
		Use:   "chain <symbol>",
		Short: "Get compact option chain for a symbol",
		Long: `Get a compact option chain with configurable columns, expiration selection,
and strike filtering. Returns rows sorted by expiration, strike, then call
before put. Use --dte or --expiration to narrow to one expiration cycle.
Use --fields to select specific columns.`,
		Example: `  schwab-agent option chain AAPL
  schwab-agent option chain AAPL --type CALL --dte 30
  schwab-agent option chain AAPL --expiration 2026-06-19 --fields strike,bid,ask,delta
  schwab-agent option chain AAPL --strike-count 5
  schwab-agent option chain AAPL --strike-min 190 --strike-max 210`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOptionChain(cmd, ref, w, opts, args)
		},
	}

	defineCobraFlags(cmd, opts)
	cmd.MarkFlagsMutuallyExclusive("dte", "expiration")
	cmd.SetFlagErrorFunc(normalizeFlagValidationErrorFunc)

	return cmd
}

// runOptionChain is the RunE body for the porcelain option chain command.
// Extracted for testability and readability.
func runOptionChain(
	cmd *cobra.Command,
	ref *client.Ref,
	w io.Writer,
	opts *optionChainOpts,
	args []string,
) error {
	if err := validateCobraOptions(cmd.Context(), opts); err != nil {
		return err
	}

	symbol := strings.ToUpper(strings.TrimSpace(args[0]))
	if err := requireArg(symbol, "symbol"); err != nil {
		return err
	}

	// Build API params. The porcelain command always uses SINGLE strategy
	// and includes the underlying quote for ATM centering.
	params := porcelainChainParams(symbol, opts)

	chain, err := ref.OptionChain(cmd.Context(), params)
	if err != nil {
		return err
	}

	if chain == nil || (len(chain.CallExpDateMap) == 0 && len(chain.PutExpDateMap) == 0) {
		return apperr.NewSymbolNotFoundError("no option chain data for "+symbol, nil)
	}

	// Resolve the column set: custom fields or defaults.
	columns, err := resolveRequestedColumns(opts.Fields)
	if err != nil {
		return err
	}

	// Build rows with the resolved columns. This uses the same underlying
	// helpers as flattenChainRows but allows custom column projection.
	rows := buildSortedChainRows(chain, columns)

	// When --dte is set, select the nearest expiration and filter rows.
	selectedExpiration := ""
	if opts.DTE > 0 {
		expirations := extractExpirationsFromChain(chain)
		selectedExpiration, err = selectNearestExpiration(expirations, opts.DTE, time.Now())
		if err != nil {
			return err
		}
		rows = filterRowsByExpiration(rows, selectedExpiration)
	}

	// When --expiration is set, the API already filtered server-side via
	// FromDate/ToDate, but record it for the output envelope.
	if selectedExpiration == "" && opts.Expiration != "" {
		selectedExpiration = opts.Expiration
	}

	// Apply local strike selectors after flattening.
	rows = applyStrikeSelectors(rows, opts, chain)

	// Determine expiration label for output. Falls back to the first row's
	// expiry when no explicit selector was provided.
	expiration := selectedExpiration
	if expiration == "" && len(rows) > 0 {
		expiration = rows[0].expiry
	}

	// Extract row values for JSON output.
	result := make([][]any, len(rows))
	for i, r := range rows {
		result[i] = r.values
	}

	meta := output.NewMetadata()
	if opts.StrikeCount > 0 {
		meta.Requested = opts.StrikeCount
	}
	meta.Returned = len(result)

	return output.WriteSuccess(w, map[string]any{
		keyUnderlying: symbol,
		"expiration":  expiration,
		keyColumns:    columns,
		keyRows:       result,
		keyRowCount:   len(result),
	}, meta)
}

// porcelainChainParams builds API parameters for the porcelain chain command.
// Always uses SINGLE strategy and includes the underlying quote for ATM
// strike centering.
func porcelainChainParams(symbol string, opts *optionChainOpts) *marketdata.OptionChainParams {
	params := &marketdata.OptionChainParams{
		Symbol:                 symbol,
		ContractType:           marketdata.OptionChainContractType(opts.Type),
		Strategy:               marketdata.OptionChainStrategySingle,
		IncludeUnderlyingQuote: true,
	}

	if opts.Expiration != "" {
		params.FromDate = opts.Expiration
		params.ToDate = opts.Expiration
	}

	if opts.Strike > 0 {
		params.Strike = opts.Strike
	}

	if string(opts.StrikeRange) != "" {
		params.Range = marketdata.OptionChainRange(opts.StrikeRange)
	}

	return params
}

// resolveRequestedColumns parses a comma-separated --fields value into the
// column list, falling back to defaults when empty.
func resolveRequestedColumns(fieldsFlag string) ([]string, error) {
	if fieldsFlag == "" {
		return chainDefaultColumns, nil
	}

	parts := strings.Split(fieldsFlag, ",")
	fields := make([]string, 0, len(parts))
	for _, f := range parts {
		trimmed := strings.TrimSpace(f)
		if trimmed != "" {
			fields = append(fields, trimmed)
		}
	}

	return resolveChainColumns(fields)
}

// buildSortedChainRows collects all contracts from a chain into sorted rows
// using the specified column set. Rows are sorted by expiration ASC, strike
// ASC, then calls before puts.
func buildSortedChainRows(chain *models.OptionChain, columns []string) []chainRow {
	var rows []chainRow
	rows = collectExpDateMap(rows, chain.CallExpDateMap, columns)
	rows = collectExpDateMap(rows, chain.PutExpDateMap, columns)
	slices.SortFunc(rows, compareChainRows)
	return rows
}

// compareChainRows provides deterministic ordering: expiry ASC, strike ASC,
// CALL before PUT.
func compareChainRows(a, b chainRow) int {
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

// extractExpirationsFromChain returns unique, sorted expiration date strings
// from a chain's CallExpDateMap and PutExpDateMap keys. Key format is
// "YYYY-MM-DD:DTE".
func extractExpirationsFromChain(chain *models.OptionChain) []string {
	seen := make(map[string]bool)

	for key := range chain.CallExpDateMap {
		date, _, _ := strings.Cut(key, ":")
		seen[date] = true
	}
	for key := range chain.PutExpDateMap {
		date, _, _ := strings.Cut(key, ":")
		seen[date] = true
	}

	expirations := make([]string, 0, len(seen))
	for date := range seen {
		expirations = append(expirations, date)
	}
	sort.Strings(expirations)

	return expirations
}

// filterRowsByExpiration keeps only rows matching the given expiration date.
func filterRowsByExpiration(rows []chainRow, expiration string) []chainRow {
	filtered := make([]chainRow, 0, len(rows))
	for _, r := range rows {
		if r.expiry == expiration {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// applyStrikeSelectors applies local strike filters after flattening.
// Filters are mutually exclusive (enforced by validation).
func applyStrikeSelectors(rows []chainRow, opts *optionChainOpts, chain *models.OptionChain) []chainRow {
	switch {
	case opts.Strike > 0:
		return filterRowsByStrike(rows, opts.Strike)
	case opts.StrikeCount > 0:
		return filterRowsByStrikeCount(rows, opts.StrikeCount, chain)
	case opts.StrikeMin > 0 || opts.StrikeMax > 0:
		return filterRowsByStrikeRange(rows, opts.StrikeMin, opts.StrikeMax)
	default:
		// --strike-range is passed to the API; no local filtering needed.
		return rows
	}
}

// filterRowsByStrike keeps rows whose strike matches the target exactly
// (within floating-point epsilon).
func filterRowsByStrike(rows []chainRow, strike float64) []chainRow {
	filtered := make([]chainRow, 0)
	for _, r := range rows {
		if nearlyEqual(r.strike, strike) {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// filterRowsByStrikeCount keeps rows whose strike is among the N unique
// strikes closest to ATM (the chain's underlying price). When fewer than N
// unique strikes exist, all rows are returned.
func filterRowsByStrikeCount(rows []chainRow, count int, chain *models.OptionChain) []chainRow {
	// Collect unique strikes.
	strikeSet := make(map[float64]bool)
	for _, r := range rows {
		strikeSet[r.strike] = true
	}

	if len(strikeSet) <= count {
		// Fewer unique strikes than requested: return everything.
		return rows
	}

	// Determine ATM reference price from the chain's underlying price.
	atmPrice := 0.0
	if chain != nil && chain.UnderlyingPrice != nil {
		atmPrice = *chain.UnderlyingPrice
	}

	// Sort unique strikes by distance to ATM, then by value for stability.
	strikes := make([]float64, 0, len(strikeSet))
	for s := range strikeSet {
		strikes = append(strikes, s)
	}
	slices.SortFunc(strikes, func(a, b float64) int {
		distA := math.Abs(a - atmPrice)
		distB := math.Abs(b - atmPrice)
		if distA != distB {
			if distA < distB {
				return -1
			}
			return 1
		}
		// Tie-break: lower strike first.
		if a < b {
			return -1
		}
		if a > b {
			return 1
		}
		return 0
	})

	// Keep the N closest strikes.
	keepStrikes := make(map[float64]bool, count)
	for i := range count {
		keepStrikes[strikes[i]] = true
	}

	filtered := make([]chainRow, 0, len(rows))
	for _, r := range rows {
		if keepStrikes[r.strike] {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// filterRowsByStrikeRange keeps rows whose strike falls within
// [minStrike, maxStrike]. A zero value means unbounded on that side.
func filterRowsByStrikeRange(rows []chainRow, minStrike, maxStrike float64) []chainRow {
	filtered := make([]chainRow, 0, len(rows))
	for _, r := range rows {
		if minStrike > 0 && r.strike < minStrike {
			continue
		}
		if maxStrike > 0 && r.strike > maxStrike {
			continue
		}
		filtered = append(filtered, r)
	}
	return filtered
}
