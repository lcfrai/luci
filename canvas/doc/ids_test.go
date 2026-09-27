package doc

import (
	"crypto/sha256"
	"regexp"
	"testing"
)

var (
	reCanvasID = regexp.MustCompile(`^cv-[0-9a-f]{16}$`)
	reNodeID   = regexp.MustCompile(`^n-[0-9a-f]{12}$`)
)

func TestIDs(t *testing.T) {
	seen := make(map[string]bool, 1000)
	for i := 0; i < 1000; i++ {
		id := NewCanvasID()
		if !reCanvasID.MatchString(id) {
			t.Fatalf("NewCanvasID() = %q, want ^cv-[0-9a-f]{16}$", id)
		}
		if seen[id] {
			t.Fatalf("NewCanvasID() repeated %q after %d draws", id, i)
		}
		seen[id] = true
		if !ValidCanvasID(id) {
			t.Fatalf("ValidCanvasID(%q) = false", id)
		}
	}

	for _, prefix := range []string{"pg", "n", "th", "msg", "rv"} {
		id := NewID(prefix)
		if !ValidID(prefix, id) {
			t.Errorf("ValidID(%q, %q) = false", prefix, id)
		}
		if prefix == "n" && !reNodeID.MatchString(id) {
			t.Errorf("NewID(\"n\") = %q, want ^n-[0-9a-f]{12}$", id)
		}
	}

	// The widths are not interchangeable: a canvas id is 16 hex, everything
	// else is 12, and neither validator accepts the other's width.
	if ValidCanvasID("cv-0000000000a1") {
		t.Error("ValidCanvasID accepted a 12-hex id")
	}
	if ValidID("n", "n-00000000000000a1") {
		t.Error("ValidID accepted a 16-hex id")
	}
	for _, bad := range []string{"", "n-0000000000A1", "n-0000000000g1", "N-0000000000a1", "n-0000000000a1 ", "n0000000000a1"} {
		if ValidID("n", bad) {
			t.Errorf("ValidID(\"n\", %q) = true, want false", bad)
		}
	}
	if ValidID("", "-0000000000a1") {
		t.Error("ValidID accepted an empty prefix")
	}
}

func TestAssetRef(t *testing.T) {
	sum := sha256.Sum256(nil)
	const want = "asset:sha256-e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	got := AssetRef(sum)
	if got != want {
		t.Fatalf("AssetRef(sha256(\"\")) = %q, want %q", got, want)
	}
	back, ok := ParseAssetRef(got)
	if !ok || back != sum {
		t.Fatalf("ParseAssetRef(%q) = %x, %v; want the same sum", got, back, ok)
	}
	bad := []string{
		"",
		"asset:sha256-E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B855", // uppercase hex
		"asset:sha256-e3b0c442",                     // short
		"sha256-" + want[len("asset:sha256-"):],     // no scheme
		"asset:sha1-" + want[len("asset:sha256-"):], // wrong hash
		want + "0", // long
		"asset:sha256-" + want[len("asset:sha256-"):] + " ",
	}
	for _, s := range bad {
		if _, ok := ParseAssetRef(s); ok {
			t.Errorf("ParseAssetRef(%q) = ok, want not ok", s)
		}
	}
	if !ValidAssetRef(want) {
		t.Errorf("ValidAssetRef(%q) = false", want)
	}
}
