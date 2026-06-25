# en16931-toolkit

A Go CLI that validates invoices against the EN 16931:2017 European e-invoicing standard and renders them as UBL 2.1 XML.

## Background

The EU's e-invoicing directive (2014/55/EU) mandates that all public-sector suppliers in Europe issue invoices in a machine-readable format that conforms to **EN 16931-1:2017** — the semantic data model that defines what an invoice *means* across all EU member states. By 2028, the mandate will extend to B2B transactions in most countries under the ViDA (VAT in the Digital Age) reform package.

EN 16931 solves a coordination problem: Germany calls it a "Rechnung", France an "Facture", Spain a "Factura", but underneath they're all the same document. The standard defines a single set of ~60 required/conditional fields (called Business Terms, BT-*) and ~120 validation rules (BR-*) that any conformant invoice must satisfy, regardless of the XML syntax used to transmit it.

The standard specifies two XML syntax bindings (EN 16931-3):
- **UBL 2.1** — used in PEPPOL BIS Billing 3.0, the network used by most EU countries
- **UN/CEFACT CII** — used in Germany's ZUGFeRD / France's Factur-X (PDF/A-3 hybrid format)

This toolkit implements the UBL 2.1 binding.

## Features

- **Business rule validation** — 36 BR-* and BR-CO-* rules from the normative specification, with precise error messages that cite the rule code and the failing field path
- **UBL 2.1 output** — generates schema-conformant XML you can feed to a PEPPOL access point or an Italian SDI gateway
- **Realistic examples** — covers the three most common scenarios in cross-border B2B invoicing: domestic standard-rated, credit note, and intra-EU reverse charge

## Usage

```bash
go build -o en16931 ./cmd/en16931

# Validate an invoice against EN 16931 business rules
./en16931 validate examples/simple_invoice.json

# Render to UBL 2.1 XML (validates first; exits non-zero if invalid)
./en16931 render examples/simple_invoice.json
./en16931 render examples/simple_invoice.json -o out.xml
```

### Example output — validation pass

```
✓  INV-2024-001 passes EN 16931:2017 (36 rules checked)
```

### Example output — validation failure

```
✗  Validation failed (2 errors):

   BR-CO-15       tax_inclusive_amount (130.00) must equal tax_exclusive_amount
                  └─ at: totals.tax_inclusive_amount
                  (100.00) + tax_amount (15.00) = 115.00

   BR-25          credit notes (type 381) must reference the original invoice number (BT-25)
                  └─ at: preceding_invoice_ref
```

## Invoice JSON format

Invoices are provided as JSON following the EN 16931 semantic model. Field names use the BT-/BG- terminology from the specification.

```jsonc
{
  "number": "INV-2024-001",          // BT-1
  "issue_date": "2024-06-01",        // BT-2  (YYYY-MM-DD)
  "due_date": "2024-06-30",          // BT-9  (optional)
  "type_code": "380",                // BT-3  (380=invoice, 381=credit note)
  "currency": "EUR",                 // BT-5  (ISO 4217)
  "buyer_reference": "PO-98765",     // BT-10 (or order_reference BT-13)

  "seller": {
    "name": "Acme Software SL",
    "vat_id": "ESB12345678",         // BT-31 — required for ES domestic invoices
    "legal_id": "B12345678",         // BT-30
    "address": { "street": "...", "city": "Madrid", "post_code": "28013", "country": "ES" }
  },

  "buyer": { /* same shape as seller */ },

  "vat_breakdown": [                 // BG-23 — one entry per VAT category+rate pair
    { "category": "S", "rate": 21, "taxable_amount": 1000.00, "tax_amount": 210.00 }
  ],

  "totals": {                        // BG-22 — all amounts must be arithmetically consistent
    "line_net_total": 1000.00,       // BT-106
    "tax_exclusive_amount": 1000.00, // BT-109
    "tax_amount": 210.00,            // BT-110
    "tax_inclusive_amount": 1210.00, // BT-112
    "payable_amount": 1210.00        // BT-115
  },

  "lines": [
    {
      "id": "1",
      "quantity": 10, "quantity_unit": "HUR",  // HUR=hour, MON=month, C62=each
      "net_amount": 1000.00,
      "vat": { "category": "S", "rate": 21 },
      "item": { "name": "Consulting services" },
      "price": { "amount": 100.00 }
    }
  ]
}
```

See [examples/](examples/) for complete working invoices covering:
- [`simple_invoice.json`](examples/simple_invoice.json) — domestic ES→DE invoice, standard 21% VAT
- [`credit_note.json`](examples/credit_note.json) — credit note (type 381) referencing original invoice
- [`reverse_charge.json`](examples/reverse_charge.json) — intra-EU B2B supply, VAT reverse charge (AE)

## Implemented business rules

| Code | Rule |
|------|------|
| BR-2 | Invoice number required |
| BR-3 | Issue date required |
| BR-4 | Type code required and in allowed set |
| BR-5 | Currency code required |
| BR-6 | Seller name required |
| BR-7 | Buyer name required |
| BR-8 | Seller country code required |
| BR-9 | At least one invoice line required |
| BR-10 | Buyer reference or purchase order reference required |
| BR-16 | Invoice line ID required and unique |
| BR-18 | Line quantity unit of measure code required |
| BR-19 | Line net amount = quantity × unit price − allowances + charges |
| BR-20 | Item name required |
| BR-21 | Item price required when line net amount ≠ 0 |
| BR-25 | Credit notes must reference the preceding invoice |
| BR-26 | Line VAT category code required |
| BR-S-2 | Standard-rated lines must have a non-zero VAT rate |
| BR-S-6 | Standard-rated VAT amount = taxable amount × rate |
| BR-Z-2 | Zero-rated VAT amount must be 0 |
| BR-E-2 | Exempt VAT amount must be 0 |
| BR-AE-3 | Reverse charge VAT amount must be 0 |
| BR-S/Z/E/AE/K/G/O-1 | VAT breakdown must cover every category used on lines |
| BR-CO-9 | Seller must have at least one tax identifier |
| BR-CO-11 | AllowanceTotal = sum of document allowances |
| BR-CO-12 | ChargeTotal = sum of document charges |
| BR-CO-13 | LineNetTotal = sum of line net amounts |
| BR-CO-14 | TaxExclusiveAmount = LineNetTotal − AllowanceTotal + ChargeTotal |
| BR-CO-15 | TaxInclusiveAmount = TaxExclusiveAmount + TaxAmount |
| BR-CO-16 | PayableAmount = TaxInclusiveAmount − PrepaidAmount + RoundingAmount |
| BR-CO-17 | Sum of VAT breakdown taxable amounts = TaxExclusiveAmount |
| BR-CO-18 | Sum of VAT breakdown tax amounts = TaxAmount |

## Design notes

**No external dependencies.** The entire tool uses Go's standard library only (`encoding/json`, `text/template`, `flag`). The UBL XML is produced by an embedded template ([`internal/ubl/invoice.xml.tmpl`](internal/ubl/invoice.xml.tmpl)), which makes the mapping between the semantic model and the XML syntax easy to audit and extend.

**Amounts as float64.** For a production integration you'd want fixed-precision decimal arithmetic (`github.com/shopspring/decimal` is the Go standard). The validator tolerates up to 0.01 rounding error on each check to accommodate accumulation across many lines; a real implementation would enforce exact equality.

**Extending to a new country.** EN 16931 is the baseline; most countries layer a national extension (CIUS) on top. Spain's Veri\*Factu regulation, for example, adds a mandatory `QRCode` element and a chain-hash across consecutive invoices. The architecture here is intentionally layered — a country-specific `validate/es` package would call `validate.Validate` first, then add its own rules, exactly as Invopop's App model works.

## References

- [EN 16931-1:2017](https://www.cen.eu/work/areas/ICT/eBusiness/Pages/WS-BII.aspx) — Semantic data model
- [PEPPOL BIS Billing 3.0](https://docs.peppol.eu/poacc/billing/3.0/) — PEPPOL's EN 16931 profile (most widely deployed)
- [UBL 2.1 Invoice schema](http://docs.oasis-open.org/ubl/os-UBL-2.1/) — OASIS UBL XSD
- [ViDA regulation](https://taxation-customs.ec.europa.eu/taxation/value-added-tax/digital-reporting-requirements_en) — EU mandate timeline for B2B e-invoicing
