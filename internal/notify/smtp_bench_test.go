package notify

import (
	"fmt"
	"strings"
	"testing"
)

var (
	benchSinkBytes  []byte
	benchSinkString string
)

func BenchmarkBuildMIME(b *testing.B) {
	for _, n := range []int{1 << 10, 16 << 10, 256 << 10} {
		b.Run(fmt.Sprintf("body=%d", n), func(b *testing.B) {
			msg := Message{
				To:      "kunde@example.de",
				Subject: "Portasplit verfügbar in München",
				Text:    strings.Repeat("a", n),
				HTML:    strings.Repeat("<p>ä</p>", n/9+1),
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchSinkBytes = buildMIME("Portasplit Finder <noreply@example.de>", msg)
			}
		})
	}
}

func BenchmarkEncodeHeader(b *testing.B) {
	for _, n := range []int{16, 256, 4096} {
		ascii := strings.Repeat("a", n)
		umlaut := strings.Repeat("a", n-1) + "ü"
		b.Run(fmt.Sprintf("ascii_len=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				benchSinkString = encodeHeader(ascii)
			}
		})
		b.Run(fmt.Sprintf("nonascii_len=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				benchSinkString = encodeHeader(umlaut)
			}
		})
	}
}

func BenchmarkExtractAddr(b *testing.B) {
	for _, n := range []int{8, 128, 2048} {
		in := strings.Repeat("N", n) + " <noreply@example.de>"
		b.Run(fmt.Sprintf("name_len=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				benchSinkString = extractAddr(in)
			}
		})
	}
}

func BenchmarkNewToken(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		t, err := NewToken()
		if err != nil {
			b.Fatal(err)
		}
		benchSinkString = t
	}
}
