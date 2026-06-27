package validate_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/apayne185/en16931-toolkit/internal/model"
	"github.com/apayne185/en16931-toolkit/internal/validate"
)

func loadFixture(t *testing.T, name string) *model.Invoice {
	t.Helper()
	f, err := os.Open("testdata/" + name)
	if err != nil {
		t.Fatalf("open fixture %s: %v", name, err)
	}
	defer f.Close()
	var inv model.Invoice
	if err := json.NewDecoder(f).Decode(&inv); err != nil {
		t.Fatalf("decode fixture %s: %v", name, err)
	}
	return &inv
}

// TestValidate_Examples validates every JSON file under examples/ at the repo
// root, ensuring published examples stay conformant as the validator evolves.
func TestValidate_Examples(t *testing.T) {
	entries, err := os.ReadDir("../../examples")
	if err != nil {
		t.Fatalf("cannot read examples dir: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		t.Run(e.Name(), func(t *testing.T) {
			f, err := os.Open(filepath.Join("../../examples", e.Name()))
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			defer f.Close()
			var inv model.Invoice
			if err := json.NewDecoder(f).Decode(&inv); err != nil {
				t.Fatalf("decode: %v", err)
			}
			errs := validate.Validate(&inv)
			if len(errs) != 0 {
				t.Errorf("expected 0 errors, got %d:", len(errs))
				for _, e := range errs {
					t.Errorf("  %s", e)
				}
			}
		})
	}
}

// TestValidate_Pass checks that all known-valid examples produce zero errors.
func TestValidate_Pass(t *testing.T) {
	cases := []string{
		"simple_invoice.json",
		"credit_note.json",
		"reverse_charge.json",
	}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			errs := validate.Validate(loadFixture(t, name))
			if len(errs) != 0 {
				t.Errorf("expected 0 errors, got %d:", len(errs))
				for _, e := range errs {
					t.Errorf("  %s", e)
				}
			}
		})
	}
}

// TestValidate_Fail checks that known-invalid fixtures fail with the expected rule codes.
func TestValidate_Fail(t *testing.T) {
	cases := []struct {
		fixture      string
		wantRules    []string // rules that must appear in the error list
		wantErrCount int      // exact number of errors expected
	}{
		{
			fixture:      "missing_header_fields.json",
			wantRules:    []string{"BR-2", "BR-4", "BR-5"},
			wantErrCount: 3,
		},
		{
			// BR-CO-16 does NOT fire: payable_amount (130) is consistent with the
			// (incorrect) tax_inclusive_amount (130). Rules are independent checks.
			fixture:      "totals_mismatch.json",
			wantRules:    []string{"BR-CO-15"},
			wantErrCount: 1,
		},
		{
			fixture:      "credit_note_missing_ref.json",
			wantRules:    []string{"BR-25"},
			wantErrCount: 1,
		},
		{
			// BR-CO-18 does NOT fire: tax_amount in totals is 0.00, consistent with
			// empty breakdown. BR-CO-17 fires because taxable sum (0) ≠ tax_exclusive (100).
			fixture:      "vat_breakdown_missing.json",
			wantRules:    []string{"BR-S-1", "BR-CO-17"},
			wantErrCount: 2,
		},
		{
			// Totals were written consistently with the wrong line amount (999 instead
			// of 1000), so only the line-level math rule fires. BR-S-6 passes because
			// 999 × 21% = 209.79 matches the breakdown's tax_amount.
			fixture:      "line_net_amount_wrong.json",
			wantRules:    []string{"BR-19"},
			wantErrCount: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			errs := validate.Validate(loadFixture(t, tc.fixture))

			if len(errs) != tc.wantErrCount {
				t.Errorf("expected %d errors, got %d:", tc.wantErrCount, len(errs))
				for _, e := range errs {
					t.Errorf("  %s", e)
				}
			}

			codes := make(map[string]bool, len(errs))
			for _, e := range errs {
				codes[e.Code] = true
			}
			for _, rule := range tc.wantRules {
				if !codes[rule] {
					t.Errorf("expected rule %s to fire, but it did not", rule)
				}
			}
		})
	}
}

// TestValidate_InlineEdgeCases covers rules that are awkward to express as JSON fixtures.
func TestValidate_InlineEdgeCases(t *testing.T) {
	t.Run("BR-10: no buyer ref and no order ref", func(t *testing.T) {
		inv := minimalValidInvoice()
		inv.BuyerReference = ""
		inv.OrderReference = ""
		assertRuleFires(t, inv, "BR-10")
	})

	t.Run("BR-10: order_reference alone satisfies the rule", func(t *testing.T) {
		inv := minimalValidInvoice()
		inv.BuyerReference = ""
		inv.OrderReference = "ORD-999"
		assertRuleAbsent(t, inv, "BR-10")
	})

	t.Run("BR-5: valid ISO 4217 currency passes", func(t *testing.T) {
		for _, code := range []string{"EUR", "USD", "GBP", "JPY", "CHF", "SEK", "PLN"} {
			inv := minimalValidInvoice()
			inv.Currency = code
			assertRuleAbsent(t, inv, "BR-5")
		}
	})

	t.Run("BR-5: unknown currency code fails", func(t *testing.T) {
		inv := minimalValidInvoice()
		inv.Currency = "ZZZ"
		assertRuleFires(t, inv, "BR-5")
	})

	t.Run("BR-5: lowercase currency code fails", func(t *testing.T) {
		inv := minimalValidInvoice()
		inv.Currency = "eur"
		assertRuleFires(t, inv, "BR-5")
	})

	t.Run("BR-8: valid ISO 3166-1 alpha-2 seller country passes", func(t *testing.T) {
		for _, code := range []string{"ES", "DE", "FR", "IT", "PT", "NL", "PL", "US"} {
			inv := minimalValidInvoice()
			inv.Seller.Address.Country = code
			assertRuleAbsent(t, inv, "BR-8")
		}
	})

	t.Run("BR-8: invalid seller country code fails", func(t *testing.T) {
		inv := minimalValidInvoice()
		inv.Seller.Address.Country = "XX"
		assertRuleFires(t, inv, "BR-8")
	})

	t.Run("BR-8: lowercase seller country code fails", func(t *testing.T) {
		inv := minimalValidInvoice()
		inv.Seller.Address.Country = "es"
		assertRuleFires(t, inv, "BR-8")
	})

	t.Run("BR-8: invalid buyer country code fails", func(t *testing.T) {
		inv := minimalValidInvoice()
		inv.Buyer.Address.Country = "XX"
		assertRuleFires(t, inv, "BR-8")
	})

	t.Run("BR-8: missing buyer country code passes (not required)", func(t *testing.T) {
		inv := minimalValidInvoice()
		inv.Buyer.Address.Country = ""
		assertRuleAbsent(t, inv, "BR-8")
	})

	t.Run("BR-16: duplicate line IDs", func(t *testing.T) {
		inv := minimalValidInvoice()
		inv.Lines = append(inv.Lines, inv.Lines[0]) // duplicate id "1"
		assertRuleFires(t, inv, "BR-16")
	})

	t.Run("BR-CO-9: seller with only legal_id satisfies the rule", func(t *testing.T) {
		inv := minimalValidInvoice()
		inv.Seller.VATID = ""
		inv.Seller.LegalID = "B12345678"
		assertRuleAbsent(t, inv, "BR-CO-9")
	})

	t.Run("BR-CO-9: seller with no tax identifiers fails", func(t *testing.T) {
		inv := minimalValidInvoice()
		inv.Seller.VATID = ""
		inv.Seller.LegalID = ""
		inv.Seller.TaxID = ""
		assertRuleFires(t, inv, "BR-CO-9")
	})

	t.Run("BR-AE-3: reverse charge with non-zero VAT tax amount", func(t *testing.T) {
		inv := minimalValidInvoice()
		inv.Lines[0].VAT.Category = model.VATReverseCharge
		inv.Lines[0].VAT.Rate = 0
		inv.VATBreakdown[0].Category = model.VATReverseCharge
		inv.VATBreakdown[0].Rate = 0
		inv.VATBreakdown[0].TaxAmount = 21.00 // must be 0 for AE
		assertRuleFires(t, inv, "BR-AE-3")
	})

	t.Run("BR-S-2: standard rate line with zero rate", func(t *testing.T) {
		inv := minimalValidInvoice()
		inv.Lines[0].VAT.Rate = 0
		assertRuleFires(t, inv, "BR-S-2")
	})
}

// TestValidate_NewRules covers the 11 rules added in the polish pass.
func TestValidate_NewRules(t *testing.T) {
	t.Run("BR-23: negative item price", func(t *testing.T) {
		inv := minimalValidInvoice()
		inv.Lines[0].Price.Amount = -10.00
		assertRuleFires(t, inv, "BR-23")
	})

	t.Run("BR-23: zero price is allowed", func(t *testing.T) {
		inv := minimalValidInvoice()
		inv.Lines[0].Price.Amount = 0
		inv.Lines[0].NetAmount = 0
		inv.Totals.LineNetTotal = 0
		inv.Totals.TaxExclusiveAmount = 0
		inv.Totals.TaxAmount = 0
		inv.Totals.TaxInclusiveAmount = 0
		inv.Totals.PayableAmount = 0
		inv.VATBreakdown[0].TaxableAmount = 0
		inv.VATBreakdown[0].TaxAmount = 0
		assertRuleAbsent(t, inv, "BR-23")
	})

	t.Run("BR-29: credit transfer without account ID", func(t *testing.T) {
		inv := minimalValidInvoice()
		inv.PaymentMeans = []model.PaymentMeans{{TypeCode: "58"}} // no AccountID
		assertRuleFires(t, inv, "BR-29")
	})

	t.Run("BR-29: SEPA transfer with account ID passes", func(t *testing.T) {
		inv := minimalValidInvoice()
		inv.PaymentMeans = []model.PaymentMeans{{TypeCode: "58", AccountID: "ES91 2100 0418 4502 0005 1332"}}
		assertRuleAbsent(t, inv, "BR-29")
	})

	t.Run("BR-29: non-transfer payment type without account ID passes", func(t *testing.T) {
		inv := minimalValidInvoice()
		inv.PaymentMeans = []model.PaymentMeans{{TypeCode: "10"}} // cash — no account ID needed
		assertRuleAbsent(t, inv, "BR-29")
	})

	t.Run("BR-37: document allowance missing VAT category", func(t *testing.T) {
		inv := minimalValidInvoice()
		inv.Allowances = []model.AllowanceCharge{{Reason: "Discount", Amount: 10}}
		assertRuleFires(t, inv, "BR-37")
	})

	t.Run("BR-38: document charge missing VAT category", func(t *testing.T) {
		inv := minimalValidInvoice()
		inv.Charges = []model.AllowanceCharge{{Reason: "Handling fee", Amount: 5}}
		assertRuleFires(t, inv, "BR-38")
	})

	t.Run("BR-S-3: standard-rated allowance with zero rate", func(t *testing.T) {
		inv := minimalValidInvoice()
		inv.Allowances = []model.AllowanceCharge{
			{Reason: "Discount", VATCategory: model.VATStandardRate, VATRate: 0, Amount: 10},
		}
		assertRuleFires(t, inv, "BR-S-3")
	})

	t.Run("BR-S-4: standard-rated charge with zero rate", func(t *testing.T) {
		inv := minimalValidInvoice()
		inv.Charges = []model.AllowanceCharge{
			{Reason: "Handling", VATCategory: model.VATStandardRate, VATRate: 0, Amount: 5},
		}
		assertRuleFires(t, inv, "BR-S-4")
	})

	t.Run("BR-L-1: Canary Islands line without breakdown entry", func(t *testing.T) {
		inv := minimalValidInvoice()
		inv.Lines[0].VAT.Category = model.VATCanaryIslands
		inv.Lines[0].VAT.Rate = 7
		// VAT breakdown still has "S" — no "L" entry
		assertRuleFires(t, inv, "BR-L-1")
	})

	t.Run("BR-M-1: Ceuta/Melilla line without breakdown entry", func(t *testing.T) {
		inv := minimalValidInvoice()
		inv.Lines[0].VAT.Category = model.VATCeutaMelilla
		inv.Lines[0].VAT.Rate = 4
		assertRuleFires(t, inv, "BR-M-1")
	})

	t.Run("BR-O-2: O breakdown mixed with other categories", func(t *testing.T) {
		inv := minimalValidInvoice()
		inv.Lines[0].VAT.Category = model.VATOutOfScope
		inv.VATBreakdown = []model.VATBreakdown{
			{Category: model.VATOutOfScope, TaxableAmount: 50, TaxAmount: 0},
			{Category: model.VATStandardRate, Rate: 21, TaxableAmount: 50, TaxAmount: 10.50},
		}
		assertRuleFires(t, inv, "BR-O-2")
	})

	t.Run("BR-O-3: O breakdown but line has different category", func(t *testing.T) {
		inv := minimalValidInvoice()
		inv.VATBreakdown = []model.VATBreakdown{
			{Category: model.VATOutOfScope, TaxableAmount: 100, TaxAmount: 0},
		}
		inv.Totals.TaxAmount = 0
		inv.Totals.TaxInclusiveAmount = 100
		inv.Totals.PayableAmount = 100
		// Line still has category S — violates BR-O-3
		assertRuleFires(t, inv, "BR-O-3")
	})

	t.Run("BR-O-3: all lines O passes", func(t *testing.T) {
		inv := minimalValidInvoice()
		inv.Lines[0].VAT.Category = model.VATOutOfScope
		inv.Lines[0].VAT.Rate = 0
		inv.VATBreakdown = []model.VATBreakdown{
			{Category: model.VATOutOfScope, TaxableAmount: 100, TaxAmount: 0},
		}
		inv.Totals.TaxAmount = 0
		inv.Totals.TaxInclusiveAmount = 100
		inv.Totals.PayableAmount = 100
		assertRuleAbsent(t, inv, "BR-O-3")
		assertRuleAbsent(t, inv, "BR-O-2")
	})

	t.Run("BR-K-2: K breakdown without seller VAT ID", func(t *testing.T) {
		inv := minimalValidInvoice()
		inv.Lines[0].VAT.Category = model.VATIntraCommunity
		inv.Lines[0].VAT.Rate = 0
		inv.VATBreakdown = []model.VATBreakdown{
			{Category: model.VATIntraCommunity, TaxableAmount: 100, TaxAmount: 0},
		}
		inv.Totals.TaxAmount = 0
		inv.Totals.TaxInclusiveAmount = 100
		inv.Totals.PayableAmount = 100
		inv.Seller.VATID = "" // no VAT ID
		inv.Seller.LegalID = "B12345678"
		inv.Buyer.VATID = "DE123456789"
		assertRuleFires(t, inv, "BR-K-2")
	})

	t.Run("BR-K-2: K breakdown without buyer VAT ID", func(t *testing.T) {
		inv := minimalValidInvoice()
		inv.Lines[0].VAT.Category = model.VATIntraCommunity
		inv.Lines[0].VAT.Rate = 0
		inv.VATBreakdown = []model.VATBreakdown{
			{Category: model.VATIntraCommunity, TaxableAmount: 100, TaxAmount: 0},
		}
		inv.Totals.TaxAmount = 0
		inv.Totals.TaxInclusiveAmount = 100
		inv.Totals.PayableAmount = 100
		inv.Buyer.VATID = ""
		inv.Buyer.TaxID = ""
		assertRuleFires(t, inv, "BR-K-2")
	})

	t.Run("BR-36: document allowance missing reason", func(t *testing.T) {
		inv := minimalValidInvoice()
		inv.Allowances = []model.AllowanceCharge{{VATCategory: model.VATStandardRate, VATRate: 21, Amount: 10}}
		assertRuleFires(t, inv, "BR-36")
	})

	t.Run("BR-36: allowance with reason passes", func(t *testing.T) {
		inv := minimalValidInvoice()
		inv.Allowances = []model.AllowanceCharge{{Reason: "Discount", VATCategory: model.VATStandardRate, VATRate: 21, Amount: 10}}
		assertRuleAbsent(t, inv, "BR-36")
	})

	t.Run("BR-39: negative document allowance amount", func(t *testing.T) {
		inv := minimalValidInvoice()
		inv.Allowances = []model.AllowanceCharge{{Reason: "Discount", VATCategory: model.VATStandardRate, VATRate: 21, Amount: -10}}
		assertRuleFires(t, inv, "BR-39")
	})

	t.Run("BR-39: zero allowance amount is allowed", func(t *testing.T) {
		inv := minimalValidInvoice()
		inv.Allowances = []model.AllowanceCharge{{Reason: "Discount", VATCategory: model.VATStandardRate, VATRate: 21, Amount: 0}}
		assertRuleAbsent(t, inv, "BR-39")
	})

	t.Run("BR-42: negative document charge amount", func(t *testing.T) {
		inv := minimalValidInvoice()
		inv.Charges = []model.AllowanceCharge{{Reason: "Handling", VATCategory: model.VATStandardRate, VATRate: 21, Amount: -5}}
		assertRuleFires(t, inv, "BR-42")
	})

	t.Run("BR-42: positive charge amount passes", func(t *testing.T) {
		inv := minimalValidInvoice()
		inv.Charges = []model.AllowanceCharge{{Reason: "Handling", VATCategory: model.VATStandardRate, VATRate: 21, Amount: 5}}
		assertRuleAbsent(t, inv, "BR-42")
	})

	t.Run("BR-K-2: K breakdown with both IDs passes", func(t *testing.T) {
		inv := minimalValidInvoice()
		inv.Lines[0].VAT.Category = model.VATIntraCommunity
		inv.Lines[0].VAT.Rate = 0
		inv.VATBreakdown = []model.VATBreakdown{
			{Category: model.VATIntraCommunity, TaxableAmount: 100, TaxAmount: 0},
		}
		inv.Totals.TaxAmount = 0
		inv.Totals.TaxInclusiveAmount = 100
		inv.Totals.PayableAmount = 100
		inv.Buyer.VATID = "DE123456789"
		assertRuleAbsent(t, inv, "BR-K-2")
	})
}

func TestError_String(t *testing.T) {
	withPath := validate.Error{Code: "BR-2", Path: "number", Message: "required"}
	if got := withPath.Error(); got != "BR-2 [number]: required" {
		t.Errorf("unexpected Error() with path: %q", got)
	}
	noPath := validate.Error{Code: "BR-9", Message: "at least one line required"}
	if got := noPath.Error(); got != "BR-9: at least one line required" {
		t.Errorf("unexpected Error() without path: %q", got)
	}
}

// assertRuleFires fails the test if rule code does not appear in the validation errors.
func assertRuleFires(t *testing.T, inv *model.Invoice, code string) {
	t.Helper()
	for _, e := range validate.Validate(inv) {
		if e.Code == code {
			return
		}
	}
	t.Errorf("expected rule %s to fire, but it did not", code)
}

// assertRuleAbsent fails the test if rule code appears in the validation errors.
func assertRuleAbsent(t *testing.T, inv *model.Invoice, code string) {
	t.Helper()
	for _, e := range validate.Validate(inv) {
		if e.Code == code {
			t.Errorf("expected rule %s NOT to fire, but it did: %s", code, e.Message)
			return
		}
	}
}

// minimalValidInvoice returns the smallest invoice that passes all EN 16931 rules.
func minimalValidInvoice() *model.Invoice {
	return &model.Invoice{
		Number:         "INV-TEST-001",
		IssueDate:      mustDate("2024-06-01"),
		TypeCode:       model.TypeCommercialInvoice,
		Currency:       "EUR",
		BuyerReference: "PO-1",
		Seller: model.Party{
			Name:  "Seller Co",
			VATID: "ESB99999999",
			Address: model.Address{
				Country: "ES",
			},
		},
		Buyer: model.Party{
			Name:    "Buyer Co",
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

func mustDate(s string) model.Date {
	var d model.Date
	if err := d.UnmarshalJSON([]byte(`"` + s + `"`)); err != nil {
		panic(err)
	}
	return d
}
