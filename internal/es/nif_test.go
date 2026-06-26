package es_test

import (
	"testing"

	"github.com/apayne185/en16931-toolkit/internal/es"
)

func TestValidateNIF(t *testing.T) {
	valid := []struct {
		name string
		nif  string
	}{
		// NIF control letter: digits % 23 → index into "TRWAGMYFPDXBNJZSQVHLCKE"
		{"NIF with ES prefix", "ES12345678Z"},
		{"NIF lowercase prefix", "es12345678Z"},
		// CIF control: B,A,E,H → digit; P,Q,R,S,W → letter; others → either.
		// B12345674: evenSum=12, oddSum=14, total=26, ctrl=(10-6)%10=4 → '4'
		{"CIF SL (B)", "B12345674"},
		// A12345674: same digit block → same control
		{"CIF SA (A)", "A12345674"},
		// G1234567D: ctrl digit=4 → letter "JABCDEFGHI"[4]='D' (G accepts both)
		{"CIF association (G)", "G1234567D"},
		{"NIE X-prefix", "X1234567L"},
		{"NIE Y-prefix", "Y1234567X"},
		{"NIE Z-prefix", "Z1234567R"},
	}

	for _, tc := range valid {
		t.Run(tc.name, func(t *testing.T) {
			if err := es.ValidateNIF(tc.nif); err != nil {
				t.Errorf("expected valid, got error: %v", err)
			}
		})
	}

	invalid := []struct {
		name string
		nif  string
	}{
		{"wrong control letter", "12345678A"},
		{"too short", "1234567"},
		{"CIF bad first letter", "I12345678"},
		{"empty", ""},
		{"random string", "NOTANIF99"},
	}

	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			if err := es.ValidateNIF(tc.nif); err == nil {
				t.Errorf("expected error for %q, got nil", tc.nif)
			}
		})
	}
}
