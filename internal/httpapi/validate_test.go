package httpapi

import "testing"

func TestValidRadius_Boundaries(t *testing.T) {
	cases := []struct {
		km   int
		want bool
	}{
		{-1, false},
		{0, false},
		{1, true},
		{2, true},
		{499, true},
		{500, true},
		{501, false},
	}
	for _, c := range cases {
		if got := validRadius(c.km); got != c.want {
			t.Errorf("validRadius(%d) = %v, want %v", c.km, got, c.want)
		}
	}
}
