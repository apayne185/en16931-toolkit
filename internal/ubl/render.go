// Package ubl renders EN 16931 invoices as UBL 2.1 XML.
// UBL (Universal Business Language) is one of the two syntax bindings
// defined in EN 16931-3; the other is UN/CEFACT CII (used by ZUGFeRD/Factur-X).
package ubl

import (
	"bytes"
	_ "embed"
	"encoding/xml"
	"fmt"
	"strings"
	"text/template"

	"github.com/apayne185/en16931-toolkit/internal/model"
)

//go:embed invoice.xml.tmpl
var rawTemplate string

// xmlEscape escapes s for safe embedding in XML text content and attribute values.
// text/template does not auto-escape XML, so all user-supplied string fields
// must pass through this function in the template.
func xmlEscape(s string) string {
	var buf strings.Builder
	xml.EscapeText(&buf, []byte(s)) //nolint:errcheck // writing to strings.Builder never fails
	return buf.String()
}

var tmpl = template.Must(
	template.New("ubl").Funcs(template.FuncMap{
		"x": xmlEscape,
		// amt formats a float64 as a monetary amount with exactly 2 decimal places.
		"amt": func(v float64) string { return fmt.Sprintf("%.2f", v) },
		// qty formats a quantity: integer quantities drop the decimal point.
		"qty": func(v float64) string {
			if v == float64(int64(v)) {
				return fmt.Sprintf("%.0f", v)
			}
			return fmt.Sprintf("%g", v)
		},
		// specID defaults to the PEPPOL BIS Billing 3.0 profile identifier,
		// which is the most widely deployed EN 16931 conformance level.
		"specID": func(s string) string {
			if s == "" {
				return "urn:cen.eu:en16931:2017#compliant#urn:fdc:peppol.eu:2017:poacc:billing:3.0"
			}
			return xmlEscape(s)
		},
	}).Parse(rawTemplate),
)

// Render produces a valid UBL 2.1 Invoice XML document from inv.
// The caller is responsible for ensuring inv has already passed Validate.
func Render(inv *model.Invoice) ([]byte, error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, inv); err != nil {
		return nil, fmt.Errorf("ubl render: %w", err)
	}
	return buf.Bytes(), nil
}
