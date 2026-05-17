package passwordgen

import (
	"regexp"
	"testing"
)

var base64urlPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func TestGenerate(t *testing.T) {
	t.Run("default length is 24", func(t *testing.T) {
		if got := Generate(0); len(got) != 24 {
			t.Fatalf("len=%d want=24", len(got))
		}
	})

	t.Run("respects custom length", func(t *testing.T) {
		if got := Generate(12); len(got) != 12 {
			t.Fatalf("len=%d want=12", len(got))
		}
		if got := Generate(40); len(got) != 40 {
			t.Fatalf("len=%d want=40", len(got))
		}
	})

	t.Run("uses base64url charset", func(t *testing.T) {
		for i := 0; i < 20; i++ {
			got := Generate(24)
			if !base64urlPattern.MatchString(got) {
				t.Fatalf("password %q contains non-base64url chars", got)
			}
		}
	})

	t.Run("calls return different values", func(t *testing.T) {
		a := Generate(24)
		b := Generate(24)
		if a == b {
			t.Fatalf("expected different values, both = %q", a)
		}
	})
}
