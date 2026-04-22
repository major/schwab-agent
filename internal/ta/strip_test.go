package ta

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStripLeadingZeros(t *testing.T) {
	tests := []struct {
		name  string
		input []float64
		want  []float64
	}{
		{
			name:  "leading zeros",
			input: []float64{0, 0, 0, 1.5, 2.0},
			want:  []float64{1.5, 2.0},
		},
		{
			name:  "no leading zeros",
			input: []float64{1.5, 2.0},
			want:  []float64{1.5, 2.0},
		},
		{
			name:  "all zeros",
			input: []float64{0, 0, 0},
			want:  []float64{},
		},
		{
			name:  "empty slice",
			input: []float64{},
			want:  []float64{},
		},
		{
			name:  "single non-zero",
			input: []float64{1.5},
			want:  []float64{1.5},
		},
		{
			name:  "single zero",
			input: []float64{0},
			want:  []float64{},
		},
		{
			name:  "zeros with negative",
			input: []float64{0, 0, -1.5, 2.0},
			want:  []float64{-1.5, 2.0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			result := StripLeadingZeros(tt.input)

			// Assert
			assert.Equal(t, tt.want, result)
		})
	}
}
