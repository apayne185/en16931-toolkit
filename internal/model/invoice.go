package model

import (
	"encoding/json"
	"fmt"
	"math"
	"time"
)

// Date is a calendar date with no time component, marshalled as "YYYY-MM-DD".
type Date struct{ time.Time }

func (d Date) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(d.Format("2006-01-02"))
}

func (d *Date) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return fmt.Errorf("date %q must be YYYY-MM-DD", s)
	}
	d.Time = t
	return nil
}

func (d Date) String() string {
	if d.IsZero() {
		return ""
	}
	return d.Format("2006-01-02")
}

// Round2 rounds a float64 to 2 decimal places.
func Round2(v float64) float64 { return math.Round(v*100) / 100 }

// InvoiceTypeCode is the document type indicator (UNTDID 1001).
type InvoiceTypeCode string

const (
	TypeCommercialInvoice InvoiceTypeCode = "380"
	TypeCreditNote        InvoiceTypeCode = "381"
	TypeCorrectedInvoice  InvoiceTypeCode = "384"
	TypeSelfBilledInvoice InvoiceTypeCode = "389"
	TypePrepaymentInvoice InvoiceTypeCode = "386"
)

// VATCategoryCode identifies how VAT applies to a supply (EN 16931 code list UNCL5305).
type VATCategoryCode string

const (
	VATStandardRate    VATCategoryCode = "S"  // Standard rated
	VATZeroRated       VATCategoryCode = "Z"  // Zero rated
	VATExempt          VATCategoryCode = "E"  // Exempt from tax
	VATReverseCharge   VATCategoryCode = "AE" // VAT reverse charge (intra-EU B2B services)
	VATIntraCommunity  VATCategoryCode = "K"  // EEA intra-community supply of goods
	VATFreeExport      VATCategoryCode = "G"  // Free export item, VAT not charged
	VATOutOfScope      VATCategoryCode = "O"  // Outside scope of VAT
	VATCanaryIslands   VATCategoryCode = "L"  // Canary Islands IGIC
	VATCeutaMelilla    VATCategoryCode = "M"  // Ceuta/Melilla IPSI
)

// Invoice is the top-level EN 16931:2017 invoice document.
// Field comments use the BT-/BG- identifiers from the semantic data model.
type Invoice struct {
	// BT-24: Specification identifier — identifies the business rules profile.
	SpecificationID string `json:"specification_id,omitempty"`
	// BT-1: Invoice number.
	Number string `json:"number"`
	// BT-2: Invoice issue date.
	IssueDate Date `json:"issue_date"`
	// BT-9: Payment due date.
	DueDate *Date `json:"due_date,omitempty"`
	// BT-3: Invoice type code (UNTDID 1001).
	TypeCode InvoiceTypeCode `json:"type_code"`
	// BT-5: Invoice currency code (ISO 4217).
	Currency string `json:"currency"`
	// BT-10: Buyer reference — a reference assigned by the buyer (e.g. cost centre).
	BuyerReference string `json:"buyer_reference,omitempty"`
	// BT-13: Purchase order reference.
	OrderReference string `json:"order_reference,omitempty"`
	// BT-25: Preceding invoice number — required on credit notes (BT-3 = 381).
	PrecedingInvoiceRef string `json:"preceding_invoice_ref,omitempty"`
	// BT-22: Invoice-level notes.
	Notes []string `json:"notes,omitempty"`

	// BG-4: Seller (accounting supplier).
	Seller Party `json:"seller"`
	// BG-7: Buyer (accounting customer).
	Buyer Party `json:"buyer"`

	// BG-16: Payment instructions.
	PaymentMeans []PaymentMeans `json:"payment_means,omitempty"`

	// BG-20: Document level allowances.
	Allowances []AllowanceCharge `json:"allowances,omitempty"`
	// BG-21: Document level charges.
	Charges []AllowanceCharge `json:"charges,omitempty"`

	// BG-23: VAT breakdown — one entry per distinct VAT category + rate combination.
	VATBreakdown []VATBreakdown `json:"vat_breakdown"`

	// BG-22: Document monetary totals.
	Totals Totals `json:"totals"`

	// BG-25: Invoice lines.
	Lines []InvoiceLine `json:"lines"`
}

// Party represents either the seller (BG-4) or buyer (BG-7).
type Party struct {
	// BT-27 / BT-44: Trading name.
	Name string `json:"name"`
	// BT-30 / BT-47: Legal entity registration identifier.
	LegalID string `json:"legal_id,omitempty"`
	// BT-31 / BT-48: VAT identifier (e.g. "ESB12345678").
	VATID string `json:"vat_id,omitempty"`
	// BT-32: Tax registration identifier (non-VAT, e.g. NIF in Spain).
	TaxID string `json:"tax_id,omitempty"`
	// BG-5 / BG-8: Postal address.
	Address Address `json:"address"`
	// BT-41 / BT-56: Contact name.
	ContactName string `json:"contact_name,omitempty"`
	// BT-43 / BT-57: Contact email.
	ContactEmail string `json:"contact_email,omitempty"`
}

// Address is a postal address (BG-5 / BG-8).
type Address struct {
	// BT-35 / BT-50: Street name.
	Street string `json:"street,omitempty"`
	// BT-36 / BT-51: Additional street line.
	Street2 string `json:"street2,omitempty"`
	// BT-37 / BT-52: City name.
	City string `json:"city,omitempty"`
	// BT-38 / BT-53: Post code.
	PostCode string `json:"post_code,omitempty"`
	// BT-39 / BT-54: Country subdivision code (ISO 3166-2).
	Subdivision string `json:"subdivision,omitempty"`
	// BT-40 / BT-55: Country code (ISO 3166-1 alpha-2). Required.
	Country string `json:"country"`
}

// PaymentMeans describes how the invoice should be paid (BG-16).
type PaymentMeans struct {
	// BT-81: Payment means type code (UNCL4461). 58 = SEPA credit transfer, 30 = credit transfer.
	TypeCode string `json:"type_code"`
	// BT-84: Payment account identifier (IBAN).
	AccountID string `json:"account_id,omitempty"`
	// BT-85: Account name.
	AccountName string `json:"account_name,omitempty"`
	// BT-86: Payment service provider identifier (BIC/SWIFT).
	ServiceProviderID string `json:"service_provider_id,omitempty"`
	// BT-83: Remittance information (payment reference).
	RemittanceInfo string `json:"remittance_info,omitempty"`
}

// AllowanceCharge is a document-level allowance (BG-20) or charge (BG-21).
type AllowanceCharge struct {
	// BT-92 / BT-99: Human-readable reason.
	Reason string `json:"reason"`
	// BT-93 / BT-100: Reason code (UNCL7161).
	ReasonCode string `json:"reason_code,omitempty"`
	// BT-95 / BT-102: VAT category of the allowance/charge.
	VATCategory VATCategoryCode `json:"vat_category"`
	// BT-96 / BT-103: VAT rate for the allowance/charge.
	VATRate float64 `json:"vat_rate"`
	// BT-92 / BT-99: Amount (always a positive number).
	Amount float64 `json:"amount"`
}

// VATBreakdown is one row in the VAT summary table (BG-23).
// Each row groups all lines sharing the same VAT category and rate.
type VATBreakdown struct {
	// BT-118: VAT category code.
	Category VATCategoryCode `json:"category"`
	// BT-119: VAT rate. Zero for exempt/reverse-charge categories.
	Rate float64 `json:"rate"`
	// BT-116: Taxable amount for this VAT category (net of allowances/charges at this rate).
	TaxableAmount float64 `json:"taxable_amount"`
	// BT-117: VAT amount for this category. Must be 0 for AE, E, Z, G, O, K.
	TaxAmount float64 `json:"tax_amount"`
	// BT-120: Reason text when category is E (exempt).
	ExemptionReason string `json:"exemption_reason,omitempty"`
	// BT-121: Reason code when category is E (exempt). Use VATEX-EU-* codes.
	ExemptionReasonCode string `json:"exemption_reason_code,omitempty"`
}

// Totals holds the document-level monetary totals (BG-22).
// All amounts are in the invoice currency (BT-5).
type Totals struct {
	// BT-106: Sum of all invoice line net amounts.
	LineNetTotal float64 `json:"line_net_total"`
	// BT-107: Sum of document-level allowance amounts.
	AllowanceTotal float64 `json:"allowance_total,omitempty"`
	// BT-108: Sum of document-level charge amounts.
	ChargeTotal float64 `json:"charge_total,omitempty"`
	// BT-109: Invoice total without VAT = LineNetTotal − AllowanceTotal + ChargeTotal.
	TaxExclusiveAmount float64 `json:"tax_exclusive_amount"`
	// BT-110: Total VAT amount.
	TaxAmount float64 `json:"tax_amount"`
	// BT-112: Invoice total with VAT = TaxExclusiveAmount + TaxAmount.
	TaxInclusiveAmount float64 `json:"tax_inclusive_amount"`
	// BT-113: Prepaid amount already paid by the buyer.
	PrepaidAmount float64 `json:"prepaid_amount,omitempty"`
	// BT-114: Rounding amount applied to produce a round payable total.
	RoundingAmount float64 `json:"rounding_amount,omitempty"`
	// BT-115: Amount due = TaxInclusiveAmount − PrepaidAmount + RoundingAmount.
	PayableAmount float64 `json:"payable_amount"`
}

// InvoiceLine is a single line item on the invoice (BG-25).
type InvoiceLine struct {
	// BT-126: Line identifier. Must be unique within the invoice.
	ID string `json:"id"`
	// BT-127: Line-level free-text note.
	Note string `json:"note,omitempty"`
	// BT-129: Quantity of goods or services invoiced.
	Quantity float64 `json:"quantity"`
	// BT-130: Unit of measure code (UN/ECE Rec 20, e.g. "C62"=each, "HUR"=hour, "MON"=month).
	QuantityUnit string `json:"quantity_unit"`
	// BT-131: Line net amount = Quantity × UnitPrice − LineAllowances + LineCharges.
	NetAmount float64 `json:"net_amount"`
	// BT-132: Order line reference.
	OrderLineRef string `json:"order_line_ref,omitempty"`
	// BG-27: Line-level allowances.
	Allowances []LineAllowanceCharge `json:"allowances,omitempty"`
	// BG-28: Line-level charges.
	Charges []LineAllowanceCharge `json:"charges,omitempty"`
	// BG-30: VAT information for this line.
	VAT LineVAT `json:"vat"`
	// BG-31: Item (product or service) details.
	Item Item `json:"item"`
	// BG-29: Unit price details.
	Price Price `json:"price"`
}

// LineAllowanceCharge is a line-level allowance (BG-27) or charge (BG-28).
type LineAllowanceCharge struct {
	Amount     float64 `json:"amount"`
	Reason     string  `json:"reason,omitempty"`
	ReasonCode string  `json:"reason_code,omitempty"`
}

// LineVAT captures the VAT classification for an invoice line (BG-30).
type LineVAT struct {
	// BT-151: VAT category code (must match a BG-23 entry).
	Category VATCategoryCode `json:"category"`
	// BT-152: VAT rate. Zero for non-standard categories.
	Rate float64 `json:"rate"`
}

// Item describes the product or service being invoiced (BG-31).
type Item struct {
	// BT-153: Item name. Required.
	Name string `json:"name"`
	// BT-154: Longer item description.
	Description string `json:"description,omitempty"`
	// BT-155: Seller's own item identifier (SKU).
	SellerID string `json:"seller_id,omitempty"`
	// BT-157: Standard item identifier (e.g. GTIN, schemeID "0160").
	StandardID string `json:"standard_id,omitempty"`
}

// Price holds unit pricing information for an invoice line (BG-29).
type Price struct {
	// BT-146: Item net price per base quantity, after any item-level discount.
	Amount float64 `json:"amount"`
	// BT-149: Base quantity the price applies to (defaults to 1 if omitted).
	BaseQuantity float64 `json:"base_quantity,omitempty"`
	// BT-150: Unit of measure for the base quantity.
	BaseQuantityUnit string `json:"base_quantity_unit,omitempty"`
}
