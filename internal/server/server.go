// Package server exposes the validator and renderer over HTTP.
// All endpoints consume and produce JSON; the render endpoint returns XML.
package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/apayne185/en16931-toolkit/internal/es"
	"github.com/apayne185/en16931-toolkit/internal/model"
	"github.com/apayne185/en16931-toolkit/internal/ubl"
	"github.com/apayne185/en16931-toolkit/internal/validate"
)

// New returns an http.Handler wired with all API routes.
func New() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/invoices/validate", handleValidate)
	mux.HandleFunc("POST /v1/invoices/render", handleRender)
	mux.HandleFunc("POST /v1/invoices/verifactu", handleVerifactu)
	mux.HandleFunc("GET /healthz", handleHealth)
	return logging(mux)
}

// Listen starts the server on addr (e.g. ":8080") and blocks until stopped.
func Listen(addr string) error {
	srv := &http.Server{
		Addr:         addr,
		Handler:      New(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	log.Printf("en16931 API listening on %s", addr)
	return srv.ListenAndServe()
}

type validationResponse struct {
	Valid  bool             `json:"valid"`
	Errors []validationErr  `json:"errors,omitempty"`
}

type validationErr struct {
	Code    string `json:"code"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message"`
}

// handleValidate checks an invoice against EN 16931 business rules.
//
// POST /v1/invoices/validate
// Body: model.Invoice (JSON)
// Response 200: { "valid": true }
// Response 422: { "valid": false, "errors": [...] }
func handleValidate(w http.ResponseWriter, r *http.Request) {
	inv, ok := decodeInvoice(w, r)
	if !ok {
		return
	}

	errs := validate.Validate(inv)
	resp := validationResponse{Valid: len(errs) == 0}
	for _, e := range errs {
		resp.Errors = append(resp.Errors, validationErr{
			Code:    e.Code,
			Path:    e.Path,
			Message: e.Message,
		})
	}

	status := http.StatusOK
	if !resp.Valid {
		status = http.StatusUnprocessableEntity
	}
	writeJSON(w, status, resp)
}

// handleRender validates an invoice and renders it as UBL 2.1 XML.
//
// POST /v1/invoices/render
// Body: model.Invoice (JSON)
// Response 200: UBL 2.1 XML (Content-Type: application/xml)
// Response 422: { "valid": false, "errors": [...] }
func handleRender(w http.ResponseWriter, r *http.Request) {
	inv, ok := decodeInvoice(w, r)
	if !ok {
		return
	}

	errs := validate.Validate(inv)
	if len(errs) > 0 {
		resp := validationResponse{Valid: false}
		for _, e := range errs {
			resp.Errors = append(resp.Errors, validationErr{e.Code, e.Path, e.Message})
		}
		writeJSON(w, http.StatusUnprocessableEntity, resp)
		return
	}

	xmlBytes, err := ubl.Render(inv)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "render failed: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.xml"`, safeFilename(inv.Number)))
	w.WriteHeader(http.StatusOK)
	w.Write(xmlBytes)
}

type verifactuRequest struct {
	Invoice       model.Invoice `json:"invoice"`
	PrevHash      string        `json:"prev_hash,omitempty"`
	PrevTimestamp string        `json:"prev_timestamp,omitempty"`
}

type verifactuResponse struct {
	Valid         bool            `json:"valid"`
	Errors        []validationErr `json:"errors,omitempty"`
	HashType      string          `json:"hash_type,omitempty"`
	Hash          string          `json:"hash,omitempty"`
	Timestamp     string          `json:"timestamp,omitempty"`
	QRVerifyURL   string          `json:"qr_verify_url,omitempty"`
}

// handleVerifactu validates against EN 16931 + Spain CIUS and returns the
// Veri*Factu chain record.
//
// POST /v1/invoices/verifactu
// Body: { "invoice": {...}, "prev_hash": "...", "prev_timestamp": "..." }
// Response 200: { "valid": true, "hash": "...", "timestamp": "...", "qr_verify_url": "..." }
// Response 422: { "valid": false, "errors": [...] }
func handleVerifactu(w http.ResponseWriter, r *http.Request) {
	if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		writeError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var req verifactuRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	errs := es.Validate(&req.Invoice)
	if len(errs) > 0 {
		resp := verifactuResponse{Valid: false}
		for _, e := range errs {
			resp.Errors = append(resp.Errors, validationErr{e.Code, e.Path, e.Message})
		}
		writeJSON(w, http.StatusUnprocessableEntity, resp)
		return
	}

	prev := es.ChainRecord{Hash: req.PrevHash, Timestamp: req.PrevTimestamp}
	rec, err := es.ChainFromInvoice(&req.Invoice, prev)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, verifactuResponse{
		Valid:       true,
		HashType:    rec.HashType,
		Hash:        rec.Hash,
		Timestamp:   rec.Timestamp,
		QRVerifyURL: rec.QRVerifyURL,
	})
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

const maxBodyBytes = 1 << 20 // 1 MiB — invoices are never this large

// decodeInvoice reads and decodes a model.Invoice from the request body.
func decodeInvoice(w http.ResponseWriter, r *http.Request) (*model.Invoice, bool) {
	if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		writeError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return nil, false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var inv model.Invoice
	if err := json.NewDecoder(r.Body).Decode(&inv); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return nil, false
	}
	return &inv, true
}

// safeFilename strips characters that are illegal or dangerous in a
// Content-Disposition filename value: quotes break the header syntax,
// CR/LF enable header injection.
func safeFilename(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '"', '\\', '\r', '\n':
			return '_'
		}
		return r
	}, s)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// logging is a minimal middleware that logs method, path, and status.
func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(rw, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rw.status, time.Since(start))
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}
