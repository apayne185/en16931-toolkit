// Package validate implements EN 16931:2017 business rules (BR-* and BR-CO-*).
// Rule codes come directly from the normative specification published by CEN.
package validate

import (
	"fmt"
	"math"

	"github.com/apayne185/en16931-toolkit/internal/model"
)

// Error is a single EN 16931 business rule violation.
type Error struct {
	// Code is the rule identifier from the EN 16931 specification, e.g. "BR-CO-15".
	Code string
	// Path identifies the offending element using a short dot-notation path.
	Path string
	// Message explains the violation in human-readable terms.
	Message string
}

func (e Error) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("%s [%s]: %s", e.Code, e.Path, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Validate runs all implemented EN 16931 business rules against inv.
// It returns every violation found. An empty slice means the invoice is conformant.
func Validate(inv *model.Invoice) []Error {
	var errs []Error
	add := func(code, path, msg string) {
		errs = append(errs, Error{Code: code, Path: path, Message: msg})
	}

	checkStructural(inv, add)
	checkDocAllowancesCharges(inv, add)
	checkLines(inv, add)
	checkTotals(inv, add)
	checkVATBreakdown(inv, add)

	return errs
}

func checkStructural(inv *model.Invoice, add func(code, path, msg string)) {
	// BR-2: An invoice shall have an Invoice number.
	if inv.Number == "" {
		add("BR-2", "number", "invoice number is required")
	}

	// BR-3: An invoice shall have an Invoice issue date.
	if inv.IssueDate.IsZero() {
		add("BR-3", "issue_date", "invoice issue date is required")
	}

	// BR-4: An invoice shall have an Invoice type code.
	if inv.TypeCode == "" {
		add("BR-4", "type_code", "invoice type code is required")
	} else {
		switch inv.TypeCode {
		case model.TypeCommercialInvoice, model.TypeCreditNote,
			model.TypeCorrectedInvoice, model.TypeSelfBilledInvoice,
			model.TypePrepaymentInvoice:
			// valid
		default:
			add("BR-4", "type_code",
				fmt.Sprintf("type code %q is not in the allowed set (380, 381, 384, 386, 389) — see UNTDID 1001", inv.TypeCode))
		}
	}

	// BR-5: An invoice shall have an Invoice currency code (ISO 4217).
	if inv.Currency == "" {
		add("BR-5", "currency", "invoice currency code is required (ISO 4217, e.g. EUR)")
	} else if !iso4217[inv.Currency] {
		add("BR-5", "currency",
			fmt.Sprintf("%q is not a recognised ISO 4217 currency code", inv.Currency))
	}

	// BR-6: An invoice shall have a Seller name.
	if inv.Seller.Name == "" {
		add("BR-6", "seller.name", "seller name is required")
	}

	// BR-7: An invoice shall have a Buyer name.
	if inv.Buyer.Name == "" {
		add("BR-7", "buyer.name", "buyer name is required")
	}

	// BR-8: An invoice shall have the Seller country code (ISO 3166-1 alpha-2).
	if inv.Seller.Address.Country == "" {
		add("BR-8", "seller.address.country", "seller country code is required (ISO 3166-1 alpha-2)")
	} else if !iso3166alpha2[inv.Seller.Address.Country] {
		add("BR-8", "seller.address.country",
			fmt.Sprintf("%q is not a recognised ISO 3166-1 alpha-2 country code", inv.Seller.Address.Country))
	}

	// BR-9: An invoice shall have at least one invoice line.
	if len(inv.Lines) == 0 {
		add("BR-9", "lines", "at least one invoice line is required")
	}

	// BR-10: An invoice shall have a Buyer reference or a Purchase order reference.
	if inv.BuyerReference == "" && inv.OrderReference == "" {
		add("BR-10", "buyer_reference",
			"either a buyer reference (BT-10) or a purchase order reference (BT-13) is required")
	}

	// BR-25: Credit notes shall contain the preceding invoice reference.
	if inv.TypeCode == model.TypeCreditNote && inv.PrecedingInvoiceRef == "" {
		add("BR-25", "preceding_invoice_ref",
			"credit notes (type 381) must reference the original invoice number (BT-25)")
	}

	// BR-CO-9: Seller shall have at least one tax identifier.
	if inv.Seller.VATID == "" && inv.Seller.TaxID == "" && inv.Seller.LegalID == "" {
		add("BR-CO-9", "seller",
			"seller must have at least one of: VAT ID (BT-31), tax registration ID (BT-32), or legal registration ID (BT-30)")
	}

	// BR-29: Credit transfer payment means must carry a payment account identifier
	// that is a structurally valid IBAN (ISO 13616-1 mod-97 checksum).
	// UNCL4461 codes: 30 = credit transfer, 58 = SEPA credit transfer.
	for i, pm := range inv.PaymentMeans {
		p := fmt.Sprintf("payment_means[%d].account_id", i)
		if pm.TypeCode == "30" || pm.TypeCode == "58" {
			if pm.AccountID == "" {
				add("BR-29", p,
					fmt.Sprintf("payment means type %q (credit transfer) requires a payment account identifier (BT-84 IBAN)", pm.TypeCode))
			} else if !validateIBAN(pm.AccountID) {
				add("BR-29", p,
					fmt.Sprintf("payment account identifier %q is not a valid IBAN (ISO 13616-1)", pm.AccountID))
			}
		}
	}
}

func checkDocAllowancesCharges(inv *model.Invoice, add func(code, path, msg string)) {
	for i, a := range inv.Allowances {
		p := fmt.Sprintf("allowances[%d]", i)

		// BR-36: A document level allowance shall have a reason.
		if a.Reason == "" {
			add("BR-36", p+".reason",
				"document-level allowance must have a reason (BT-97)")
		}

		// BR-37: A document level allowance shall have a VAT category code.
		if a.VATCategory == "" {
			add("BR-37", p+".vat_category",
				"document-level allowance must have a VAT category code (BT-95)")
		}

		// BR-39: A document level allowance amount shall not be negative.
		if a.Amount < 0 {
			add("BR-39", p+".amount",
				fmt.Sprintf("document-level allowance amount (%.2f) must not be negative", a.Amount))
		}

		// BR-S-3: Standard-rated document allowances must have a non-zero VAT rate.
		if a.VATCategory == model.VATStandardRate && a.VATRate == 0 {
			add("BR-S-3", p+".vat_rate",
				"document-level allowance with VAT category 'S' must have a non-zero VAT rate (BT-96)")
		}
	}

	for i, c := range inv.Charges {
		p := fmt.Sprintf("charges[%d]", i)

		// BR-41: A document level charge shall have a reason.
		if c.Reason == "" {
			add("BR-41", p+".reason",
				"document-level charge must have a reason (BT-104)")
		}

		// BR-38: A document level charge shall have a VAT category code.
		if c.VATCategory == "" {
			add("BR-38", p+".vat_category",
				"document-level charge must have a VAT category code (BT-102)")
		}

		// BR-42: A document level charge amount shall not be negative.
		if c.Amount < 0 {
			add("BR-42", p+".amount",
				fmt.Sprintf("document-level charge amount (%.2f) must not be negative", c.Amount))
		}

		// BR-S-4: Standard-rated document charges must have a non-zero VAT rate.
		if c.VATCategory == model.VATStandardRate && c.VATRate == 0 {
			add("BR-S-4", p+".vat_rate",
				"document-level charge with VAT category 'S' must have a non-zero VAT rate (BT-103)")
		}
	}
}

func checkLines(inv *model.Invoice, add func(code, path, msg string)) {
	seenIDs := map[string]bool{}

	for i, line := range inv.Lines {
		p := fmt.Sprintf("lines[%d]", i)

		// BR-16: An invoice line shall have a unique identifier.
		if line.ID == "" {
			add("BR-16", p+".id", "invoice line identifier is required")
		} else if seenIDs[line.ID] {
			add("BR-16", p+".id",
				fmt.Sprintf("invoice line identifier %q is not unique", line.ID))
		} else {
			seenIDs[line.ID] = true
		}

		// BR-18: An invoice line shall have an invoiced quantity unit of measure code.
		if line.QuantityUnit == "" {
			add("BR-18", p+".quantity_unit",
				"invoiced quantity unit of measure code is required (UN/ECE Rec 20, e.g. C62, HUR, MON)")
		}

		// BR-19: An invoice line shall have a net amount.
		// Validate the math: net = quantity × price − allowances + charges.
		// Round once on the final sum, not after each term, to avoid accumulating
		// intermediate rounding errors on lines with multiple allowances/charges.
		baseQty := line.Price.BaseQuantity
		if baseQty == 0 {
			baseQty = 1
		}
		expectedNet := line.Quantity * line.Price.Amount / baseQty
		for _, a := range line.Allowances {
			expectedNet -= a.Amount
		}
		for _, c := range line.Charges {
			expectedNet += c.Amount
		}
		expectedNet = model.Round2(expectedNet)
		if math.Abs(model.Round2(line.NetAmount)-expectedNet) > 0.005 {
			add("BR-19", p+".net_amount",
				fmt.Sprintf("net amount %.2f does not match quantity (%.4f) × price (%.4f) − allowances + charges = %.2f",
					line.NetAmount, line.Quantity, line.Price.Amount, expectedNet))
		}

		// BR-20: An invoice line shall have an item name.
		if line.Item.Name == "" {
			add("BR-20", p+".item.name", "item name is required")
		}

		// BR-23: The item net price shall not be negative.
		if line.Price.Amount < 0 {
			add("BR-23", p+".price.amount",
				fmt.Sprintf("item net price (%.4f) must not be negative — use a line-level allowance for discounts", line.Price.Amount))
		}

		// BR-21: An invoice line shall have an item net price.
		if line.Price.Amount == 0 && line.NetAmount != 0 {
			add("BR-21", p+".price.amount",
				"item net price is required when the line net amount is non-zero")
		}

		// BR-26: An invoice line shall have a VAT category code.
		if line.VAT.Category == "" {
			add("BR-26", p+".vat.category", "invoice line VAT category code is required")
		}

		// BR-S-2: Standard-rated lines shall have a VAT rate.
		if line.VAT.Category == model.VATStandardRate && line.VAT.Rate == 0 {
			add("BR-S-2", p+".vat.rate",
				"standard-rated lines (category S) must specify a non-zero VAT rate")
		}
	}
}

func checkTotals(inv *model.Invoice, add func(code, path, msg string)) {
	t := inv.Totals

	// BR-CO-13: LineNetTotal shall equal the sum of line net amounts.
	var lineSum float64
	for _, l := range inv.Lines {
		lineSum += l.NetAmount
	}
	lineSum = model.Round2(lineSum)
	if math.Abs(model.Round2(t.LineNetTotal)-lineSum) > 0.005 {
		add("BR-CO-13", "totals.line_net_total",
			fmt.Sprintf("line_net_total %.2f must equal sum of line net amounts %.2f",
				t.LineNetTotal, lineSum))
	}

	// BR-CO-11: AllowanceTotal shall equal the sum of document-level allowance amounts.
	var allowSum float64
	for _, a := range inv.Allowances {
		allowSum += a.Amount
	}
	if math.Abs(model.Round2(t.AllowanceTotal)-model.Round2(allowSum)) > 0.005 {
		add("BR-CO-11", "totals.allowance_total",
			fmt.Sprintf("allowance_total %.2f must equal sum of document allowances %.2f",
				t.AllowanceTotal, allowSum))
	}

	// BR-CO-12: ChargeTotal shall equal the sum of document-level charge amounts.
	var chargeSum float64
	for _, c := range inv.Charges {
		chargeSum += c.Amount
	}
	if math.Abs(model.Round2(t.ChargeTotal)-model.Round2(chargeSum)) > 0.005 {
		add("BR-CO-12", "totals.charge_total",
			fmt.Sprintf("charge_total %.2f must equal sum of document charges %.2f",
				t.ChargeTotal, chargeSum))
	}

	// BR-CO-14: TaxExclusiveAmount = LineNetTotal − AllowanceTotal + ChargeTotal.
	expectedExcl := model.Round2(t.LineNetTotal - t.AllowanceTotal + t.ChargeTotal)
	if math.Abs(model.Round2(t.TaxExclusiveAmount)-expectedExcl) > 0.005 {
		add("BR-CO-14", "totals.tax_exclusive_amount",
			fmt.Sprintf("tax_exclusive_amount (%.2f) must equal line_net_total (%.2f) − allowance_total (%.2f) + charge_total (%.2f) = %.2f",
				t.TaxExclusiveAmount, t.LineNetTotal, t.AllowanceTotal, t.ChargeTotal, expectedExcl))
	}

	// BR-CO-15: TaxInclusiveAmount = TaxExclusiveAmount + TaxAmount.
	expectedIncl := model.Round2(t.TaxExclusiveAmount + t.TaxAmount)
	if math.Abs(model.Round2(t.TaxInclusiveAmount)-expectedIncl) > 0.005 {
		add("BR-CO-15", "totals.tax_inclusive_amount",
			fmt.Sprintf("tax_inclusive_amount (%.2f) must equal tax_exclusive_amount (%.2f) + tax_amount (%.2f) = %.2f",
				t.TaxInclusiveAmount, t.TaxExclusiveAmount, t.TaxAmount, expectedIncl))
	}

	// BR-CO-16: PayableAmount = TaxInclusiveAmount − PrepaidAmount + RoundingAmount.
	expectedPayable := model.Round2(t.TaxInclusiveAmount - t.PrepaidAmount + t.RoundingAmount)
	if math.Abs(model.Round2(t.PayableAmount)-expectedPayable) > 0.005 {
		add("BR-CO-16", "totals.payable_amount",
			fmt.Sprintf("payable_amount (%.2f) must equal tax_inclusive_amount (%.2f) − prepaid_amount (%.2f) + rounding_amount (%.2f) = %.2f",
				t.PayableAmount, t.TaxInclusiveAmount, t.PrepaidAmount, t.RoundingAmount, expectedPayable))
	}
}

func checkVATBreakdown(inv *model.Invoice, add func(code, path, msg string)) {
	// Collect the set of VAT categories used across lines, document allowances, and charges.
	// EN 16931-1 §10.7: the breakdown requirement applies to all three sources.
	usedCategories := map[model.VATCategoryCode]bool{}
	for _, l := range inv.Lines {
		usedCategories[l.VAT.Category] = true
	}
	for _, a := range inv.Allowances {
		if a.VATCategory != "" {
			usedCategories[a.VATCategory] = true
		}
	}
	for _, c := range inv.Charges {
		if c.VATCategory != "" {
			usedCategories[c.VATCategory] = true
		}
	}

	breakdownCategories := map[model.VATCategoryCode]bool{}
	for _, vb := range inv.VATBreakdown {
		breakdownCategories[vb.Category] = true
	}

	// BR-S-1 / BR-Z-1 / BR-E-1 / BR-AE-1 / BR-K-1 / BR-G-1 / BR-O-1 / BR-L-1 / BR-M-1:
	// Each VAT category used on lines, allowances, or charges must have a breakdown entry.
	// Ordered slice (not map) so error messages are always produced in spec order.
	type catRule struct {
		cat  model.VATCategoryCode
		rule string
	}
	categoryRules := []catRule{
		{model.VATStandardRate, "BR-S-1"},
		{model.VATZeroRated, "BR-Z-1"},
		{model.VATExempt, "BR-E-1"},
		{model.VATReverseCharge, "BR-AE-1"},
		{model.VATIntraCommunity, "BR-K-1"},
		{model.VATFreeExport, "BR-G-1"},
		{model.VATOutOfScope, "BR-O-1"},
		{model.VATCanaryIslands, "BR-L-1"},
		{model.VATCeutaMelilla, "BR-M-1"},
	}
	for _, cr := range categoryRules {
		if usedCategories[cr.cat] && !breakdownCategories[cr.cat] {
			add(cr.rule, "vat_breakdown",
				fmt.Sprintf("invoice contains lines, allowances, or charges with VAT category %q but vat_breakdown has no entry for that category", cr.cat))
		}
	}

	// BR-O-2: An invoice with a VAT breakdown entry "O" (out of scope) must not
	// contain breakdown entries with any other VAT category.
	if breakdownCategories[model.VATOutOfScope] && len(breakdownCategories) > 1 {
		add("BR-O-2", "vat_breakdown",
			"VAT category 'O' (out of scope) must not be mixed with other VAT categories in the breakdown")
	}

	// BR-O-3: If the VAT breakdown contains "O", all invoice lines must also be "O".
	if breakdownCategories[model.VATOutOfScope] {
		for i, l := range inv.Lines {
			if l.VAT.Category != model.VATOutOfScope {
				add("BR-O-3", fmt.Sprintf("lines[%d].vat.category", i),
					fmt.Sprintf("VAT breakdown contains 'O' (out of scope) but line %d has category %q — all lines must be 'O'",
						i, l.VAT.Category))
			}
		}
	}

	// BR-K-2: An invoice with a "K" (intra-community) VAT breakdown entry requires
	// the seller VAT identifier (BT-31) and the buyer VAT identifier (BT-48) or
	// buyer tax registration identifier (BT-49).
	if breakdownCategories[model.VATIntraCommunity] {
		if inv.Seller.VATID == "" {
			add("BR-K-2", "seller.vat_id",
				"intra-community supply (VAT category 'K') requires the seller VAT identifier (BT-31)")
		}
		if inv.Buyer.VATID == "" && inv.Buyer.TaxID == "" {
			add("BR-K-2", "buyer.vat_id",
				"intra-community supply (VAT category 'K') requires the buyer VAT identifier (BT-48) or tax registration identifier (BT-49)")
		}
	}

	// Per-entry validations.
	for i, vb := range inv.VATBreakdown {
		p := fmt.Sprintf("vat_breakdown[%d]", i)

		// BR-E-2: Exempt VAT breakdown entries must have a zero tax amount.
		if vb.Category == model.VATExempt && vb.TaxAmount != 0 {
			add("BR-E-2", p+".tax_amount",
				fmt.Sprintf("exempt category (E) must have tax_amount 0.00, got %.2f", vb.TaxAmount))
		}

		// BR-AE-3: Reverse charge entries must have a zero tax amount.
		if vb.Category == model.VATReverseCharge && vb.TaxAmount != 0 {
			add("BR-AE-3", p+".tax_amount",
				fmt.Sprintf("reverse charge category (AE) must have tax_amount 0.00, got %.2f — the buyer accounts for the VAT", vb.TaxAmount))
		}

		// BR-Z-2: Zero-rated entries must have a zero tax amount.
		if vb.Category == model.VATZeroRated && vb.TaxAmount != 0 {
			add("BR-Z-2", p+".tax_amount",
				fmt.Sprintf("zero-rated category (Z) must have tax_amount 0.00, got %.2f", vb.TaxAmount))
		}

		// BR-S-6: Standard-rated VAT amount must equal TaxableAmount × Rate / 100.
		if vb.Category == model.VATStandardRate {
			expectedTax := model.Round2(vb.TaxableAmount * vb.Rate / 100)
			if math.Abs(model.Round2(vb.TaxAmount)-expectedTax) > 0.005 {
				add("BR-S-6", p+".tax_amount",
					fmt.Sprintf("VAT amount (%.2f) must equal taxable_amount (%.2f) × rate (%.2f%%) = %.2f",
						vb.TaxAmount, vb.TaxableAmount, vb.Rate, expectedTax))
			}
		}
	}

	// BR-CO-17: Sum of VAT breakdown taxable amounts must equal TaxExclusiveAmount.
	var taxableSum float64
	for _, vb := range inv.VATBreakdown {
		taxableSum += vb.TaxableAmount
	}
	if math.Abs(model.Round2(taxableSum)-model.Round2(inv.Totals.TaxExclusiveAmount)) > 0.005 {
		add("BR-CO-17", "vat_breakdown",
			fmt.Sprintf("sum of vat_breakdown taxable amounts (%.2f) must equal tax_exclusive_amount (%.2f)",
				taxableSum, inv.Totals.TaxExclusiveAmount))
	}

	// BR-CO-18: Sum of VAT breakdown tax amounts must equal TaxAmount.
	var taxSum float64
	for _, vb := range inv.VATBreakdown {
		taxSum += vb.TaxAmount
	}
	if math.Abs(model.Round2(taxSum)-model.Round2(inv.Totals.TaxAmount)) > 0.005 {
		add("BR-CO-18", "vat_breakdown",
			fmt.Sprintf("sum of vat_breakdown tax amounts (%.2f) must equal totals.tax_amount (%.2f)",
				taxSum, inv.Totals.TaxAmount))
	}
}
