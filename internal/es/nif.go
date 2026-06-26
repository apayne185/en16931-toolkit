package es

import (
	"fmt"
	"strings"
	"unicode"
)

// nifLetters is the control-character alphabet for Spanish NIF/NIE (Mod 23).
const nifLetters = "TRWAGMYFPDXBNJZSQVHLCKE"

// ValidateNIF checks whether s is a structurally valid Spanish tax identifier:
// NIF (8 digits + control letter), NIE (X/Y/Z + 7 digits + letter), or
// CIF (letter + 7 digits + digit-or-letter control).
//
// The "ES" VAT prefix is stripped automatically so both "B12345678" and
// "ESB12345678" are accepted.
func ValidateNIF(s string) error {
	s = strings.ToUpper(strings.TrimPrefix(strings.ToUpper(s), "ES"))
	if len(s) == 0 {
		return fmt.Errorf("empty identifier")
	}

	switch {
	case isDigit(rune(s[0])):
		return validateNIF(s)
	case s[0] == 'X' || s[0] == 'Y' || s[0] == 'Z':
		return validateNIE(s)
	default:
		return validateCIF(s)
	}
}

// validateNIF checks a physical-person NIF: 8 digits + 1 control letter.
func validateNIF(s string) error {
	if len(s) != 9 {
		return fmt.Errorf("NIF must be 9 characters (8 digits + letter), got %d", len(s))
	}
	for _, r := range s[:8] {
		if !isDigit(r) {
			return fmt.Errorf("NIF characters 1–8 must be digits")
		}
	}
	if !isLetter(rune(s[8])) {
		return fmt.Errorf("NIF character 9 must be a letter")
	}

	n := atoi(s[:8])
	expected := nifLetters[n%23]
	if s[8] != expected {
		return fmt.Errorf("NIF control letter is %q, expected %q", string(s[8]), string(expected))
	}
	return nil
}

// validateNIE checks a foreigner NIE: X/Y/Z + 7 digits + control letter.
func validateNIE(s string) error {
	if len(s) != 9 {
		return fmt.Errorf("NIE must be 9 characters, got %d", len(s))
	}
	prefix := map[byte]string{'X': "0", 'Y': "1", 'Z': "2"}[s[0]]
	return validateNIF(prefix + s[1:])
}

// validateCIF checks a legal-entity CIF: 1 letter + 7 digits + control digit or letter.
//
// CIF first-letter codes: A=SA, B=SL, C=SNC, D=SC, E=SCMD, F=Cooperativa,
// G=Asociación, H=ComunidadBienes, J=SCV, N=EntidadExtranjera,
// P=CorporaciónLocal, Q=OrganismoPúblico, R=Congregación, S=ÓrganoEstatal,
// U=UTE, V=OtraForma, W=EstablecimientoPermanente.
func validateCIF(s string) error {
	if len(s) != 9 {
		return fmt.Errorf("CIF must be 9 characters, got %d", len(s))
	}
	validFirst := "ABCDEFGHJNPQRSUVW"
	if !strings.ContainsRune(validFirst, rune(s[0])) {
		return fmt.Errorf("CIF first character %q is not a valid entity type code", string(s[0]))
	}
	digits := s[1:8]
	for _, r := range digits {
		if !isDigit(r) {
			return fmt.Errorf("CIF characters 2–8 must be digits")
		}
	}

	// Compute control: sum odd-position digits (after doubling & folding) + even-position digits.
	var evenSum, oddSum int
	for i, r := range digits {
		d := int(r - '0')
		if (i+1)%2 == 0 { // positions 2,4,6 are even (1-indexed within digit block)
			evenSum += d
		} else {
			d *= 2
			oddSum += d/10 + d%10
		}
	}
	total := (evenSum + oddSum) % 10
	control := (10 - total) % 10

	// Some entity types use a letter control; others use a digit.
	letterControl := "JABCDEFGHI"[control]
	digitControl := byte('0' + control)

	last := s[8]
	// Entity types P, Q, R, S, W always use letter control.
	// Entity types A, B, E, H always use digit control.
	// Others accept both.
	switch s[0] {
	case 'P', 'Q', 'R', 'S', 'W':
		if last != letterControl {
			return fmt.Errorf("CIF control character is %q, expected letter %q", string(last), string(letterControl))
		}
	case 'A', 'B', 'E', 'H':
		if last != digitControl {
			return fmt.Errorf("CIF control character is %q, expected digit %q", string(last), string(digitControl))
		}
	default:
		if last != letterControl && last != digitControl {
			return fmt.Errorf("CIF control character is %q, expected %q or %q", string(last), string(letterControl), string(digitControl))
		}
	}
	return nil
}

func isDigit(r rune) bool  { return r >= '0' && r <= '9' }
func isLetter(r rune) bool { return unicode.IsLetter(r) }

func atoi(s string) int {
	n := 0
	for _, r := range s {
		n = n*10 + int(r-'0')
	}
	return n
}
