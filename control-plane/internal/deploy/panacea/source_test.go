package panacea

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// keyGate is task 05's acceptance grep (task-05.md:380-381), byte for byte:
//
//	grep -rnE '["`]pnc_["`]|pnc_[0-9a-fA-F]{8}' control-plane/internal --include=*.go | grep -v _test.go
//
// It must print exactly one line once task 05 lands: deploy_connection.go's prefix check. So no
// non-test file in this package may match it: a quoted prefix is a second place that names the
// prefix as a literal, and eight hex digits after it are a real key pasted into source.
var keyGate = regexp.MustCompile("[\"`]pnc_[\"`]|pnc_[0-9a-fA-F]{8}")

func TestNoKeyPrefixLiteralInSource(t *testing.T) {
	var scanned int
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		scanned++
		for i, line := range strings.Split(string(b), "\n") {
			if keyGate.MatchString(line) {
				t.Errorf("%s:%d matches task 05's key gate; it would be a second line in that grep: %s",
					filepath.ToSlash(path), i+1, strings.TrimSpace(line))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// client.go, errors.go and cmd/versioncheck/main.go at least: a walk that saw nothing proves nothing.
	if scanned < 3 {
		t.Fatalf("scanned %d non-test .go files under the package; want at least 3", scanned)
	}
}
