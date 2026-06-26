package ubl_test

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/apayne185/en16931-toolkit/internal/model"
	"github.com/apayne185/en16931-toolkit/internal/ubl"
)

// minimalInvoice returns the smallest invoice that passes EN 16931 validation.
func minimalInvoice() *model.Invoice {
	var d model.Date
	_ = d.UnmarshalJSON([]byte(`"2024-06-01"`))
	return &model.Invoice{
		Number:         "INV-TEST-001",
		IssueDate:      d,
		TypeCode:       model.TypeCommercialInvoice,
		Currency:       "EUR",
		BuyerReference: "PO-1",
		Seller: model.Party{
			Name:  "Seller Co",
			VATID: "ESB12345674",
			Address: model.Address{Country: "ES"},
		},
		Buyer: model.Party{
			Name:    "Buyer Co",
			Address: model.Address{Country: "DE"},
		},
		VATBreakdown: []model.VATBreakdown{
			{Category: model.VATStandardRate, Rate: 21, TaxableAmount: 100, TaxAmount: 21},
		},
		Totals: model.Totals{
			LineNetTotal:       100,
			TaxExclusiveAmount: 100,
			TaxAmount:          21,
			TaxInclusiveAmount: 121,
			PayableAmount:      121,
		},
		Lines: []model.InvoiceLine{{
			ID:           "1",
			Quantity:     1,
			QuantityUnit: "C62",
			NetAmount:    100,
			VAT:          model.LineVAT{Category: model.VATStandardRate, Rate: 21},
			Item:         model.Item{Name: "Widget"},
			Price:        model.Price{Amount: 100},
		}},
	}
}

func TestRender_WellFormedXML(t *testing.T) {
	out, err := ubl.Render(minimalInvoice())
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	if err := xml.Unmarshal(out, new(any)); err != nil {
		t.Errorf("output is not well-formed XML: %v", err)
	}
}

func TestRender_ContainsInvoiceNumber(t *testing.T) {
	out, err := ubl.Render(minimalInvoice())
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	if !strings.Contains(string(out), "INV-TEST-001") {
		t.Error("output should contain the invoice number")
	}
}

func TestRender_UBLNamespace(t *testing.T) {
	out, err := ubl.Render(minimalInvoice())
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	body := string(out)
	if !strings.Contains(body, "urn:oasis:names:specification:ubl:schema:xsd:Invoice-2") {
		t.Error("output should declare the UBL 2.1 Invoice namespace")
	}
}

func TestRender_XMLEscaping(t *testing.T) {
	inv := minimalInvoice()
	// Fields that contain XML special characters must be escaped in the output.
	inv.Number = `INV<001>&"test"`
	inv.Lines[0].Item.Name = `Widget <special> & "quoted"`
	inv.Seller.Name = `Seller & Co <Ltd>`
	inv.Notes = []string{`Note with <tags> & ampersands`}

	out, err := ubl.Render(inv)
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}

	// The output must be well-formed XML (injection would break parsing).
	if err := xml.Unmarshal(out, new(any)); err != nil {
		t.Errorf("output with special chars is not well-formed XML: %v", err)
	}

	body := string(out)
	// Raw unescaped characters must not appear in element content.
	if strings.Contains(body, `INV<001>`) {
		t.Error("unescaped < in invoice number — XML injection not prevented")
	}
	// Escaped form must be present.
	if !strings.Contains(body, "INV&lt;001&gt;") {
		t.Error("expected &lt; and &gt; escapes in invoice number")
	}
}

func TestRender_CreditNote(t *testing.T) {
	var due model.Date
	_ = due.UnmarshalJSON([]byte(`"2024-07-15"`))
	inv := minimalInvoice()
	inv.Number = "CN-2024-001"
	inv.TypeCode = model.TypeCreditNote
	inv.PrecedingInvoiceRef = "INV-2024-001"

	out, err := ubl.Render(inv)
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	body := string(out)
	if !strings.Contains(body, "BillingReference") {
		t.Error("credit note output should contain BillingReference element")
	}
	if !strings.Contains(body, "INV-2024-001") {
		t.Error("credit note output should contain the preceding invoice reference")
	}
}

func TestRender_ReverseCharge(t *testing.T) {
	inv := minimalInvoice()
	inv.Lines[0].VAT.Category = model.VATReverseCharge
	inv.Lines[0].VAT.Rate = 0
	inv.VATBreakdown[0].Category = model.VATReverseCharge
	inv.VATBreakdown[0].Rate = 0
	inv.VATBreakdown[0].TaxAmount = 0
	inv.VATBreakdown[0].ExemptionReason = "Reverse charge — Art. 196 Dir. 2006/112/EC"
	inv.VATBreakdown[0].ExemptionReasonCode = "VATEX-EU-AE"
	inv.Totals.TaxAmount = 0
	inv.Totals.TaxInclusiveAmount = 100
	inv.Totals.PayableAmount = 100

	out, err := ubl.Render(inv)
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	body := string(out)
	if !strings.Contains(body, "TaxExemptionReason") {
		t.Error("reverse charge output should contain TaxExemptionReason element")
	}
	if !strings.Contains(body, "VATEX-EU-AE") {
		t.Error("reverse charge output should contain the exemption reason code")
	}
}

func TestRender_PEPPOLSpecID(t *testing.T) {
	out, err := ubl.Render(minimalInvoice())
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	if !strings.Contains(string(out), "urn:cen.eu:en16931:2017#compliant#urn:fdc:peppol.eu:2017:poacc:billing:3.0") {
		t.Error("output should default to PEPPOL BIS Billing 3.0 specification ID")
	}
}
