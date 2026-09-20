package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewLogger(t *testing.T) {
	tests := []struct {
		name      string
		level     string
		wantError bool
	}{
		{name: "debug", level: "debug"},
		{name: "info", level: "info"},
		{name: "warn", level: "warn"},
		{name: "error", level: "error"},
		{name: "invalid", level: "trace", wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := newLogger(tt.level)
			if tt.wantError {
				require.Error(t, err)
				require.Nil(t, logger)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, logger)
		})
	}
}
