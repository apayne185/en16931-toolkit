package validate

import "testing"

func TestValidateIBAN(t *testing.T) {
	valid := []struct {
		name string
		iban string
	}{
		{"Spanish IBAN", "ES9121000418450200051332"},
		{"Spanish IBAN with spaces", "ES91 2100 0418 4502 0005 1332"},
		{"German IBAN", "DE89370400440532013000"},
		{"French IBAN", "FR7630006000011234567890189"},
		{"GB IBAN", "GB29NWBK60161331926819"},
		{"Dutch IBAN", "NL91ABNA0417164300"},
		{"Italian IBAN", "IT60X0542811101000000123456"},
		{"lowercase — ISO 13616 is case-insensitive", "es9121000418450200051332"},
	}

	for _, tc := range valid {
		t.Run(tc.name, func(t *testing.T) {
			if !validateIBAN(tc.iban) {
				t.Errorf("expected %q to be valid", tc.iban)
			}
		})
	}

	invalid := []struct {
		name string
		iban string
	}{
		{"empty", ""},
		{"too short", "ES91"},
		{"wrong checksum", "ES9221000418450200051332"}, // check digits changed
		{"not an IBAN", "not-an-iban"},
		{"letters in check digit position", "ESXX21000418450200051332"},
		{"too long", "ES91210004184502000513320000000000000000"},
	}

	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			if validateIBAN(tc.iban) {
				t.Errorf("expected %q to be invalid", tc.iban)
			}
		})
	}
}
