package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("body is not a JSON object: %v (%q)", err, rec.Body.String())
	}
	return m
}

func TestWriteJSON(t *testing.T) {
	cases := []struct {
		name     string
		status   int
		value    any
		wantBody string
	}{
		{"ok object", http.StatusOK, map[string]string{"status": "ok"}, `{"status":"ok"}`},
		{"accepted", http.StatusAccepted, map[string]string{"status": "pending_confirmation"}, `{"status":"pending_confirmation"}`},
		{"empty array stays array", http.StatusOK, map[string][]string{"products": {}}, `{"products":[]}`},
		{"service unavailable", http.StatusServiceUnavailable, map[string]string{"status": "degraded"}, `{"status":"degraded"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			writeJSON(rec, tc.status, tc.value)
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d", rec.Code, tc.status)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
				t.Fatalf("Content-Type = %q", ct)
			}
			var got, want any
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("invalid JSON %q: %v", rec.Body.String(), err)
			}
			_ = json.Unmarshal([]byte(tc.wantBody), &want)
			gb, _ := json.Marshal(got)
			wb, _ := json.Marshal(want)
			if string(gb) != string(wb) {
				t.Fatalf("body = %s, want %s", gb, wb)
			}
		})
	}
}

// B20: every error uses {error, message, field|null}.
func TestWriteError_Envelope(t *testing.T) {
	cases := []struct {
		name      string
		status    int
		code      string
		message   string
		field     *string
		wantField any
	}{
		{"validation with field", http.StatusBadRequest, codeValidation, "PLZ ungültig", fieldPtr("plz"), "plz"},
		{"validation radiusKm", http.StatusBadRequest, codeValidation, "Radius ungültig", fieldPtr("radiusKm"), "radiusKm"},
		{"missing token", http.StatusBadRequest, codeInvalidTok, "Token fehlt", nil, nil},
		{"unknown token", http.StatusNotFound, codeInvalidTok, "Token unbekannt", nil, nil},
		{"not found", http.StatusNotFound, codeNotFound, "nicht gefunden", nil, nil},
		{"internal", http.StatusInternalServerError, codeInternal, "Interner Fehler", nil, nil},
		{"empty field string", http.StatusBadRequest, codeValidation, "x", fieldPtr(""), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			writeError(rec, tc.status, tc.code, tc.message, tc.field)
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d", rec.Code, tc.status)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
				t.Fatalf("Content-Type = %q", ct)
			}
			m := decodeBody(t, rec)
			if len(m) != 3 {
				t.Fatalf("envelope must have exactly error/message/field, got %v", m)
			}
			field, ok := m["field"]
			if !ok {
				t.Fatalf("field key missing (must be present, null when unset): %v", m)
			}
			if field != tc.wantField {
				t.Fatalf("field = %#v, want %#v", field, tc.wantField)
			}
			if m["error"] != tc.code {
				t.Fatalf("error = %v, want %q", m["error"], tc.code)
			}
			if m["message"] != tc.message {
				t.Fatalf("message = %v, want %q", m["message"], tc.message)
			}
		})
	}
}

func TestErrorCodes(t *testing.T) {
	cases := map[string]string{
		codeValidation: "validation_error",
		codeInvalidTok: "invalid_token",
		codeNotFound:   "not_found",
		codeInternal:   "internal",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("code %q, want %q", got, want)
		}
	}
}

func TestFieldPtr(t *testing.T) {
	a := fieldPtr("email")
	b := fieldPtr("email")
	if a == nil || *a != "email" {
		t.Fatalf("fieldPtr = %v", a)
	}
	if a == b {
		t.Fatal("fieldPtr must return a fresh pointer per call")
	}
}
