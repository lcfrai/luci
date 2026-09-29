package project_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
)

// X-3 review 4's spelling attacks on I-B, lifted: git names the nested location
// in a spelling other than the projects directory's — the long form of the 8.3
// t.TempDir, or upper case — so a boundary found by NAME would never see the
// walk enter a mount. The leaf is a junction. The boundary is found by
// identity, and that must survive ConfineToMounts' walk (X-3 review 4 I-D).
func TestPathRefusesANestedJunctionUnderAnotherSpelling(t *testing.T) {
	for _, spell := range []string{"long", "upper"} {
		t.Run(spell, func(t *testing.T) {
			root := t.TempDir() // the 8.3 form on this host
			dir := makeRepo(t, root, "app")
			nested := filepath.Join(dir, ".worktrees", "feat")
			x3Register(t, dir, nested)
			longDir, err := filepath.EvalSymlinks(dir)
			if err != nil {
				t.Fatal(err)
			}
			x3ReparseJunction(t, outsideDir(t), nested)
			alt := filepath.Join(longDir, ".worktrees", "feat")
			if spell == "upper" {
				alt = strings.ToUpper(alt)
			}
			if alt == nested {
				t.Skipf("no other spelling on this host: %q", alt)
			}
			if _, err := os.Lstat(alt); err != nil {
				t.Skipf("this host does not answer to %q: %v", alt, err)
			}
			g := newFakeGit()
			g.reply(dir, worktreePorcelain(dir, alt), "worktree", "list", "--porcelain")

			wantPathRefused(t, project.NewScanner(root, g), alt, "spelled "+spell)
		})
	}
}
