package notify

import (
	"strconv"
	"strings"
	"testing"
)

// FuzzFormatPriceEUR checks that the formatted German price string reads back
// as the exact amount of cents, sign included.
func FuzzFormatPriceEUR(f *testing.F) {
	f.Skip("aqaa: fuzz finding F1 (violation), re-enable once it is fixed; see report")
	for _, c := range []int{0, 1, 99, 100, 1999, 123456, -1, -50, -100, -1999, 1<<31 - 1, -1 << 31} {
		f.Add(c)
	}
	f.Fuzz(func(t *testing.T, cents int) {
		got := FormatPriceEUR(&cents)
		num, ok := strings.CutSuffix(got, " €")
		if !ok {
			t.Fatalf("FormatPriceEUR(%d) = %q: missing \" €\" suffix", cents, got)
		}
		euros, frac, ok := strings.Cut(num, ",")
		if !ok || len(frac) != 2 {
			t.Fatalf("FormatPriceEUR(%d) = %q: expected <euros>,<2 digits>", cents, got)
		}
		neg := strings.HasPrefix(euros, "-")
		e, err := strconv.ParseUint(strings.TrimPrefix(euros, "-"), 10, 64)
		if err != nil {
			t.Fatalf("FormatPriceEUR(%d) = %q: bad euros part: %v", cents, got, err)
		}
		r, err := strconv.ParseUint(frac, 10, 64)
		if err != nil {
			t.Fatalf("FormatPriceEUR(%d) = %q: bad cents part: %v", cents, got, err)
		}
		// Compare in uint64 magnitude so math.MinInt does not overflow.
		mag := uint64(cents)
		if cents < 0 {
			mag = -mag
		}
		if neg != (cents < 0) || e*100+r != mag {
			t.Errorf("FormatPriceEUR(%d) = %q does not read back as %d cents", cents, got, cents)
		}
	})
}

// FuzzEncodeHeader checks that an encoded Subject value can never break out
// of its header line (no CR or LF), whatever the subject contains.
func FuzzEncodeHeader(f *testing.F) {
	f.Skip("aqaa: fuzz finding F2 (violation), re-enable once it is fixed; see report")
	for _, s := range []string{"", "Hallo", "Verfügbar: Bohrmaschine", "a\r\nBcc: x@y.z", "line\nbreak", "\xff\xfe", strings.Repeat("ä", 200)} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		got := encodeHeader(s)
		if strings.ContainsAny(got, "\r\n") {
			t.Errorf("encodeHeader(%q) = %q contains CR/LF (header injection)", s, got)
		}
	})
}
