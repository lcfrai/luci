package doc

import (
	"errors"
	"math/rand"
	"testing"
)

// The vectors are worked from the algorithm in task-05.md, digit by digit.
// Alphabet index: '0'=0 … '9'=9, 'A'=10 … 'Z'=35, 'a'=36 … 'z'=61; 'V'=31.
func TestBetween_vectors(t *testing.T) {
	cases := []struct{ a, b, want string }{
		{"", "", "V"},    // da=0, db=62 -> floor(62/2)=31
		{"V", "", "k"},   // da=31, db=62 -> floor(93/2)=46
		{"", "V", "F"},   // da=0, db=31 -> floor(31/2)=15
		{"V", "W", "VV"}, // db-da==1: copy 'V', drop the bound, then 62/2
		{"V", "V1", "V0V"},
		{"z", "", "zV"},
		{"", "1", "0V"},
	}
	for _, c := range cases {
		got, err := Between(c.a, c.b)
		if err != nil {
			t.Errorf("Between(%q,%q) = error %v, want %q", c.a, c.b, err, c.want)
			continue
		}
		if got != c.want {
			t.Errorf("Between(%q,%q) = %q, want %q", c.a, c.b, got, c.want)
		}
	}
}

func TestBetween_errors(t *testing.T) {
	cases := [][2]string{
		{"V", "V"},  // equal bounds
		{"W", "V"},  // inverted bounds
		{"V0", ""},  // a ends in '0'
		{"V!", ""},  // a is outside the alphabet
		{"", "V0"},  // b ends in '0'
		{"", "V!"},  // b is outside the alphabet
		{"VV", "V"}, // inverted by length
	}
	for _, c := range cases {
		got, err := Between(c[0], c[1])
		if !errors.Is(err, ErrOrder) {
			t.Errorf("Between(%q,%q) = %q, %v; want ErrOrder", c[0], c[1], got, err)
		}
	}
}

func randKey(r *rand.Rand) string {
	b := make([]byte, 1+r.Intn(6))
	for i := range b {
		b[i] = OrderAlphabet[r.Intn(len(OrderAlphabet))]
	}
	if last := len(b) - 1; b[last] == '0' {
		b[last] = OrderAlphabet[1+r.Intn(len(OrderAlphabet)-1)]
	}
	return string(b)
}

func TestBetween_property(t *testing.T) {
	r := rand.New(rand.NewSource(20260923))
	for i := 0; i < 10000; i++ {
		a, b := randKey(r), randKey(r)
		if a > b {
			a, b = b, a
		}
		if a == b {
			continue
		}
		switch r.Intn(6) {
		case 0:
			a = "" // the bottom
		case 1:
			b = "" // the top
		}
		k, err := Between(a, b)
		if err != nil {
			t.Fatalf("Between(%q,%q): %v", a, b, err)
		}
		if !ValidOrder(k) {
			t.Fatalf("Between(%q,%q) = %q, which is not a valid order key", a, b, k)
		}
		if a != "" && k <= a {
			t.Fatalf("Between(%q,%q) = %q, which is not above a", a, b, k)
		}
		if b != "" && k >= b {
			t.Fatalf("Between(%q,%q) = %q, which is not below b", a, b, k)
		}
	}
}

func TestSpread(t *testing.T) {
	got, err := Spread("", "", 7)
	if err != nil {
		t.Fatalf("Spread(\"\",\"\",7): %v", err)
	}
	if len(got) != 7 {
		t.Fatalf("Spread(\"\",\"\",7) returned %d keys, want 7: %q", len(got), got)
	}
	for i, k := range got {
		if !ValidOrder(k) {
			t.Errorf("Spread key %d = %q is not a valid order key", i, k)
		}
		if i > 0 && got[i-1] >= k {
			t.Errorf("Spread keys are not ascending at %d: %q then %q", i, got[i-1], k)
		}
	}

	inner, err := Spread("V", "W", 3)
	if err != nil {
		t.Fatalf("Spread(\"V\",\"W\",3): %v", err)
	}
	for i, k := range inner {
		if k <= "V" || k >= "W" {
			t.Errorf("Spread key %d = %q is not strictly between V and W", i, k)
		}
	}

	empty, err := Spread("V", "W", 0)
	if err != nil {
		t.Fatalf("Spread(\"V\",\"W\",0): %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("Spread(\"V\",\"W\",0) = %q, want no keys", empty)
	}

	if _, err := Spread("W", "V", 1); !errors.Is(err, ErrOrder) {
		t.Errorf("Spread with inverted bounds err = %v, want ErrOrder", err)
	}
	if _, err := Spread("V", "W", -1); !errors.Is(err, ErrOrder) {
		t.Errorf("Spread with a negative count err = %v, want ErrOrder", err)
	}
}

func TestValidOrder(t *testing.T) {
	for _, k := range []string{"V", "0V", "zzzV", "0123456789", "Az"} {
		if !ValidOrder(k) {
			t.Errorf("ValidOrder(%q) = false, want true", k)
		}
	}
	for _, k := range []string{"", "0", "V0", "V!", "V-", " V", "V\n"} {
		if ValidOrder(k) {
			t.Errorf("ValidOrder(%q) = true, want false", k)
		}
	}
}
