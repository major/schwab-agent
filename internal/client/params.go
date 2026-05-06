package client

import "time"

// setParam adds a key-value pair to the map if value is non-empty.
func setParam(m map[string]string, key, value string) {
	if value != "" {
		m[key] = value
	}
}

// defaultDateRange fills in from and to with a 60-day lookback window
// (RFC 3339 format, UTC) when either value is empty. The Schwab API
// requires date parameters on order and transaction endpoints; this
// provides a sensible default matching the Python schwab-py client.
func defaultDateRange(from, to string) (string, string) {
	now := time.Now().UTC()
	fromDate := from
	toDate := to
	if fromDate == "" {
		fromDate = now.AddDate(0, 0, -60).Format(time.RFC3339)
	}
	if toDate == "" {
		toDate = now.Format(time.RFC3339)
	}
	return fromDate, toDate
}
