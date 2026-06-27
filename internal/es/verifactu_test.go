package es_test

import (
	"strings"
	"testing"

	"github.com/apayne185/en16931-toolkit/internal/es"
	"github.com/apayne185/en16931-toolkit/internal/model"
	"github.com/apayne185/en16931-toolkit/internal/validate"
)

func TestChain_FirstInvoice(t *testing.T) {
	rec := es.Chain(es.ChainInput{
		SellerNIF:     "B66600888",
		InvoiceNumber: "INV-2024-001",
		IssueDate:     "01-06-2024",
		TypeCode:      "F1",
		TaxAmount:     "420.00",
		TotalAmount:   "2420.00",
	})

	if len(rec.Hash) != 64 {
		t.Errorf("hash should be 64 hex chars, got %d: %s", len(rec.Hash), rec.Hash)
	}
	if rec.Hash != strings.ToUpper(rec.Hash) {
		t.Error("hash should be uppercase hex")
	}
	if rec.HashType != es.HashTypeSHA256 {
		t.Errorf("expected hash type %q, got %q", es.HashTypeSHA256, rec.HashType)
	}
	if !strings.Contains(rec.QRVerifyURL, "agenciatributaria") {
		t.Errorf("QR URL should point to AEAT, got: %s", rec.QRVerifyURL)
	}
	if !strings.Contains(rec.QRVerifyURL, "B66600888") {
		t.Errorf("QR URL should contain seller NIF, got: %s", rec.QRVerifyURL)
	}
}

func TestChain_Deterministic(t *testing.T) {
	in := es.ChainInput{
		SellerNIF:     "B66600888",
		InvoiceNumber: "INV-2024-001",
		IssueDate:     "01-06-2024",
		TypeCode:      "F1",
		TaxAmount:     "420.00",
		TotalAmount:   "2420.00",
	}
	r1 := es.Chain(in)
	r2 := es.Chain(in)
	// The hash itself must be deterministic; only the timestamp varies.
	if r1.Hash != r2.Hash {
		t.Errorf("hash is not deterministic: %s vs %s", r1.Hash, r2.Hash)
	}
}

func TestChain_ChainLinkChangesHash(t *testing.T) {
	base := es.ChainInput{
		SellerNIF:     "B66600888",
		InvoiceNumber: "INV-2024-002",
		IssueDate:     "02-06-2024",
		TypeCode:      "F1",
		TaxAmount:     "210.00",
		TotalAmount:   "1210.00",
	}

	withoutPrev := es.Chain(base)

	base.PrevHash = "AABBCC00112233445566778899AABBCC00112233445566778899AABBCC001122"
	base.PrevTimestamp = "01-06-2024 10:00:00"
	withPrev := es.Chain(base)

	if withoutPrev.Hash == withPrev.Hash {
		t.Error("hash should differ when PrevHash changes — chain is broken")
	}
}

func TestChainFromInvoice(t *testing.T) {
	inv := minimalSpainInvoice()
	rec, err := es.ChainFromInvoice(inv, es.ChainRecord{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rec.Hash) != 64 {
		t.Errorf("expected 64-char hash, got %d", len(rec.Hash))
	}
}

func TestValidateSpain_Valid(t *testing.T) {
	inv := minimalSpainInvoice()
	errs := es.Validate(inv)
	if len(errs) != 0 {
		t.Errorf("expected 0 errors, got %d:", len(errs))
		for _, e := range errs {
			t.Errorf("  %s", e)
		}
	}
}

func TestValidateSpain_InvalidNIF(t *testing.T) {
	inv := minimalSpainInvoice()
	inv.Seller.VATID = "ESB12345679" // wrong control digit (should be 4)
	errs := es.Validate(inv)
	if !hasCode(errs, "ES-01") {
		t.Error("expected ES-01 for invalid NIF, but it did not fire")
	}
}

func TestValidateSpain_NonSpanishSeller(t *testing.T) {
	inv := minimalSpainInvoice()
	inv.Seller.Address.Country = "DE"
	errs := es.Validate(inv)
	if !hasCode(errs, "ES-02") {
		t.Error("expected ES-02 for non-Spanish seller country, but it did not fire")
	}
}

func TestValidateSpain_CorrectedInvoiceMissingRef(t *testing.T) {
	// TypeCorrectedInvoice (384) maps to Veri*Factu R-type but BR-25 (base rule)
	// only fires for TypeCreditNote (381). ES-04 covers the gap.
	inv := minimalSpainInvoice()
	inv.TypeCode = model.TypeCorrectedInvoice
	inv.PrecedingInvoiceRef = ""
	errs := es.Validate(inv)
	if !hasCode(errs, "ES-04") {
		t.Errorf("expected ES-04 for corrected invoice without preceding ref; got: %v", errs)
	}
}

func TestChainFromInvoice_InvalidTypeCode(t *testing.T) {
	inv := minimalSpainInvoice()
	inv.TypeCode = "999" // not a valid EN 16931 type code
	_, err := es.ChainFromInvoice(inv, es.ChainRecord{})
	if err == nil {
		t.Error("expected error for unmappable type code, got nil")
	}
}

func TestChainFromInvoice_AllTypeCodes(t *testing.T) {
	cases := []struct {
		code     model.InvoiceTypeCode
		wantType string
	}{
		{model.TypeCommercialInvoice, "F1"},
		{model.TypeSelfBilledInvoice, "F1"},
		{model.TypePrepaymentInvoice, "F1"},
		{model.TypeCreditNote, "R1"},
		{model.TypeCorrectedInvoice, "R1"},
	}
	for _, tc := range cases {
		inv := minimalSpainInvoice()
		inv.TypeCode = tc.code
		if tc.code == model.TypeCreditNote || tc.code == model.TypeCorrectedInvoice {
			inv.PrecedingInvoiceRef = "INV-PREV-001"
		}
		rec, err := es.ChainFromInvoice(inv, es.ChainRecord{})
		if err != nil {
			t.Errorf("type %s: unexpected error: %v", tc.code, err)
			continue
		}
		if len(rec.Hash) != 64 {
			t.Errorf("type %s: expected 64-char hash, got %d", tc.code, len(rec.Hash))
		}
	}
}

func TestValidateSpain_InvalidTypeCode(t *testing.T) {
	inv := minimalSpainInvoice()
	inv.TypeCode = "999"
	errs := es.Validate(inv)
	// Base rules fire first (BR-4 for invalid type code), es.Validate returns early.
	if len(errs) == 0 {
		t.Error("expected errors for invalid type code, got none")
	}
}

func hasCode(errs []validate.Error, code string) bool {
	for _, e := range errs {
		if e.Code == code {
			return true
		}
	}
	return false
}

func mustDate(s string) model.Date {
	var d model.Date
	if err := d.UnmarshalJSON([]byte(`"` + s + `"`)); err != nil {
		panic(err)
	}
	return d
}

func minimalSpainInvoice() *model.Invoice {
	return &model.Invoice{
		Number:         "INV-ES-001",
		IssueDate:      mustDate("2024-06-01"),
		TypeCode:       model.TypeCommercialInvoice,
		Currency:       "EUR",
		BuyerReference: "PO-1",
		Seller: model.Party{
			Name:  "Acme Software SL",
			VATID: "ESB12345674",
			Address: model.Address{
				Street:   "Calle Gran Vía 1",
				City:     "Madrid",
				PostCode: "28013",
				Country:  "ES",
			},
		},
		Buyer: model.Party{
			Name:    "DeepL SE",
			VATID:   "DE123456789",
			Address: model.Address{Country: "DE"},
		},
		VATBreakdown: []model.VATBreakdown{
			{Category: model.VATStandardRate, Rate: 21, TaxableAmount: 100.00, TaxAmount: 21.00},
		},
		Totals: model.Totals{
			LineNetTotal:       100.00,
			TaxExclusiveAmount: 100.00,
			TaxAmount:          21.00,
			TaxInclusiveAmount: 121.00,
			PayableAmount:      121.00,
		},
		Lines: []model.InvoiceLine{
			{
				ID:           "1",
				Quantity:     1,
				QuantityUnit: "C62",
				NetAmount:    100.00,
				VAT:          model.LineVAT{Category: model.VATStandardRate, Rate: 21},
				Item:         model.Item{Name: "Widget"},
				Price:        model.Price{Amount: 100.00},
			},
		},
	}
}
