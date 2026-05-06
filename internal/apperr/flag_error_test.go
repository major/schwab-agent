package apperr

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassifyFlagError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantOK   bool
		wantKind string
		wantFlag string
		wantGot  string
	}{
		{
			name:     "unknown flag",
			err:      errors.New("unknown flag: --bogus"),
			wantOK:   true,
			wantKind: FlagErrorUnknown,
			wantFlag: "bogus",
		},
		{
			name: "invalid long flag value",
			err: errors.New(
				`invalid argument "many" for "--count" flag: strconv.ParseInt: parsing "many": invalid syntax`,
			),
			wantOK:   true,
			wantKind: FlagErrorInvalidValue,
			wantFlag: "count",
			wantGot:  "many",
		},
		{
			name: "invalid short and long flag value",
			err: errors.New(
				`invalid argument "many" for "-c, --count" flag: strconv.ParseInt: parsing "many": invalid syntax`,
			),
			wantOK:   true,
			wantKind: FlagErrorInvalidValue,
			wantFlag: "count",
			wantGot:  "many",
		},
		{
			name:   "unrelated error",
			err:    errors.New(`unknown command "bogus" for "test"`),
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flagErr, ok := ClassifyFlagError(tt.err)
			assert.Equal(t, tt.wantOK, ok)
			if !tt.wantOK {
				assert.Nil(t, flagErr)
				return
			}

			require.NotNil(t, flagErr)
			assert.Equal(t, tt.wantKind, flagErr.Kind)
			assert.Equal(t, tt.wantFlag, flagErr.FlagName)
			assert.Equal(t, tt.wantGot, flagErr.Value)
			assert.ErrorIs(t, flagErr, tt.err)
		})
	}
}

func TestNormalizeFlagErrorFallback(t *testing.T) {
	cause := errors.New("pflag changed its error text")

	err := NormalizeFlagError(nil, cause)

	var flagErr *FlagError
	require.ErrorAs(t, err, &flagErr)
	assert.Equal(t, FlagErrorInvalidValue, flagErr.Kind)
	assert.Equal(t, 11, flagErr.ExitCode())
	assert.ErrorIs(t, err, cause)
}
