package commands

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/models"
	"github.com/major/schwab-agent/internal/orderbuilder"
)

const (
	sellCoveredCallName          = "sell-covered-call"
	sellCoveredCallShort         = "Sell a covered call against existing shares"
	coveredCallSharesPerContract = 100.0
	previewSafetyCoveredCall     = "sell-covered-call"
	sellCoveredCallLong          = `Sell a covered call against shares already held in the selected account.
This porcelain hardcodes a CALL SELL_TO_OPEN option leg so agents only provide
the contract details, quantity, and price. Preview and place fetch account
positions first and reject the command unless the account has at least 100
long shares per contract for the underlying. Build remains offline and cannot
verify holdings.`
	sellCoveredCallExample = `  schwab-agent order build sell-covered-call --underlying F --expiration 2026-06-18 --strike 14 --quantity 1 --type LIMIT --price 1.00
  schwab-agent order preview sell-covered-call --underlying F --expiration 2026-06-18 --strike 14 --quantity 1 --type LIMIT --price 1.00 --save-preview
  schwab-agent order place sell-covered-call --underlying F --expiration 2026-06-18 --strike 14 --quantity 1 --type LIMIT --price 1.00`
)

func sellCoveredCallConfig() singleLegConfig {
	return singleLegConfig{
		name:    sellCoveredCallName,
		short:   sellCoveredCallShort,
		long:    sellCoveredCallLong,
		example: sellCoveredCallExample,
		putCall: models.PutCallCall,
		action:  models.InstructionSellToOpen,
	}
}

func sellCoveredCallBuildCommand(w io.Writer) *cobra.Command {
	return makeSingleLegBuildCmd(w, sellCoveredCallConfig())
}

func sellCoveredCallPlaceCommand(c *client.Ref, configPath string, w io.Writer) *cobra.Command {
	opts := &singleLegOpts{}
	cmd := &cobra.Command{
		Use:     sellCoveredCallName,
		Short:   sellCoveredCallShort,
		Long:    sellCoveredCallLong,
		Example: sellCoveredCallExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSellCoveredCallPlace(cmd, c, configPath, w, opts, args)
		},
	}
	cmd.SetFlagErrorFunc(normalizeFlagValidationErrorFunc)
	defineCobraFlags(cmd, opts)

	return cmd
}

func sellCoveredCallPreviewCommand(c *client.Ref, configPath string, w io.Writer) *cobra.Command {
	opts := &singleLegOpts{}
	cmd := &cobra.Command{
		Use:     sellCoveredCallName,
		Short:   sellCoveredCallShort,
		Long:    sellCoveredCallLong,
		Example: sellCoveredCallExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSellCoveredCallPreview(cmd, c, configPath, w, opts, args)
		},
	}
	cmd.SetFlagErrorFunc(normalizeFlagValidationErrorFunc)
	defineCobraFlags(cmd, opts)
	cmd.Flags().Bool(
		"save-preview",
		false,
		"Save this preview locally and return a digest for order place --from-preview",
	)

	return cmd
}

func runSellCoveredCallPlace(
	cmd *cobra.Command,
	c *client.Ref,
	configPath string,
	w io.Writer,
	opts *singleLegOpts,
	args []string,
) error {
	configFlag, err := effectiveConfigFlag(cmd, configPath)
	if err != nil {
		return err
	}
	if mutableErr := requireMutableEnabled(configFlag); mutableErr != nil {
		return mutableErr
	}

	acct, params, order, err := buildSellCoveredCallForAccount(cmd, c, configFlag, opts, args)
	if err != nil {
		return err
	}
	verifyErr := verifyCoveredCallShares(cmd.Context(), c, acct.Hash, params.Underlying, params.Quantity)
	if verifyErr != nil {
		return verifyErr
	}

	response, err := c.PlaceOrder(cmd.Context(), acct.Hash, order)
	if err != nil {
		return err
	}

	data, errs := fetchOrderActionData(cmd, c, acct.Hash, commandUsePlace, response.OrderID, order)
	return writeOrderActionResult(w, &data, errs, acct)
}

func runSellCoveredCallPreview(
	cmd *cobra.Command,
	c *client.Ref,
	configPath string,
	w io.Writer,
	opts *singleLegOpts,
	args []string,
) error {
	configFlag, err := effectiveConfigFlag(cmd, configPath)
	if err != nil {
		return err
	}

	acct, params, order, err := buildSellCoveredCallForAccount(cmd, c, configFlag, opts, args)
	if err != nil {
		return err
	}
	verifyErr := verifyCoveredCallShares(cmd.Context(), c, acct.Hash, params.Underlying, params.Quantity)
	if verifyErr != nil {
		return verifyErr
	}

	preview, err := c.PreviewOrder(cmd.Context(), acct.Hash, order)
	if err != nil {
		return err
	}

	savePreview, err := cmd.Flags().GetBool("save-preview")
	if err != nil {
		return err
	}
	return writeOrderPreviewResult(
		w,
		acct,
		order,
		preview,
		savePreview,
		sellCoveredCallPreviewSafetyCheck(params),
	)
}

func sellCoveredCallPreviewSafetyCheck(params *orderbuilder.OptionParams) previewSafetyCheck {
	return previewSafetyCheck{
		Type:       previewSafetyCoveredCall,
		Underlying: params.Underlying,
		Contracts:  params.Quantity,
	}
}

func verifySavedPreviewSafetyCheck(cmd *cobra.Command, c *client.Ref, entry *savedOrderPreview) error {
	if entry.SafetyCheck == nil {
		return nil
	}
	if entry.SafetyCheck.Type != previewSafetyCoveredCall {
		return newValidationError("preview ledger entry uses an unsupported safety check")
	}

	// The preview digest guarantees the reviewed payload is unchanged, but it
	// cannot reserve shares. Re-run the covered-call preflight immediately before
	// final placement so a saved preview cannot become uncovered if shares were
	// sold after preview.
	return verifyCoveredCallShares(
		cmd.Context(),
		c,
		entry.Account,
		entry.SafetyCheck.Underlying,
		entry.SafetyCheck.Contracts,
	)
}

func buildSellCoveredCallForAccount(
	cmd *cobra.Command,
	c *client.Ref,
	configPath string,
	opts *singleLegOpts,
	args []string,
) (resolvedAccountInfo, *orderbuilder.OptionParams, *models.OrderRequest, error) {
	acct, err := commandAccountInfo(cmd, c, configPath)
	if err != nil {
		return resolvedAccountInfo{}, nil, nil, err
	}

	params, order, err := buildSellCoveredCallOrder(cmd, opts, args)
	if err != nil {
		return resolvedAccountInfo{}, nil, nil, err
	}

	return acct, params, order, nil
}

func buildSellCoveredCallOrder(
	cmd *cobra.Command,
	opts *singleLegOpts,
	args []string,
) (*orderbuilder.OptionParams, *models.OrderRequest, error) {
	if err := validateCobraOptions(cmd.Context(), opts); err != nil {
		return nil, nil, err
	}

	config := sellCoveredCallConfig()
	params, err := parseSingleLegFactory(config.putCall, config.action)(opts, args)
	if err != nil {
		return nil, nil, err
	}
	if validateErr := orderbuilder.ValidateOptionOrder(params); validateErr != nil {
		return nil, nil, validateErr
	}

	order, err := orderbuilder.BuildOptionOrder(params)
	if err != nil {
		return nil, nil, err
	}

	return params, order, nil
}

func verifyCoveredCallShares(
	ctx context.Context,
	c *client.Ref,
	accountHash string,
	underlying string,
	contracts float64,
) error {
	requiredShares := contracts * coveredCallSharesPerContract
	account, err := c.Account(ctx, accountHash, "positions")
	if err != nil {
		return fmt.Errorf("verify covered call shares: %w", err)
	}

	for _, position := range account.SecuritiesAccount.Positions {
		if !positionMatchesEquitySymbol(position, underlying) || position.LongQuantity == nil {
			continue
		}

		if *position.LongQuantity >= requiredShares {
			return nil
		}

		return newValidationError(
			fmt.Sprintf(
				"sell-covered-call requires at least %.0f long shares of %s for %.0f contract(s); account has %.0f",
				requiredShares,
				strings.ToUpper(strings.TrimSpace(underlying)),
				contracts,
				*position.LongQuantity,
			),
		)
	}

	return newValidationError(
		fmt.Sprintf(
			"sell-covered-call requires at least %.0f long shares of %s for %.0f contract(s); no matching equity position was found",
			requiredShares,
			strings.ToUpper(strings.TrimSpace(underlying)),
			contracts,
		),
	)
}

func positionMatchesEquitySymbol(position models.Position, symbol string) bool {
	if position.Instrument == nil || position.Instrument.Symbol == nil {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(*position.Instrument.Symbol), strings.TrimSpace(symbol)) {
		return false
	}

	if position.Instrument.AssetType == nil {
		return false
	}

	return strings.EqualFold(*position.Instrument.AssetType, string(models.AssetTypeEquity))
}
