package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tranmh/portasplitfinder/internal/domain"
)

const jsonContentType = "application/json; charset=utf-8"

func decodeObject(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &m); err != nil {
		t.Fatalf("body is not a JSON object: %v (%q)", err, rr.Body.String())
	}
	return m
}

func TestWriteJSON(t *testing.T) {
	rr := httptest.NewRecorder()
	writeJSON(rr, http.StatusAccepted, map[string]string{"status": "pending_confirmation"})
	if rr.Code != http.StatusAccepted {
		t.Fatalf("status %d, want 202", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != jsonContentType {
		t.Errorf("Content-Type = %q, want %q", ct, jsonContentType)
	}
	if got := decodeObject(t, rr)["status"]; got != "pending_confirmation" {
		t.Errorf("status field = %v", got)
	}
}

func TestWriteError_Shape(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		code    string
		message string
		field   *string
	}{
		{"validation with field", http.StatusBadRequest, codeValidation, "plz must be 5 digits", fieldPtr("plz")},
		{"invalid token", http.StatusNotFound, codeInvalidTok, "unknown token", fieldPtr("token")},
		{"not found without field", http.StatusNotFound, codeNotFound, "no such thing", nil},
		{"internal without field", http.StatusInternalServerError, codeInternal, "internal server error", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			writeError(rr, c.status, c.code, c.message, c.field)
			if rr.Code != c.status {
				t.Fatalf("status %d, want %d", rr.Code, c.status)
			}
			if ct := rr.Header().Get("Content-Type"); ct != jsonContentType {
				t.Errorf("Content-Type = %q", ct)
			}
			m := decodeObject(t, rr)
			if len(m) != 3 {
				t.Errorf("error object has keys %v, want exactly error/message/field", m)
			}
			if m["error"] != c.code {
				t.Errorf("error = %v, want %q", m["error"], c.code)
			}
			if m["message"] != c.message {
				t.Errorf("message = %v, want %q", m["message"], c.message)
			}
			field, present := m["field"]
			if !present {
				t.Fatal("field key missing; contract requires field|null")
			}
			if c.field == nil {
				if field != nil {
					t.Errorf("field = %v, want null", field)
				}
			} else if field != *c.field {
				t.Errorf("field = %v, want %q", field, *c.field)
			}
		})
	}
}

func TestErrorCodesMatchContract(t *testing.T) {
	want := map[string]string{
		"validation": "validation_error",
		"invalidTok": "invalid_token",
		"notFound":   "not_found",
		"internal":   "internal",
	}
	got := map[string]string{
		"validation": codeValidation,
		"invalidTok": codeInvalidTok,
		"notFound":   codeNotFound,
		"internal":   codeInternal,
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("code %s = %q, want %q", k, got[k], w)
		}
	}
}

// failingStore makes every list call fail to exercise the internal error path.
type failingStore struct{ *fakeStore }

var errBoom = errors.New("boom")

func (failingStore) ListProducts(context.Context) ([]domain.Product, error) { return nil, errBoom }
func (failingStore) ListSources(context.Context) ([]domain.Source, error)   { return nil, errBoom }
func (failingStore) LatestSnapshots(context.Context) ([]domain.AvailabilitySnapshot, error) {
	return nil, errBoom
}

func TestInternalError_ContractShape(t *testing.T) {
	s := New(Config{Store: failingStore{newFakeStore()}})
	for _, path := range []string{"/api/products", "/api/sources", "/api/availability"} {
		t.Run(path, func(t *testing.T) {
			rr := do(s, http.MethodGet, path, nil)
			if rr.Code != http.StatusInternalServerError {
				t.Fatalf("status %d, want 500", rr.Code)
			}
			if ct := rr.Header().Get("Content-Type"); ct != jsonContentType {
				t.Errorf("Content-Type = %q", ct)
			}
			m := decodeObject(t, rr)
			if m["error"] != codeInternal {
				t.Errorf("error = %v, want internal", m["error"])
			}
			if f, ok := m["field"]; !ok || f != nil {
				t.Errorf("field = %v (present=%v), want null", f, ok)
			}
			if msg, _ := m["message"].(string); msg == "" || msg == errBoom.Error() {
				t.Errorf("message = %q, want a generic non-empty message", msg)
			}
		})
	}
}
