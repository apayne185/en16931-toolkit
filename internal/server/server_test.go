package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/apayne185/en16931-toolkit/internal/server"
)

var handler = server.New()

func post(t *testing.T, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path,
		bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

const validInvoiceJSON = `{
  "number": "INV-TEST-001",
  "issue_date": "2024-06-01",
  "type_code": "380",
  "currency": "EUR",
  "buyer_reference": "PO-1",
  "seller": {
    "name": "Seller Co",
    "vat_id": "ESB12345674",
    "address": { "country": "ES" }
  },
  "buyer": { "name": "Buyer Co", "address": { "country": "DE" } },
  "vat_breakdown": [
    { "category": "S", "rate": 21, "taxable_amount": 100.00, "tax_amount": 21.00 }
  ],
  "totals": {
    "line_net_total": 100.00,
    "tax_exclusive_amount": 100.00,
    "tax_amount": 21.00,
    "tax_inclusive_amount": 121.00,
    "payable_amount": 121.00
  },
  "lines": [{
    "id": "1",
    "quantity": 1, "quantity_unit": "C62",
    "net_amount": 100.00,
    "vat": { "category": "S", "rate": 21 },
    "item": { "name": "Widget" },
    "price": { "amount": 100.00 }
  }]
}`

func TestHealthz(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestValidate_Valid(t *testing.T) {
	rr := post(t, "/v1/invoices/validate", validInvoiceJSON)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body)
	}
	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["valid"] != true {
		t.Errorf("expected valid=true, got %v", resp)
	}
}

func TestValidate_Invalid(t *testing.T) {
	body := `{"number":"","issue_date":"2024-06-01","type_code":"380","currency":"EUR",
              "buyer_reference":"PO-1",
              "seller":{"name":"S","vat_id":"ESB12345674","address":{"country":"ES"}},
              "buyer":{"name":"B","address":{"country":"DE"}},
              "vat_breakdown":[{"category":"S","rate":21,"taxable_amount":100,"tax_amount":21}],
              "totals":{"line_net_total":100,"tax_exclusive_amount":100,"tax_amount":21,"tax_inclusive_amount":121,"payable_amount":121},
              "lines":[{"id":"1","quantity":1,"quantity_unit":"C62","net_amount":100,"vat":{"category":"S","rate":21},"item":{"name":"Widget"},"price":{"amount":100}}]}`
	rr := post(t, "/v1/invoices/validate", body)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rr.Code)
	}
	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["valid"] != false {
		t.Errorf("expected valid=false")
	}
	errs, _ := resp["errors"].([]any)
	if len(errs) == 0 {
		t.Error("expected non-empty errors array")
	}
}

func TestRender_Valid(t *testing.T) {
	rr := post(t, "/v1/invoices/render", validInvoiceJSON)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/xml") {
		t.Errorf("expected application/xml, got %q", ct)
	}
	if !strings.Contains(rr.Body.String(), "<Invoice") {
		t.Error("response body should contain UBL Invoice element")
	}
	if !strings.Contains(rr.Body.String(), "INV-TEST-001") {
		t.Error("response body should contain invoice number")
	}
}

func TestRender_Invalid(t *testing.T) {
	rr := post(t, "/v1/invoices/render", `{"number":"","currency":"EUR","type_code":"380","issue_date":"2024-01-01","buyer_reference":"PO-1","seller":{"name":"S","vat_id":"ESB12345674","address":{"country":"ES"}},"buyer":{"name":"B","address":{"country":"DE"}},"vat_breakdown":[],"totals":{"line_net_total":0,"tax_exclusive_amount":0,"tax_amount":0,"tax_inclusive_amount":0,"payable_amount":0},"lines":[]}`)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rr.Code)
	}
}

func TestValidate_WrongContentType(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/invoices/validate",
		bytes.NewBufferString(validInvoiceJSON))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnsupportedMediaType {
		t.Errorf("expected 415, got %d", rr.Code)
	}
}

func TestVerifactu_Valid(t *testing.T) {
	body := `{"invoice":` + validInvoiceJSON + `}`
	rr := post(t, "/v1/invoices/verifactu", body)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body)
	}
	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["valid"] != true {
		t.Errorf("expected valid=true, got %v", resp)
	}
	hash, _ := resp["hash"].(string)
	if len(hash) != 64 {
		t.Errorf("expected 64-char hash, got %q", hash)
	}
	qr, _ := resp["qr_verify_url"].(string)
	if !strings.Contains(qr, "agenciatributaria") {
		t.Errorf("QR URL should point to AEAT, got %q", qr)
	}
}
