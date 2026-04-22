package ta

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	schwabErrors "github.com/major/schwab-agent/internal/errors"
)

func TestIntervalToHistoryParams(t *testing.T) {
	tests := []struct {
		name      string
		interval  string
		wantPT    string
		wantP     string
		wantFT    string
		wantF     string
		wantErr   bool
		errMsg    string
	}{
		{
			name:     "daily",
			interval: "daily",
			wantPT:   "year",
			wantP:    "1",
			wantFT:   "daily",
			wantF:    "1",
			wantErr:  false,
		},
		{
			name:     "weekly",
			interval: "weekly",
			wantPT:   "year",
			wantP:    "1",
			wantFT:   "weekly",
			wantF:    "1",
			wantErr:  false,
		},
		{
			name:     "1min",
			interval: "1min",
			wantPT:   "day",
			wantP:    "10",
			wantFT:   "minute",
			wantF:    "1",
			wantErr:  false,
		},
		{
			name:     "5min",
			interval: "5min",
			wantPT:   "day",
			wantP:    "10",
			wantFT:   "minute",
			wantF:    "5",
			wantErr:  false,
		},
		{
			name:     "15min",
			interval: "15min",
			wantPT:   "day",
			wantP:    "10",
			wantFT:   "minute",
			wantF:    "15",
			wantErr:  false,
		},
		{
			name:     "30min",
			interval: "30min",
			wantPT:   "day",
			wantP:    "10",
			wantFT:   "minute",
			wantF:    "30",
			wantErr:  false,
		},
		{
			name:     "invalid",
			interval: "invalid",
			wantErr:  true,
			errMsg:   "unsupported interval",
		},
		{
			name:     "empty",
			interval: "",
			wantErr:  true,
			errMsg:   "unsupported interval",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			pt, p, ft, f, err := IntervalToHistoryParams(tt.interval)

			// Assert
			if tt.wantErr {
				require.Error(t, err)
				var valErr *schwabErrors.ValidationError
				require.True(t, assert.ErrorAs(t, err, &valErr), "error should be ValidationError")
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantPT, pt)
				assert.Equal(t, tt.wantP, p)
				assert.Equal(t, tt.wantFT, ft)
				assert.Equal(t, tt.wantF, f)
			}
		})
	}
}
