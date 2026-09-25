package main

import (
	"os"
	"path/filepath"
	"testing"
)

// fixture reads testdata/licenses/<name>: each is a licence's text as a
// module ships it (the provenance of each is in testdata/licenses/README.md).
func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "licenses", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// moduleDir makes a directory holding files, name -> content.
func moduleDir(t *testing.T, files map[string][]byte) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestClassify(t *testing.T) {
	cat := func(names ...string) []byte {
		var b []byte
		for _, n := range names {
			b = append(append(b, fixture(t, n)...), '\n')
		}
		return b
	}
	cases := []struct {
		name       string
		files      map[string][]byte
		wantExpr   string
		wantSource string
	}{
		{"MIT", map[string][]byte{"LICENSE": fixture(t, "MIT.txt")}, "MIT", "LICENSE"},
		{"Apache", map[string][]byte{"LICENSE": fixture(t, "Apache-2.0.txt")}, "Apache-2.0", "LICENSE"},
		{"BSD-2", map[string][]byte{"LICENSE": fixture(t, "BSD-2-Clause.txt")}, "BSD-2-Clause", "LICENSE"},
		{"BSD-3", map[string][]byte{"LICENSE": fixture(t, "BSD-3-Clause.txt")}, "BSD-3-Clause", "LICENSE"},
		{"ISC", map[string][]byte{"LICENSE": fixture(t, "ISC.txt")}, "ISC", "LICENSE"},
		{"0BSD", map[string][]byte{"LICENSE": fixture(t, "0BSD.txt")}, "0BSD", "LICENSE"},
		{"GPL-3.0", map[string][]byte{"COPYING": fixture(t, "GPL-3.0.txt")}, "GPL-3.0-only", "COPYING"},
		// MPL-2.0 names the GNU licences as its "Secondary Licenses": only MPL.
		{"MPL-2.0", map[string][]byte{"LICENSE": fixture(t, "MPL-2.0.txt")}, "MPL-2.0", "LICENSE"},
		{"Apache + CC-BY section", map[string][]byte{"LICENSE": cat("Apache-2.0.txt", "CC-BY-4.0-section.txt")},
			"Apache-2.0 AND LicenseRef-unclassified", "LICENSE"},
		{"dual MIT/Apache files", map[string][]byte{"LICENSE-MIT": fixture(t, "MIT.txt"), "LICENSE-APACHE": fixture(t, "Apache-2.0.txt")},
			"MIT OR Apache-2.0", "LICENSE-APACHE, LICENSE-MIT"},
		// github.com/josharian/intern v1.0.0 holds only "license.md" (RR-I9-3).
		{"lower-case license.md", map[string][]byte{"license.md": fixture(t, "MIT.txt")}, "MIT", "license.md"},
		{"two files are their AND", map[string][]byte{"LICENSE": fixture(t, "MIT.txt"), "COPYING": fixture(t, "MPL-2.0.txt")},
			"MPL-2.0 AND MIT", "COPYING, LICENSE"},
		{"the same licence twice is one", map[string][]byte{"LICENSE": fixture(t, "MIT.txt"), "LICENSE.txt": fixture(t, "MIT.txt")},
			"MIT", "LICENSE, LICENSE.txt"},
		{"a file that matches nothing", map[string][]byte{"LICENCE": []byte("All rights reserved.\n")},
			"LicenseRef-unclassified", "LICENCE"},
		// A dual pair only ORs when each file holds the licence its name says.
		{"a misnamed dual pair is an AND", map[string][]byte{"LICENSE-MIT": fixture(t, "GPL-3.0.txt"), "LICENSE-APACHE": fixture(t, "Apache-2.0.txt")},
			"Apache-2.0 AND GPL-3.0-only", "LICENSE-APACHE, LICENSE-MIT"},
		{"a README is not a licence file", map[string][]byte{"README.md": fixture(t, "MIT.txt"), "LICENSE": fixture(t, "ISC.txt")},
			"ISC", "LICENSE"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			expr, source, err := Classify(moduleDir(t, c.files))
			if err != nil {
				t.Fatal(err)
			}
			if expr != c.wantExpr || source != c.wantSource {
				t.Fatalf("Classify = (%q, %q), want (%q, %q)", expr, source, c.wantExpr, c.wantSource)
			}
			if _, err := ParseSPDX(expr); err != nil {
				t.Fatalf("Classify's %q does not parse: %v", expr, err)
			}
		})
	}
}

// Hole 3: a module with no licence file is ("", "none", nil): it fails the
// gate, and it is not an error, because the gate ran.
func TestClassify_noLicenceFile(t *testing.T) {
	expr, source, err := Classify(moduleDir(t, map[string][]byte{"go.mod": []byte("module x\n")}))
	if err != nil || expr != "" || source != "none" {
		t.Fatalf("Classify(empty module) = (%q, %q, %v), want (\"\", \"none\", nil)", expr, source, err)
	}
	if _, _, err := Classify(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("Classify of a missing directory returned no error")
	}
}
