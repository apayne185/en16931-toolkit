# en16931-toolkit

[![CI](https://github.com/apayne185/en16931-toolkit/actions/workflows/ci.yml/badge.svg)](https://github.com/apayne185/en16931-toolkit/actions/workflows/ci.yml)

A Go toolkit that validates invoices against the EN 16931:2017 European e-invoicing standard, renders them as UBL 2.1 XML, and implements Spain's Veri\*Factu CIUS — all with zero external dependencies.

## Background

The EU's e-invoicing directive (2014/55/EU) mandates that all public-sector suppliers in Europe issue invoices in a machine-readable format that conforms to **EN 16931-1:2017** — the semantic data model that defines what an invoice *means* across all EU member states. By 2028, the mandate will extend to B2B transactions in most countries under the ViDA (VAT in the Digital Age) reform package.

EN 16931 solves a coordination problem: Germany calls it a "Rechnung", France an "Facture", Spain a "Factura", but underneath they're all the same document. The standard defines ~60 required/conditional fields (Business Terms, BT-\*) and ~120 validation rules (BR-\*) that any conformant invoice must satisfy, regardless of the XML syntax used to transmit it.

Two XML syntax bindings are specified in EN 16931-3:
- **UBL 2.1** — used in PEPPOL BIS Billing 3.0, the network used by most EU countries
- **UN/CEFACT CII** — used in Germany's ZUGFeRD / France's Factur-X (PDF/A-3 hybrid format)

This toolkit implements the UBL 2.1 binding and the Spanish Veri\*Factu CIUS.

## Features

- **EN 16931 validation** — 51 BR-\* and BR-CO-\* rules with precise error messages citing the rule code and failing field path
- **UBL 2.1 rendering** — schema-conformant XML you can feed to a PEPPOL access point or an Italian SDI gateway, with full XML injection protection
- **Spain Veri\*Factu CIUS** — seller NIF/CIF/NIE validation, SHA-256 invoice chain hashing, and AEAT QR verification URL (Real Decreto 1007/2023)
- **HTTP API** — JSON REST API exposing all three operations
- **Zero external dependencies** — standard library only

## Quick start

```bash
git clone https://github.com/apayne185/en16931-toolkit
cd en16931-toolkit
go build -o en16931 ./cmd/en16931
```

## CLI

```bash
# Validate an invoice against EN 16931 business rules
./en16931 validate examples/simple_invoice.json

# Render to UBL 2.1 XML (validates first; exits non-zero if invalid)
./en16931 render examples/simple_invoice.json
./en16931 render examples/simple_invoice.json -o out.xml

# Compute a Veri*Factu chain record (first invoice in a series)
./en16931 verifactu examples/simple_invoice.json

# Chain to a previous invoice
./en16931 verifactu examples/simple_invoice.json \
  -prev-hash 3C9D...F1A2 \
  -prev-timestamp "01-06-2024 09:00:00"

# Start the HTTP API server
./en16931 serve              # default :8080
./en16931 serve -addr :9090
```

### Validation output

```
✓  INV-2024-001 passes EN 16931:2017 (51 rules checked)
```

```
✗  Validation failed (2 errors):

   BR-CO-15       tax_inclusive_amount (130.00) must equal tax_exclusive_amount
                  (100.00) + tax_amount (15.00) = 115.00
                  └─ at: totals.tax_inclusive_amount

   BR-25          credit notes (type 381) must reference the original invoice number (BT-25)
                  └─ at: preceding_invoice_ref
```

## HTTP API

All endpoints require `Content-Type: application/json`.

### `POST /v1/invoices/validate`

Validates an invoice against EN 16931 business rules.

```bash
curl -s -X POST http://localhost:8080/v1/invoices/validate \
  -H 'Content-Type: application/json' \
  -d @examples/simple_invoice.json | jq .
```

```json
{ "valid": true }
```

On failure, returns HTTP 422 with the rule violations:

```json
{
  "valid": false,
  "errors": [
    { "code": "BR-CO-15", "path": "totals.tax_inclusive_amount", "message": "..." }
  ]
}
```

### `POST /v1/invoices/render`

Validates and renders the invoice as UBL 2.1 XML. Returns `application/xml` on success, HTTP 422 JSON on validation failure.

```bash
curl -s -X POST http://localhost:8080/v1/invoices/render \
  -H 'Content-Type: application/json' \
  -d @examples/simple_invoice.json \
  -o out.xml
```

### `POST /v1/invoices/verifactu`

Validates against EN 16931 + Spain CIUS rules and returns the Veri\*Factu chain record. Pass `prev_hash` and `prev_timestamp` from the preceding invoice, or omit both for the first invoice in a series.

```bash
curl -s -X POST http://localhost:8080/v1/invoices/verifactu \
  -H 'Content-Type: application/json' \
  -d '{
    "invoice": '"$(cat examples/simple_invoice.json)"',
    "prev_hash": "",
    "prev_timestamp": ""
  }' | jq .
```

```json
{
  "valid": true,
  "hash_type": "01",
  "hash": "3C9D4F...A1B2",
  "timestamp": "01-06-2024 09:00:00",
  "qr_verify_url": "https://www2.agenciatributaria.gob.es/wlpl/TIKE-CONT/ValidarQR?fecha=..."
}
```

### `GET /healthz`

```bash
curl http://localhost:8080/healthz
# {"status":"ok"}
```

## Spain Veri\*Factu

[Real Decreto 1007/2023](https://www.boe.es/eli/es/rd/2023/12/05/1007) is Spain's mandatory e-invoicing regulation for businesses using computerised billing systems. It layers three requirements on top of EN 16931:

1. **Seller NIF/CIF/NIE validation** — the seller's tax ID must be structurally valid, not just present. The toolkit validates the Mod-23 check letter for NIF, the digit-summing algorithm for CIF, and NIE substitution.
2. **Invoice chain integrity** — each invoice carries a SHA-256 fingerprint of the previous one (over 8 canonical fields joined by `&`), making the sequence tamper-evident without a central ledger.
3. **AEAT QR verification** — a URL pointing to the tax authority's registry, constructed safely via `url.Values` to prevent injection.

Spain-specific validation rules implemented:

| Code  | Rule |
|-------|------|
| ES-01 | Seller VAT ID must start with "ES" and pass NIF/CIF/NIE structural validation |
| ES-02 | Seller country must be "ES" |
| ES-03 | Invoice type code must map to a Veri\*Factu TipoFactura (F1/R1) |
| ES-04 | Corrected invoices (type 384) must reference the preceding invoice |

## Invoice JSON format

Invoices are provided as JSON following the EN 16931 semantic model. Field names map directly to the BT-/BG- terminology from the specification.

```jsonc
{
  "number": "INV-2024-001",          // BT-1
  "issue_date": "2024-06-01",        // BT-2  (YYYY-MM-DD)
  "due_date": "2024-06-30",          // BT-9  (optional)
  "type_code": "380",                // BT-3  (380=invoice, 381=credit note, 384=corrected, 386=prepayment, 389=self-billed)
  "currency": "EUR",                 // BT-5  (ISO 4217)
  "buyer_reference": "PO-98765",     // BT-10 (or order_reference BT-13)

  "seller": {
    "name": "Acme Software SL",
    "vat_id": "ESB12345674",         // BT-31
    "legal_id": "B12345674",         // BT-30
    "address": { "street": "...", "city": "Madrid", "post_code": "28013", "country": "ES" }
  },

  "buyer": { /* same shape as seller */ },

  "payment_means": [                 // BG-16 (optional)
    { "type_code": "58", "account_id": "ES91 2100...", "service_provider_id": "CAIXESBBXXX" }
  ],

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
      "quantity": 10, "quantity_unit": "HUR",  // HUR=hour, MON=month, C62=each (UN/ECE Rec 20)
      "net_amount": 1000.00,
      "vat": { "category": "S", "rate": 21 },
      "item": { "name": "Consulting services" },
      "price": { "amount": 100.00 }
    }
  ]
}
```

See [examples/](examples/) for complete working invoices:
- [`simple_invoice.json`](examples/simple_invoice.json) — ES→DE, standard 21% VAT, SEPA payment
- [`credit_note.json`](examples/credit_note.json) — type 381, references original invoice
- [`reverse_charge.json`](examples/reverse_charge.json) — intra-EU B2B, VAT reverse charge (AE)

## Implemented EN 16931 business rules

51 of ~120 normative rules are currently implemented.

| Code | Rule |
|------|------|
| BR-2 | Invoice number required |
| BR-3 | Issue date required |
| BR-4 | Type code required and in allowed set (380/381/384/386/389) |
| BR-5 | Currency code required and must be a valid ISO 4217 alphabetic code |
| BR-6 | Seller name required |
| BR-7 | Buyer name required |
| BR-8 | Seller country code required and must be a valid ISO 3166-1 alpha-2 code; buyer country validated when present |
| BR-9 | At least one invoice line required |
| BR-10 | Buyer reference or purchase order reference required |
| BR-16 | Invoice line ID required and unique |
| BR-18 | Line quantity unit of measure code required |
| BR-19 | Line net amount = quantity × unit price − allowances + charges |
| BR-20 | Item name required |
| BR-21 | Item price required when line net amount ≠ 0 |
| BR-23 | Item net price must not be negative |
| BR-25 | Credit notes must reference the preceding invoice |
| BR-26 | Line VAT category code required |
| BR-29 | Credit transfer payment (type 30/58) must carry an account identifier (IBAN) |
| BR-36 | Document-level allowance must have a reason |
| BR-37 | Document-level allowance must have a VAT category code |
| BR-38 | Document-level charge must have a VAT category code |
| BR-39 | Document-level allowance amount must not be negative |
| BR-42 | Document-level charge amount must not be negative |
| BR-S-1 | Standard-rated lines must have a VAT breakdown entry |
| BR-S-2 | Standard-rated lines must have a non-zero VAT rate |
| BR-S-3 | Standard-rated document allowance must have a non-zero VAT rate |
| BR-S-4 | Standard-rated document charge must have a non-zero VAT rate |
| BR-S-6 | Standard-rated VAT amount = taxable amount × rate |
| BR-Z-1 | Zero-rated lines must have a VAT breakdown entry |
| BR-Z-2 | Zero-rated VAT amount must be 0 |
| BR-E-1 | Exempt lines must have a VAT breakdown entry |
| BR-E-2 | Exempt VAT amount must be 0 |
| BR-AE-1 | Reverse charge lines must have a VAT breakdown entry |
| BR-AE-3 | Reverse charge VAT amount must be 0 |
| BR-K-1 | Intra-community lines must have a VAT breakdown entry |
| BR-K-2 | Intra-community supply requires seller VAT ID and buyer VAT/tax ID |
| BR-G-1 | Free export lines must have a VAT breakdown entry |
| BR-O-1 | Out-of-scope lines must have a VAT breakdown entry |
| BR-O-2 | VAT category "O" must not be mixed with other categories in the breakdown |
| BR-O-3 | If breakdown contains "O", all invoice lines must also be "O" |
| BR-L-1 | Canary Islands IGIC lines must have a VAT breakdown entry |
| BR-M-1 | Ceuta/Melilla IPSI lines must have a VAT breakdown entry |
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

**ISO 4217 and ISO 3166-1 validation.** BR-5 and BR-8 validate currency and country codes against embedded lookup maps (177 active ISO 4217 currency codes, 249 ISO 3166-1 alpha-2 country codes), not just presence. A typo like `"eur"` or a made-up code like `"XX"` is caught at the rule level with a clear error message.

**No external dependencies.** The entire toolkit uses Go's standard library only (`encoding/json`, `text/template`, `crypto/sha256`, `net/http`, `flag`). The UBL XML is produced by an embedded template ([`internal/ubl/invoice.xml.tmpl`](internal/ubl/invoice.xml.tmpl)), making the mapping between the semantic model and the XML syntax easy to audit and extend.

**XML injection protection.** `text/template` does not auto-escape XML the way `html/template` escapes HTML. All user-supplied string fields in the UBL template pass through `xmlEscape` (backed by `encoding/xml.EscapeText`) so that an invoice number like `<script>` cannot break the output document.

**BR-19 rounding.** The net amount check accumulates allowances and charges in raw float64 and rounds once on the final sum, not after each term. Rounding after each step causes accumulated drift on lines with multiple adjustments and produces false positives.

**Layered CIUS architecture.** EN 16931 is the baseline; countries layer a national extension (CIUS) on top. The Spain package (`internal/es`) calls `validate.Validate` first, then appends its own rules — exactly the same pattern used in production e-invoicing platforms. Adding a new country means adding a new package without touching the core.

**Amounts as float64.** For a production system you'd want fixed-precision decimal arithmetic (`github.com/shopspring/decimal`). Each arithmetic check uses a 0.005 threshold: since both operands are rounded to two decimal places, the minimum real discrepancy is 0.01 (one cent), and 0.01 > 0.005 is always true — so any genuine one-cent error is caught while float64 representation noise (≪ 0.001) is ignored.

## References

- [EN 16931-1:2017](https://www.cen.eu/work/areas/ICT/eBusiness/Pages/WS-BII.aspx) — Semantic data model
- [PEPPOL BIS Billing 3.0](https://docs.peppol.eu/poacc/billing/3.0/) — PEPPOL's EN 16931 profile
- [UBL 2.1 Invoice schema](http://docs.oasis-open.org/ubl/os-UBL-2.1/) — OASIS UBL XSD
- [Real Decreto 1007/2023](https://www.boe.es/eli/es/rd/2023/12/05/1007) — Spain Veri\*Factu regulation
- [ViDA regulation](https://taxation-customs.ec.europa.eu/taxation/value-added-tax/digital-reporting-requirements_en) — EU mandate timeline for B2B e-invoicing
