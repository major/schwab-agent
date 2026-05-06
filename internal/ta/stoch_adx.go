package ta

import (
	"fmt"

	"github.com/markcheno/go-talib"

	"github.com/major/schwab-agent/internal/apperr"
)

// Stochastic computes the Stochastic Oscillator (%K and %D lines).
//
// CRITICAL parameter mapping (CLI vs go-talib naming differs):
//
//	kPeriod  (CLI: --k-period)  -> talib fastKPeriod (lookback, default 14)
//	smoothK  (CLI: --smooth-k)  -> talib slowKPeriod  (smoothing of %K, default 3)
//	dPeriod  (CLI: --d-period)  -> talib slowDPeriod  (period of %D, default 3)
//	slowKMAType and slowDMAType are hardcoded to 0 (SMA) and not exposed.
//
// Returns (slowK, slowD []float64, err error). Both slices are zero-stripped and aligned.
func Stochastic(highs, lows, closes []float64, kPeriod, smoothK, dPeriod int) (slowK, slowD []float64, err error) {
	if kPeriod <= 0 || smoothK <= 0 || dPeriod <= 0 {
		return nil, nil, apperr.NewValidationError(
			"stochastic: all periods must be > 0",
			nil,
		)
	}
	if len(highs) != len(lows) || len(highs) != len(closes) {
		return nil, nil, apperr.NewValidationError(
			fmt.Sprintf(
				"stochastic: mismatched slice lengths: highs=%d, lows=%d, closes=%d",
				len(highs),
				len(lows),
				len(closes),
			),
			nil,
		)
	}

	// Parameter mapping: kPeriod=fastKPeriod, smoothK=slowKPeriod, dPeriod=slowDPeriod
	// slowKMAType=0 (SMA), slowDMAType=0 (SMA) - hardcoded per spec
	rawSlowK, rawSlowD := talib.Stoch(highs, lows, closes, kPeriod, smoothK, 0, dPeriod, 0)

	slowK = StripLeadingZeros(rawSlowK)
	slowD = StripLeadingZeros(rawSlowD)

	// Align to shortest length
	minLen := min(len(slowD), len(slowK))
	if len(slowK) > minLen {
		slowK = slowK[len(slowK)-minLen:]
	}
	if len(slowD) > minLen {
		slowD = slowD[len(slowD)-minLen:]
	}

	return slowK, slowD, nil
}

// ADX computes Average Directional Index along with DI+ and DI- companion lines.
// Requires THREE separate go-talib calls (unlike pandas_ta which returns all from one call).
//
// Returns (adx, plusDI, minusDI []float64, err error). All slices stripped and aligned.
func ADX(highs, lows, closes []float64, period int) (adx, plusDI, minusDI []float64, err error) {
	if period <= 0 {
		return nil, nil, nil, apperr.NewValidationError(
			fmt.Sprintf("adx period must be > 0, got %d", period),
			nil,
		)
	}
	if len(highs) != len(lows) || len(highs) != len(closes) {
		return nil, nil, nil, apperr.NewValidationError(
			fmt.Sprintf(
				"adx: mismatched slice lengths: highs=%d, lows=%d, closes=%d",
				len(highs),
				len(lows),
				len(closes),
			),
			nil,
		)
	}

	rawADX := talib.Adx(highs, lows, closes, period)
	rawPlusDI := talib.PlusDI(highs, lows, closes, period)
	rawMinusDI := talib.MinusDI(highs, lows, closes, period)

	adx = StripLeadingZeros(rawADX)
	plusDI = StripLeadingZeros(rawPlusDI)
	minusDI = StripLeadingZeros(rawMinusDI)

	// Align to shortest length
	minLen := min(len(minusDI), min(len(plusDI), len(adx)))
	if len(adx) > minLen {
		adx = adx[len(adx)-minLen:]
	}
	if len(plusDI) > minLen {
		plusDI = plusDI[len(plusDI)-minLen:]
	}
	if len(minusDI) > minLen {
		minusDI = minusDI[len(minusDI)-minLen:]
	}

	return adx, plusDI, minusDI, nil
}
