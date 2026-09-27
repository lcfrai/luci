package doc

import (
	"errors"
	"testing"
)

func TestDesignKey_roundTrip(t *testing.T) {
	const canvasID = "cv-00000000000000a1"
	key, err := DesignKey(canvasID, 11)
	if err != nil {
		t.Fatalf("DesignKey(%q, 11): %v", canvasID, err)
	}
	if want := "design:cv-00000000000000a1@11"; key != want {
		t.Fatalf("DesignKey = %q, want %q", key, want)
	}
	gotID, gotRev, err := ParseDesignKey(key)
	if err != nil {
		t.Fatalf("ParseDesignKey(%q): %v", key, err)
	}
	if gotID != canvasID || gotRev != 11 {
		t.Fatalf("ParseDesignKey(%q) = %q, %d; want %q, 11", key, gotID, gotRev, canvasID)
	}

	// The bounds of the Hub's grammar, both ends inclusive.
	for _, rev := range []int{1, 999_999_999} {
		k, err := DesignKey(canvasID, rev)
		if err != nil {
			t.Fatalf("DesignKey(%q, %d): %v", canvasID, rev, err)
		}
		if _, back, err := ParseDesignKey(k); err != nil || back != rev {
			t.Fatalf("ParseDesignKey(%q) = %d, %v; want %d", k, back, err, rev)
		}
	}

	for _, c := range []struct {
		id  string
		rev int
	}{
		{canvasID, 0},
		{canvasID, -1},
		{canvasID, 1_000_000_000},
		{"cv-short", 11},
		{"cv-00000000000000A1", 11}, // uppercase hex
		{"", 11},
	} {
		if got, err := DesignKey(c.id, c.rev); !errors.Is(err, ErrBadKey) {
			t.Errorf("DesignKey(%q, %d) = %q, %v; want ErrBadKey", c.id, c.rev, got, err)
		}
	}
}

func TestParseDesignKey_rejects(t *testing.T) {
	bad := []string{
		"",
		"design:cv-00000000000000a1@011", // a leading zero is not the Hub's grammar
		"design:cv-00000000000000a1@0",
		"design:cv-00000000000000a1@",
		"design:cv-00000000000000a1",
		"design:cv-00000000000000a1@11@12",
		"design:cv-00000000000000a1@1000000000",
		"design:cv-00000000000000a1@+11",
		"design:cv-00000000000000a1@ 11",
		"design:cv-00000000000000a1@11 ",
		"design:cv-0000000000a1@11", // the control plane's 12-hex width
		"design:cv-00000000000000A1@11",
		"Design:cv-00000000000000a1@11",
		"design:@11",
		"cv-00000000000000a1@11",
		"design:cv-00000000000000a1@1e2",
	}
	for _, s := range bad {
		if id, rev, err := ParseDesignKey(s); !errors.Is(err, ErrBadKey) {
			t.Errorf("ParseDesignKey(%q) = %q, %d, %v; want ErrBadKey", s, id, rev, err)
		}
	}
}
