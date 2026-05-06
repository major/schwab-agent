package ta

import (
	"fmt"
	"math"

	"github.com/major/schwab-agent/internal/apperr"
)

const (
	volatilityRegimeElevated = "elevated"
	volatilityRegimeExtreme  = "extreme"
	volatilityRegimeHigh     = "high"
	volatilityRegimeLow      = "low"
	volatilityRegimeNormal   = "normal"
	volatilityRegimeVeryLow  = "very_low"

	tradingDaysPerYear = 252
	regimeVeryLow      = 10.0
	regimeLow          = 15.0
	regimeNormal       = 20.0
	regimeElevated     = 30.0
	regimeHigh         = 50.0
)

// HistoricalVolatilityResult summarizes close-to-close historical volatility.
type HistoricalVolatilityResult struct {
	DailyVol       float64 `json:"daily_vol"`
	WeeklyVol      float64 `json:"weekly_vol"`
	MonthlyVol     float64 `json:"monthly_vol"`
	AnnualizedVol  float64 `json:"annualized_vol"`
	PercentileRank float64 `json:"percentile_rank"`
	Regime         string  `json:"regime"`
	MinVol         float64 `json:"min_vol"`
	MaxVol         float64 `json:"max_vol"`
	MeanVol        float64 `json:"mean_vol"`
}

// HistoricalVolatility computes close-to-close historical volatility over a rolling window.
// It uses simple returns, sample standard deviation, and reports volatility in percentage terms.
func HistoricalVolatility(closes []float64, period int) (*HistoricalVolatilityResult, error) {
	if period <= 0 {
		return nil, apperr.NewValidationError(
			fmt.Sprintf("historical volatility period must be > 0, got %d", period),
			nil,
		)
	}

	if len(closes) < period+1 {
		return nil, apperr.NewValidationError(
			fmt.Sprintf(
				"historical volatility requires at least %d closing prices, got %d",
				period+1,
				len(closes),
			),
			nil,
		)
	}

	returns := make([]float64, len(closes)-1)
	for i := 1; i < len(closes); i++ {
		returns[i-1] = (closes[i] - closes[i-1]) / closes[i-1]
	}

	volSeries := make([]float64, len(returns)-period+1)
	for j := period - 1; j < len(returns); j++ {
		start := j - period + 1
		volSeries[start] = stddev(returns[start : j+1])
	}

	dailyVolDecimal := volSeries[len(volSeries)-1]
	annualizationFactor := math.Sqrt(tradingDaysPerYear)
	annualizedSeries := make([]float64, len(volSeries))

	minVol := volSeries[0] * annualizationFactor * 100
	maxVol := minVol
	meanVol := 0.0

	for i, vol := range volSeries {
		annualized := vol * annualizationFactor * 100
		annualizedSeries[i] = annualized
		meanVol += annualized

		if annualized < minVol {
			minVol = annualized
		}
		if annualized > maxVol {
			maxVol = annualized
		}
	}

	meanVol /= float64(len(annualizedSeries))
	annualizedVol := dailyVolDecimal * annualizationFactor * 100

	return &HistoricalVolatilityResult{
		DailyVol:       dailyVolDecimal * 100,
		WeeklyVol:      dailyVolDecimal * math.Sqrt(5) * 100,
		MonthlyVol:     dailyVolDecimal * math.Sqrt(21) * 100,
		AnnualizedVol:  annualizedVol,
		PercentileRank: percentileRank(volSeries, dailyVolDecimal),
		Regime:         classifyRegime(annualizedVol),
		MinVol:         minVol,
		MaxVol:         maxVol,
		MeanVol:        meanVol,
	}, nil
}

func classifyRegime(annualizedVol float64) string {
	switch {
	case annualizedVol < regimeVeryLow:
		return volatilityRegimeVeryLow
	case annualizedVol < regimeLow:
		return volatilityRegimeLow
	case annualizedVol < regimeNormal:
		return volatilityRegimeNormal
	case annualizedVol < regimeElevated:
		return volatilityRegimeElevated
	case annualizedVol < regimeHigh:
		return volatilityRegimeHigh
	default:
		return volatilityRegimeExtreme
	}
}

func stddev(data []float64) float64 {
	n := len(data)
	if n <= 1 {
		return 0
	}

	mean := 0.0
	for _, value := range data {
		mean += value
	}
	mean /= float64(n)

	sumSq := 0.0
	for _, value := range data {
		diff := value - mean
		sumSq += diff * diff
	}

	return math.Sqrt(sumSq / float64(n-1))
}

func percentileRank(series []float64, current float64) float64 {
	if len(series) == 0 {
		return 0
	}

	count := 0
	for _, value := range series {
		if value <= current {
			count++
		}
	}

	return float64(count) / float64(len(series)) * 100
}
