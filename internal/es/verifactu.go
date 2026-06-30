// Package es implements Spain's Veri*Factu CIUS (Country/User Implementation Specification),
// defined in Real Decreto 1007/2023, de 5 de diciembre.
//
// Veri*Factu layers three requirements on top of EN 16931:
//  1. Invoice chain integrity — each invoice carries a SHA-256 fingerprint of the
//     previous one, making the sequence tamper-evident without a central ledger.
//  2. Seller NIF/CIF validation — Spain requires a structurally valid tax ID,
//     not just any VAT identifier.
//  3. AEAT verification URL — a QR code that recipients can scan to check the
//     invoice against the tax authority's real-time registry.
//
// The chain hash input is the concatenation of eight fields joined by "&",
// hashed with SHA-256, and encoded as uppercase hexadecimal.
package es

import (
	"crypto/sha256"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/apayne185/en16931-toolkit/internal/model"
)

const (
	// aeatVerifyBase is the AEAT production URL for QR invoice verification.
	aeatVerifyBase = "https://www2.agenciatributaria.gob.es/wlpl/TIKE-CONT/ValidarQR"

	// firstChainSentinel is the placeholder used when there is no preceding invoice.
	firstChainSentinel = "0"

	// HashTypeSHA256 is the AEAT code for the SHA-256 algorithm (TipoHuella field).
	HashTypeSHA256 = "01"
)

// ChainInput holds the values needed to compute a Veri*Factu chain record.
// All string amounts use dot as the decimal separator and no thousands separator.
type ChainInput struct {
	// SellerNIF is the seller's Spanish tax identifier, without the "ES" prefix.
	SellerNIF string
	// InvoiceNumber is the full invoice identifier (NumSerieFactura).
	InvoiceNumber string
	// IssueDate is the invoice date in DD-MM-YYYY format.
	IssueDate string
	// TypeCode is the Veri*Factu document type: F1 (standard), F2 (simplified),
	// F3 (summary), R1–R5 (corrective subtypes).
	TypeCode string
	// TaxAmount is the total VAT charged, formatted to exactly 2 decimal places.
	TaxAmount string
	// TotalAmount is the invoice total including VAT, 2 decimal places.
	TotalAmount string
	// PrevHash is the Hash field from the preceding ChainRecord, or
	// firstChainSentinel ("0") for the first invoice in a series.
	PrevHash string
	// PrevTimestamp is the Timestamp from the preceding ChainRecord, or
	// firstChainSentinel ("0") for the first invoice.
	PrevTimestamp string
}

// ChainRecord is the output of Chain: the computed hash and verification URL
// for one invoice, ready to be attached to the XML or submitted to AEAT.
type ChainRecord struct {
	// Hash is the SHA-256 fingerprint of this invoice (uppercase hex, 64 chars).
	Hash string
	// Timestamp is when this hash was generated (DD-MM-YYYY HH:MM:SS local time).
	Timestamp string
	// HashType is the AEAT algorithm code (always "01" for SHA-256).
	HashType string
	// QRVerifyURL is the full AEAT URL for recipient QR verification.
	QRVerifyURL string
}

// Chain computes the Veri*Factu chain record for one invoice.
// Call it in invoice-number order; pass the previous record's Hash and
// Timestamp as PrevHash/PrevTimestamp, or leave them empty for the first invoice.
func Chain(in ChainInput) ChainRecord {
	if in.PrevHash == "" {
		in.PrevHash = firstChainSentinel
	}
	if in.PrevTimestamp == "" {
		in.PrevTimestamp = firstChainSentinel
	}

	now := time.Now().Format("02-01-2006 15:04:05")

	// Hash input: eight fields joined by "&" as specified in the AEAT technical
	// documentation for Veri*Factu registration records (RegistroFactura).
	hashInput := strings.Join([]string{
		in.SellerNIF,
		in.InvoiceNumber,
		in.IssueDate,
		in.TypeCode,
		in.TaxAmount,
		in.TotalAmount,
		in.PrevHash,
		in.PrevTimestamp,
	}, "&")

	sum := sha256.Sum256([]byte(hashInput))
	hash := fmt.Sprintf("%X", sum)

	params := url.Values{
		"nif":      {in.SellerNIF},
		"numserie": {in.InvoiceNumber},
		"fecha":    {in.IssueDate},
		"importe":  {in.TotalAmount},
	}
	qr := aeatVerifyBase + "?" + params.Encode()

	return ChainRecord{
		Hash:        hash,
		Timestamp:   now,
		HashType:    HashTypeSHA256,
		QRVerifyURL: qr,
	}
}

// ChainFromInvoice is a convenience wrapper that builds ChainInput from a
// model.Invoice and the previous record. Pass a zero ChainRecord for the first
// invoice in a series.
func ChainFromInvoice(inv *model.Invoice, prev ChainRecord) (ChainRecord, error) {
	nif := strings.TrimPrefix(strings.ToUpper(inv.Seller.VATID), "ES")
	if nif == "" {
		nif = strings.TrimPrefix(strings.ToUpper(inv.Seller.TaxID), "ES")
	}

	esDate := inv.IssueDate.Format("02-01-2006") // DD-MM-YYYY

	typeCode, err := mapTypeCode(inv.TypeCode)
	if err != nil {
		return ChainRecord{}, err
	}

	return Chain(ChainInput{
		SellerNIF:     nif,
		InvoiceNumber: inv.Number,
		IssueDate:     esDate,
		TypeCode:      typeCode,
		TaxAmount:     fmt.Sprintf("%.2f", inv.Totals.TaxAmount),
		TotalAmount:   fmt.Sprintf("%.2f", inv.Totals.TaxInclusiveAmount),
		PrevHash:      prev.Hash,
		PrevTimestamp: prev.Timestamp,
	}), nil
}

// mapTypeCode converts an EN 16931 invoice type code (UNTDID 1001) to the
// Veri*Factu TipoFactura code.
func mapTypeCode(code model.InvoiceTypeCode) (string, error) {
	switch code {
	case model.TypeCommercialInvoice:
		return "F1", nil
	case model.TypeCreditNote, model.TypeCorrectedInvoice:
		// R1 = corrective by law or judgment; default corrective type.
		// R2–R5 apply in specific circumstances; R1 is the safe default here.
		return "R1", nil
	case model.TypeSelfBilledInvoice:
		return "F1", nil
	case model.TypePrepaymentInvoice:
		return "F1", nil
	default:
		return "", fmt.Errorf("no Veri*Factu TipoFactura mapping for EN 16931 type code %q", code)
	}
}
