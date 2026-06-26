package es

import (
	"fmt"
	"strings"

	"github.com/apayne185/en16931-toolkit/internal/model"
	"github.com/apayne185/en16931-toolkit/internal/validate"
)

// Validate runs the base EN 16931 rules followed by Spain-specific CIUS rules.
// It returns a combined error list. The Spain rules only run if the base rules pass,
// since several Spain checks assume structurally valid data.
func Validate(inv *model.Invoice) []validate.Error {
	errs := validate.Validate(inv)
	if len(errs) > 0 {
		return errs
	}
	return validateSpain(inv)
}

func validateSpain(inv *model.Invoice) []validate.Error {
	var errs []validate.Error
	add := func(code, path, msg string) {
		errs = append(errs, validate.Error{Code: code, Path: path, Message: msg})
	}

	// ES-01: Seller VAT ID must be a structurally valid Spanish NIF/CIF.
	// Spain does not accept generic EU VAT IDs in the seller position for
	// domestic invoices — the AEAT chain hash uses the raw NIF as a key.
	if inv.Seller.VATID == "" {
		add("ES-01", "seller.vat_id",
			"Spanish invoices require a seller VAT ID (NIF/CIF)")
	} else if !strings.HasPrefix(strings.ToUpper(inv.Seller.VATID), "ES") {
		add("ES-01", "seller.vat_id",
			fmt.Sprintf("seller VAT ID %q must begin with country prefix ES for Spanish invoices", inv.Seller.VATID))
	} else if err := ValidateNIF(inv.Seller.VATID); err != nil {
		add("ES-01", "seller.vat_id",
			fmt.Sprintf("seller NIF/CIF is not structurally valid: %v", err))
	}

	// ES-02: Seller must be established in Spain (country code ES).
	// Veri*Factu applies to Spanish-resident businesses. Foreign sellers
	// submitting to Spanish buyers use the general EN 16931 rules only.
	if inv.Seller.Address.Country != "ES" {
		add("ES-02", "seller.address.country",
			fmt.Sprintf("Veri*Factu applies to sellers established in Spain (country ES); got %q", inv.Seller.Address.Country))
	}

	// ES-03: Invoice type code must be mappable to a Veri*Factu TipoFactura.
	if _, err := mapTypeCode(inv.TypeCode); err != nil {
		add("ES-03", "type_code", err.Error())
	}

	// ES-04: Credit/corrective invoices (R-type) must carry a preceding invoice reference.
	// This mirrors BR-25 but is worth re-stating clearly in the Spain context because
	// Veri*Factu uses R1–R5 as distinct document types, not just a flag.
	if inv.TypeCode == model.TypeCreditNote || inv.TypeCode == model.TypeCorrectedInvoice {
		if inv.PrecedingInvoiceRef == "" {
			add("ES-04", "preceding_invoice_ref",
				"corrective invoices (R-type in Veri*Factu) must reference the original invoice")
		}
	}

	return errs
}
