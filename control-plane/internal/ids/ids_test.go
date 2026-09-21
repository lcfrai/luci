package ids_test

import (
	"regexp"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/ids"
)

var (
	idPattern    = regexp.MustCompile(`^[a-z]-[0-9a-f]{8}$`)
	tokenPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)
)

func TestNewHasThePrefixAndEightHexCharacters(t *testing.T) {
	got := ids.New("s")
	if !idPattern.MatchString(got) {
		t.Fatalf("New(%q) = %q, want the shape s-3f2a91c0", "s", got)
	}
}

func TestNewKeepsWhicheverPrefixItIsGiven(t *testing.T) {
	for _, prefix := range []string{"s", "p", "b", "w", "m"} {
		got := ids.New(prefix)
		if got[:2] != prefix+"-" {
			t.Errorf("New(%q) = %q, want it to start %q", prefix, got, prefix+"-")
		}
	}
}

// docs/FUNCTIONAL-SPEC.md known defect 2: a home-rolled uid() collides on fast
// successive calls. 200 draws from 32 bits of crypto/rand collide with
// probability about 200*199/2/2^32, i.e. 1 in 216000 — rare enough that this
// test is not flaky. A time-seeded or counter-seeded generator collides inside
// 200 calls on Windows, where the clock granularity is ~15ms.
func TestNewDoesNotCollideOnFastSuccessiveCalls(t *testing.T) {
	const draws = 200
	seen := make(map[string]struct{}, draws)
	for i := 0; i < draws; i++ {
		id := ids.New("s")
		if _, dup := seen[id]; dup {
			t.Fatalf("New collided at draw %d: %q", i, id)
		}
		seen[id] = struct{}{}
	}
}

func TestTokenIsUrlSafeAnd43Characters(t *testing.T) {
	got := ids.Token()
	if !tokenPattern.MatchString(got) {
		t.Fatalf("Token() = %q, want 43 URL-safe base64 characters (256 bits)", got)
	}
}

func TestTokensDiffer(t *testing.T) {
	a, b := ids.Token(), ids.Token()
	if a == b {
		t.Fatalf("two Token() calls returned the same secret %q", a)
	}
}
