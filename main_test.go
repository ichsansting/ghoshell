package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestDispatch(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantCode int
		wantOut  string // substring expected on stdout
		wantErr  string // substring expected on stderr
	}{
		{"launch dispatches", []string{"launch"}, 0, "launch", ""},
		{"pack dispatches", []string{"pack"}, 0, "pack", ""},
		{"no subcommand prints usage, non-zero", nil, 2, "", "usage:"},
		{"unknown subcommand prints usage, non-zero", []string{"bogus"}, 2, "", "unknown command"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			got := dispatch(tt.args, &out, &errb)
			if got != tt.wantCode {
				t.Errorf("exit code = %d, want %d", got, tt.wantCode)
			}
			if tt.wantOut != "" && !strings.Contains(out.String(), tt.wantOut) {
				t.Errorf("stdout = %q, want substring %q", out.String(), tt.wantOut)
			}
			if tt.wantErr != "" && !strings.Contains(errb.String(), tt.wantErr) {
				t.Errorf("stderr = %q, want substring %q", errb.String(), tt.wantErr)
			}
		})
	}
}
