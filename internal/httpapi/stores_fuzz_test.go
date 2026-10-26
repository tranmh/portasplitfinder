package httpapi

import (
	"strconv"
	"testing"
)

// FuzzIntParam: an empty value yields the default; otherwise ok is true exactly
// when the value is a decimal integer in [lo, hi], and then the number is returned.
func FuzzIntParam(f *testing.F) {
	f.Add("", 20, 1, 100)
	f.Add("1", 20, 1, 100)
	f.Add("100", 20, 1, 100)
	f.Add("101", 20, 1, 100)
	f.Add("0", 1, 1, 1<<31-1)
	f.Add("-5", 1, -10, 10)
	f.Add("abc", 1, 1, 100)
	f.Add("+7", 1, 1, 100)
	f.Add(" 7", 1, 1, 100)
	f.Add("99999999999999999999999", 1, 1, 100)
	f.Add("\xff\xfe", 1, 1, 100)
	f.Fuzz(func(t *testing.T, v string, def, lo, hi int) {
		n, ok := intParam(v, def, lo, hi)
		if v == "" {
			if !ok || n != def {
				t.Fatalf("intParam(%q, %d, %d, %d) = (%d, %v), want (%d, true)", v, def, lo, hi, n, ok, def)
			}
			return
		}
		want, err := strconv.Atoi(v)
		wantOK := err == nil && want >= lo && want <= hi
		if ok != wantOK {
			t.Fatalf("intParam(%q, %d, %d, %d) ok = %v, want %v", v, def, lo, hi, ok, wantOK)
		}
		if ok && (n != want || n < lo || n > hi) {
			t.Fatalf("intParam(%q, %d, %d, %d) = %d, want %d within [%d, %d]", v, def, lo, hi, n, want, lo, hi)
		}
	})
}
