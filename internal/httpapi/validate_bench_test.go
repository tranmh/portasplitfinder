package httpapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var benchSinkBool bool

func BenchmarkValidEmail(b *testing.B) {
	for _, n := range []int{8, 64, 512} {
		in := strings.Repeat("a", n) + "@example.de"
		b.Run(fmt.Sprintf("local_len=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				benchSinkBool = validEmail(in)
			}
		})
	}
	b.Run("display_name", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			benchSinkBool = validEmail("A <a@b.de>")
		}
	})
}

func BenchmarkValidPLZ(b *testing.B) {
	for _, in := range []string{"73035", " 73035", strings.Repeat("7", 1024)} {
		b.Run(fmt.Sprintf("len=%d", len(in)), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				benchSinkBool = validPLZ(in)
			}
		})
	}
}

func BenchmarkValidRadius(b *testing.B) {
	radii := []int{0, 1, 50, 500, 501}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		benchSinkBool = validRadius(radii[i%len(radii)])
	}
}

func BenchmarkWriteError(b *testing.B) {
	for _, n := range []int{16, 256, 4096} {
		msg := strings.Repeat("x", n)
		field := fieldPtr("email")
		b.Run(fmt.Sprintf("msg_len=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				w := httptest.NewRecorder()
				writeError(w, http.StatusBadRequest, codeValidation, msg, field)
				benchSinkBool = w.Code == http.StatusBadRequest
			}
		})
	}
}

func BenchmarkWriteJSON(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		items := make([]map[string]any, n)
		for i := range items {
			items[i] = map[string]any{"id": fmt.Sprintf("id-%d", i), "inStock": i%2 == 0, "priceCents": 49900}
		}
		b.Run(fmt.Sprintf("items=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				w := httptest.NewRecorder()
				writeJSON(w, http.StatusOK, items)
				benchSinkBool = w.Body.Len() > 0
			}
		})
	}
}
