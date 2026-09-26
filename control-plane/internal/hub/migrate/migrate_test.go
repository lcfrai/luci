package migrate

import (
	"crypto/sha256"
	"errors"
	"testing"
	"testing/fstest"
)

func file(s string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(s)} }

// TestLoadOrdersAndHashes: versions come from the number in the name, and each
// hash is of the bytes exactly as they sit in the FS (no trim, no CRLF fix).
func TestLoadOrdersAndHashes(t *testing.T) {
	b := "  CREATE TABLE b (x int);\r\n\n"
	a := "CREATE TABLE a (x int);\n"
	fsys := fstest.MapFS{
		"0002_b.sql": file(b),
		"0001_a.sql": file(a),
		"README.md":  file("not a migration"),
	}
	ms, err := Load(fsys)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(ms) != 2 || ms[0].Version != 1 || ms[1].Version != 2 {
		t.Fatalf("versions = %v, want [1 2]", versions(ms))
	}
	if ms[0].Name != "a" || ms[1].Name != "b" {
		t.Fatalf("names = %q %q, want a b", ms[0].Name, ms[1].Name)
	}
	if ms[0].SQL != a || ms[1].SQL != b {
		t.Fatalf("SQL not the file's bytes verbatim: %q %q", ms[0].SQL, ms[1].SQL)
	}
	if ms[0].SHA256 != sha256.Sum256([]byte(a)) {
		t.Fatalf("0001's SHA256 is not sha256 of its bytes")
	}
	if ms[1].SHA256 != sha256.Sum256([]byte(b)) {
		t.Fatalf("0002's SHA256 is not sha256 of its bytes (trimmed or normalised?)")
	}
}

// TestLoadRejectsBadNames: every malformed or gapped set is ErrBadName; what is
// not a root-level *.sql file is ignored, not refused.
func TestLoadRejectsBadNames(t *testing.T) {
	bad := map[string]fstest.MapFS{
		"too few digits": {"1_a.sql": file("x")},
		"five digits":    {"00001_a.sql": file("x")},
		"hyphen":         {"0001-a.sql": file("x")},
		"upper case":     {"0001_A.sql": file("x")},
		"empty name":     {"0001_.sql": file("x")},
		"two for one":    {"0001_a.sql": file("x"), "0001_b.sql": file("y")},
		"a gap":          {"0001_a.sql": file("x"), "0003_c.sql": file("z")},
		"zero":           {"0000_x.sql": file("x")},
		"starts at two":  {"0002_b.sql": file("x")},
	}
	for name, fsys := range bad {
		t.Run(name, func(t *testing.T) {
			ms, err := Load(fsys)
			if !errors.Is(err, ErrBadName) {
				t.Fatalf("Load = %v, %v; want ErrBadName", versions(ms), err)
			}
		})
	}
	ignored := map[string]struct {
		fsys fstest.MapFS
		want int
	}{
		"non-sql files":    {fstest.MapFS{"0001_a.sql": file("x"), "notes.txt": file("y"), "0002_b.sql.bak": file("z")}, 1},
		"upper-case .SQL":  {fstest.MapFS{"0001_a.SQL": file("x")}, 0},
		"sql in a sub dir": {fstest.MapFS{"sub/0003_c.sql": file("x"), "0001_a.sql": file("x"), "0002_b.sql": file("y")}, 2},
		"empty FS":         {fstest.MapFS{}, 0},
	}
	for name, c := range ignored {
		t.Run(name, func(t *testing.T) {
			ms, err := Load(c.fsys)
			if err != nil || len(ms) != c.want {
				t.Fatalf("Load = %v, %v; want %d migrations, no error", versions(ms), err, c.want)
			}
			for i, m := range ms {
				if m.Version != i+1 {
					t.Fatalf("versions = %v, want 1..%d", versions(ms), c.want)
				}
			}
		})
	}
}

func versions(ms []Migration) []int {
	out := make([]int, len(ms))
	for i, m := range ms {
		out[i] = m.Version
	}
	return out
}
