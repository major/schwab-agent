package commands

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/models"
)

const flagEnumAnnotation = "schwab-agent/flag-enum"

type cobraValidatable interface {
	Validate(context.Context) []error
}

type cobraStringEnumValue struct {
	field   reflect.Value
	valid   []string
	aliases map[string]string
}

func (v *cobraStringEnumValue) Set(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		v.field.SetString("")
		return nil
	}

	for alias, canonical := range v.aliases {
		if strings.EqualFold(trimmed, alias) {
			v.field.SetString(canonical)
			return nil
		}
	}

	for _, candidate := range v.valid {
		if strings.EqualFold(trimmed, candidate) {
			v.field.SetString(candidate)
			return nil
		}
	}

	return fmt.Errorf("invalid value %q (allowed: %s)", raw, strings.Join(v.valid, ", "))
}

func (v *cobraStringEnumValue) String() string {
	if v == nil || !v.field.IsValid() {
		return ""
	}

	return v.field.String()
}

func (v *cobraStringEnumValue) Type() string { return "string" }

// requireArg returns a ValidationError if value is empty.
// Use this for required positional arguments and flag values.
func requireArg(value, name string) error {
	if value == "" {
		return apperr.NewValidationError(name+" is required", nil)
	}

	return nil
}

// parseEnum validates raw against valid, returning the matching enum member or
// fallback when raw is empty. Trims whitespace and uppercases before comparison.
func parseEnum[T ~string](raw string, valid []T, fallback T, label string) (T, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return fallback, nil
	}

	candidate := T(strings.ToUpper(v))
	if slices.Contains(valid, candidate) {
		return candidate, nil
	}

	var zero T

	return zero, newValidationError(
		fmt.Sprintf("invalid %s: %q (valid: %s)", label, raw, validEnumString(valid)),
	)
}

// requireEnum validates raw against valid, returning a ValidationError when raw
// is empty. Use this for required enum flags that have no fallback.
func requireEnum[T ~string](raw string, valid []T, label string) (T, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		var zero T

		return zero, newValidationError(label + " is required")
	}

	candidate := T(strings.ToUpper(v))
	if slices.Contains(valid, candidate) {
		return candidate, nil
	}

	var zero T

	return zero, newValidationError(
		fmt.Sprintf("invalid %s: %q (valid: %s)", label, raw, validEnumString(valid)),
	)
}

// requireTypedEnum validates that a typed enum flag with no useful zero value
// was provided. Cobra validates explicit enum values through pflag.Value, but
// omitted flags still arrive as the enum type's zero value.
func requireTypedEnum[T ~string](value T, label string) error {
	if strings.TrimSpace(string(value)) == "" {
		return newValidationError(label + " is required")
	}

	return nil
}

// defineAndConstrain centralizes Cobra flag registration plus flag relationships
// for order commands. Each pair represents a set of flags that must be mutually
// exclusive and where one member must be provided.
func defineAndConstrain[O any](cmd *cobra.Command, opts O, exclusivePairs ...[]string) {
	defineCobraFlags(cmd, opts)
	for _, pair := range exclusivePairs {
		cmd.MarkFlagsMutuallyExclusive(pair...)
		cmd.MarkFlagsOneRequired(pair...)
	}
}

// defineCobraFlags registers flags from the existing option structs using Cobra
// bindings. Keeping this reflection local preserves concise command setup while
// still making RunE handlers read regular typed option structs after pflag
// parsing.
func defineCobraFlags(cmd *cobra.Command, opts any) {
	value := reflect.ValueOf(opts)
	if value.Kind() != reflect.Pointer || value.IsNil() || value.Elem().Kind() != reflect.Struct {
		panic("defineCobraFlags requires a pointer to an options struct")
	}

	structValue := value.Elem()
	structType := structValue.Type()
	for i := range structValue.NumField() {
		fieldType := structType.Field(i)
		name := fieldType.Tag.Get("flag")
		if name == "" {
			continue
		}

		field := structValue.Field(i)
		usage := fieldType.Tag.Get("flagdescr")
		short := fieldType.Tag.Get("flagshort")
		if defaultValue := fieldType.Tag.Get("default"); defaultValue != "" {
			setCobraFlagDefault(field, defaultValue, name)
		}
		registerCobraFlag(cmd.Flags(), name, short, usage, field)

		if fieldType.Tag.Get("flagrequired") == "true" {
			if err := cmd.MarkFlagRequired(name); err != nil {
				panic(err)
			}
		}
	}
}

func setCobraFlagDefault(field reflect.Value, raw, name string) {
	switch field.Kind() {
	case reflect.String:
		field.SetString(raw)
	case reflect.Int:
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			panic(fmt.Sprintf("invalid default for --%s: %v", name, err))
		}
		field.SetInt(int64(parsed))
	case reflect.Float64:
		parsed, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			panic(fmt.Sprintf("invalid default for --%s: %v", name, err))
		}
		field.SetFloat(parsed)
	case reflect.Bool:
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			panic(fmt.Sprintf("invalid default for --%s: %v", name, err))
		}
		field.SetBool(parsed)
	default:
		panic(fmt.Sprintf("unsupported default for --%s type %s", name, field.Type()))
	}
}

func registerCobraFlag(flags *pflag.FlagSet, name, short, usage string, field reflect.Value) {
	switch field.Kind() {
	case reflect.String:
		if field.Type() == reflect.TypeFor[string]() {
			flags.StringVarP(field.Addr().Interface().(*string), name, short, field.String(), usage)
			return
		}

		value := field.Addr().Interface()
		valid, aliases := cobraEnumValues(value)
		flags.VarP(&cobraStringEnumValue{field: field, valid: valid, aliases: aliases}, name, short, usage)
		if len(valid) > 0 {
			flags.Lookup(name).Annotations = map[string][]string{flagEnumAnnotation: valid}
		}
	case reflect.Bool:
		flags.BoolVarP(field.Addr().Interface().(*bool), name, short, field.Bool(), usage)
	case reflect.Int:
		flags.IntVarP(field.Addr().Interface().(*int), name, short, int(field.Int()), usage)
	case reflect.Float64:
		flags.Float64VarP(field.Addr().Interface().(*float64), name, short, field.Float(), usage)
	case reflect.Slice:
		if field.Type().Elem().Kind() == reflect.String {
			flags.StringArrayVarP(field.Addr().Interface().(*[]string), name, short, nil, usage)
			return
		}
		panic(fmt.Sprintf("unsupported slice flag %s type %s", name, field.Type()))
	default:
		panic(fmt.Sprintf("unsupported flag %s type %s", name, field.Type()))
	}
}

func validateCobraOptions(ctx context.Context, opts any) error {
	validatable, ok := opts.(cobraValidatable)
	if !ok {
		return nil
	}

	errs := validatable.Validate(ctx)
	if len(errs) == 0 {
		return nil
	}

	messages := make([]string, 0, len(errs))
	for _, err := range errs {
		messages = append(messages, err.Error())
	}

	return newValidationError(strings.Join(messages, "; "))
}

func cobraEnumValues(value any) (valid []string, aliases map[string]string) {
	switch value.(type) {
	case *models.Instruction:
		return enumStrings(validInstructions), nil
	case *models.OrderType:
		return enumStrings(validOrderTypes), map[string]string{"MOC": string(models.OrderTypeMarketOnClose), "LOC": string(models.OrderTypeLimitOnClose)}
	case *models.Duration:
		return enumStrings(validDurations), map[string]string{"GTC": string(models.DurationGoodTillCancel), "FOK": string(models.DurationFillOrKill), "IOC": string(models.DurationImmediateOrCancel)}
	case *models.Session:
		return enumStrings(validSessions), nil
	case *models.StopPriceLinkBasis:
		return enumStrings(validStopPriceLinkBases), nil
	case *models.StopPriceLinkType:
		return enumStrings(validStopPriceLinkTypes), nil
	case *models.StopType:
		return enumStrings(validStopTypes), nil
	case *models.SpecialInstruction:
		return enumStrings(validSpecialInstructions), nil
	case *models.RequestedDestination:
		return enumStrings(validDestinations), nil
	case *models.PriceLinkBasis:
		return enumStrings(validPriceLinkBases), nil
	case *models.PriceLinkType:
		return enumStrings(validPriceLinkTypes), nil
	case *orderStatusFilter:
		return enumStrings(validOrderStatusFilters), nil
	case *chainContractType:
		return enumStrings([]chainContractType{chainContractTypeCall, chainContractTypePut, chainContractTypeAll}), nil
	case *chainStrategy:
		return enumStrings([]chainStrategy{chainStrategySingle, chainStrategyAnalytical, chainStrategyCovered, chainStrategyVertical, chainStrategyCalendar, chainStrategyStrangle, chainStrategyStraddle, chainStrategyButterfly, chainStrategyCondor, chainStrategyDiagonal, chainStrategyCollar, chainStrategyRoll}), nil
	case *strikeRange:
		return enumStrings([]strikeRange{strikeRangeITM, strikeRangeNTM, strikeRangeOTM, strikeRangeSAK, strikeRangeSBK, strikeRangeSNK, strikeRangeAll}), nil
	case *historyPeriodType:
		return enumStrings([]historyPeriodType{historyPeriodTypeDay, historyPeriodTypeMonth, historyPeriodTypeYear, historyPeriodTypeYTD}), nil
	case *historyFrequencyType:
		return enumStrings([]historyFrequencyType{historyFrequencyTypeMinute, historyFrequencyTypeDaily, historyFrequencyTypeWeekly, historyFrequencyTypeMonthly}), nil
	case *historyFrequency:
		return enumStrings([]historyFrequency{historyFrequency1, historyFrequency5, historyFrequency10, historyFrequency15, historyFrequency30}), nil
	case *instrumentProjection:
		return enumStrings([]instrumentProjection{instrumentProjectionSymbolSearch, instrumentProjectionSymbolRegex, instrumentProjectionDescSearch, instrumentProjectionDescRegex, instrumentProjectionSearch, instrumentProjectionFundamental}), nil
	case *moversSort:
		return enumStrings([]moversSort{moversSortVolume, moversSortTrades, moversSortPercentChangeUp, moversSortPercentChangeDown}), nil
	case *moversFrequency:
		return enumStrings([]moversFrequency{moversFrequency0, moversFrequency1, moversFrequency5, moversFrequency10, moversFrequency30, moversFrequency60}), nil
	case *positionSort:
		return enumStrings([]positionSort{positionSortPnLDesc, positionSortPnLAsc, positionSortValueDesc}), nil
	case *taInterval:
		return enumStrings([]taInterval{taIntervalDaily, taIntervalWeekly, taInterval1Min, taInterval5Min, taInterval15Min, taInterval30Min}), nil
	default:
		return nil, nil
	}
}

func enumStrings[E ~string](values []E) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		result = append(result, string(value))
	}
	sort.Strings(result)

	return result
}

// validEnumString joins valid enum values into a comma-separated string for
// error messages. Example: "BUY, SELL, BUY_TO_COVER".
func validEnumString[T ~string](valid []T) string {
	s := make([]string, len(valid))
	for i, v := range valid {
		s[i] = string(v)
	}

	return strings.Join(s, ", ")
}

// newValidationError creates a consistent validation error for command parsing.
func newValidationError(message string) error {
	return apperr.NewValidationError(message, nil)
}

// requireSubcommand returns a cobra.RunE that rejects direct invocation of
// a parent command with a clear error. Without this, cobra produces a confusing
// "no subcommands available" message when the user omits the subcommand
// (e.g. "quote AAPL" instead of "quote get AAPL").
func requireSubcommand(cmd *cobra.Command, args []string) error {
	list := strings.Join(visibleSubcommandNames(cmd), ", ")

	if len(args) > 0 {
		return apperr.NewValidationError(
			fmt.Sprintf("unknown command %q for %q; available subcommands: %s", args[0], cmd.Name(), list),
			nil,
		)
	}

	return apperr.NewValidationError(
		fmt.Sprintf("%q requires a subcommand: %s", cmd.Name(), list),
		nil,
	)
}

// defaultSubcommand returns a RunE for parent commands that have a preferred
// positional-only shorthand. Cobra has already failed to resolve args[0] as a
// subcommand when this parent RunE is called, so a non-subcommand first arg can
// be handed directly to the default child command's RunE.
func defaultSubcommand(sub *cobra.Command) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 || slices.Contains(visibleSubcommandNames(cmd), args[0]) {
			return requireSubcommand(cmd, args)
		}

		// Do not call Execute here: that would re-run the root PersistentPreRunE,
		// causing duplicate auth/client setup. The shorthand intentionally forwards
		// positional args only; flags still require the explicit subcommand form.
		return sub.RunE(sub, args)
	}
}

// suggestSubcommands handles usage errors that happen before RunE runs. This
// is most useful for parent commands whose subcommands own distinct flags: an
// unknown flag on the parent usually means the user skipped the subcommand.
func suggestSubcommands(cmd *cobra.Command, err error) error {
	if !strings.Contains(err.Error(), "unknown flag") && !strings.Contains(err.Error(), "flag provided but not defined") {
		return err
	}

	names := visibleSubcommandNames(cmd)
	if len(names) == 0 {
		return err
	}

	return apperr.NewValidationError(
		fmt.Sprintf(
			"%q requires a subcommand: %s (%s)",
			cmd.CommandPath(),
			strings.Join(names, ", "),
			err.Error(),
		),
		err,
	)
}

// normalizeFlagValidationError keeps enum flag errors consistent with the older
// command-level validation messages while still letting pflag reject invalid
// enum values before RunE starts.
func normalizeFlagValidationError(err error) error {
	return normalizeFlagValidationErrorWithCommand(nil, err)
}

func normalizeFlagValidationErrorWithCommand(cmd *cobra.Command, err error) error {
	message := err.Error()
	if !strings.Contains(message, "invalid value") || !strings.Contains(message, " for \"--") {
		return err
	}

	invalidValue := ""
	if before, after, ok := strings.Cut(message, "invalid argument \""); ok {
		_ = before
		invalidValue, _, _ = strings.Cut(after, "\"")
	}

	_, after, ok := strings.Cut(message, " for \"--")
	if !ok {
		return err
	}
	flagName, _, ok := strings.Cut(after, "\" flag")
	if !ok || flagName == "" {
		return err
	}

	validValues := enumValuesForFlag(cmd, flagName)
	if len(validValues) == 0 {
		validValues = enumValuesFromMessage(message)
	}
	if invalidValue == "" || len(validValues) == 0 {
		return err
	}

	return newValidationError(fmt.Sprintf("invalid %s: %q (valid: %s)", flagName, invalidValue, validEnumString(validValues)))
}

func normalizeFlagValidationErrorFunc(cmd *cobra.Command, err error) error {
	if cmd == nil {
		return normalizeFlagValidationError(err)
	}

	return normalizeFlagValidationErrorWithCommand(cmd, err)
}

func enumValuesForFlag(cmd *cobra.Command, flagName string) []string {
	if cmd == nil {
		return nil
	}

	flag := cmd.Flags().Lookup(flagName)
	if flag == nil {
		flag = cmd.InheritedFlags().Lookup(flagName)
	}
	if flag == nil || flag.Annotations == nil {
		return nil
	}

	return cleanEnumValues(flag.Annotations[flagEnumAnnotation])
}

func enumValuesFromMessage(message string) []string {
	_, after, ok := strings.Cut(message, "(allowed: ")
	if !ok {
		return nil
	}
	values, _, ok := strings.Cut(after, ")")
	if !ok {
		return nil
	}

	return cleanEnumValues(strings.Split(values, ","))
}

func cleanEnumValues(values []string) []string {
	clean := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		// Optional enum flag registrations may include an empty value so the flag can
		// be omitted. That zero value is implementation detail, not useful
		// remediation text for someone who typed a bad explicit value.
		if value == "" {
			continue
		}
		clean = append(clean, value)
	}

	return clean
}

// visibleSubcommandNames returns only invokable subcommands for user-facing
// errors, omitting hidden commands.
func visibleSubcommandNames(cmd *cobra.Command) []string {
	names := make([]string, 0, len(cmd.Commands()))
	for _, sub := range cmd.Commands() {
		if sub.Hidden {
			continue
		}
		names = append(names, sub.Name())
	}

	return names
}
