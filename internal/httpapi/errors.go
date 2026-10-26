package httpapi

import (
	"encoding/json"
	"net/http"
)

// apiError is the contract's single error shape.
type apiError struct {
	Error   string  `json:"error"`
	Message string  `json:"message"`
	Field   *string `json:"field"`
}

// Common machine codes per the contract.
const (
	codeValidation = "validation_error"
	codeInvalidTok = "invalid_token"
	codeNotFound   = "not_found"
	codeInternal   = "internal"
	codeRateLimit  = "rate_limited"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string, field *string) {
	writeJSON(w, status, apiError{Error: code, Message: message, Field: field})
}

func fieldPtr(s string) *string { return &s }
