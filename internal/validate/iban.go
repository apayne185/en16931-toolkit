package validate

import "strings"

// validateIBAN returns true if s is a structurally valid IBAN per ISO 13616-1.
// It checks length bounds, character set, and the mod-97 checksum — but not
// whether the country prefix is an active IBAN country (that would require a
// separate country-length table; the mod-97 check catches most typos anyway).
func validateIBAN(s string) bool {
	// Normalise: remove spaces, uppercase.
	s = strings.ToUpper(strings.ReplaceAll(s, " ", ""))

	// IBAN: 2-letter country, 2 check digits, 11–30 alphanumeric BBAN chars.
	if len(s) < 15 || len(s) > 34 {
		return false
	}

	// First four characters: 2 alpha (country) + 2 digits (check).
	for _, r := range s[:2] {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	for _, r := range s[2:4] {
		if r < '0' || r > '9' {
			return false
		}
	}

	// Remaining characters must be alphanumeric.
	for _, r := range s[4:] {
		if !((r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z')) {
			return false
		}
	}

	// Mod-97 check: move first 4 chars to the end, replace letters with digits
	// (A=10 … Z=35), compute the resulting number mod 97. Valid IBANs give 1.
	rearranged := s[4:] + s[:4]
	var num strings.Builder
	for _, r := range rearranged {
		if r >= 'A' && r <= 'Z' {
			// A→10, B→11, … Z→35. Values are always two digits (10-35).
			val := int(r-'A') + 10
			num.WriteByte(byte('0' + val/10))
			num.WriteByte(byte('0' + val%10))
		} else {
			num.WriteRune(r)
		}
	}

	return mod97(num.String()) == 1
}

// mod97 computes n mod 97 for an arbitrarily large decimal string without
// converting it to a big.Int — process left-to-right in 9-digit chunks.
func mod97(n string) int {
	remainder := 0
	for _, ch := range n {
		remainder = (remainder*10 + int(ch-'0')) % 97
	}
	return remainder
}
