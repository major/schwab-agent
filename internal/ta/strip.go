package ta

// StripLeadingZeros removes leading zero-valued elements from go-talib output.
// Handles: all-zeros -> []float64{} (empty), no-zeros -> as-is, mixed -> strip prefix only.
func StripLeadingZeros(values []float64) []float64 {
	if len(values) == 0 {
		return []float64{}
	}

	// Find the first non-zero index.
	firstNonZeroIdx := 0
	for i, v := range values {
		if v != 0 {
			firstNonZeroIdx = i
			break
		}
		// If we reach the end without finding a non-zero, all are zeros.
		if i == len(values)-1 {
			return []float64{}
		}
	}

	return values[firstNonZeroIdx:]
}
