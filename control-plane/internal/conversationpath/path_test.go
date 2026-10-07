package conversationpath

import (
	"path/filepath"
	"testing"
)

func TestPrivatePhysicalNamesAreCompactAndRetainEveryLogicalIdentity(t *testing.T) {
	root := UserRoot("gateway", "account-a")
	if filepath.Dir(root) != filepath.Join("gateway", "c") || root == UserRoot("gateway", "account-b") {
		t.Fatal("account roots are not isolated")
	}
	base := NativeRelative("local", "worktree", "codex", "conversation")
	if len(filepath.Base(base)) != 22 || filepath.Dir(base) != "n" {
		t.Fatal("native home was not physically compact")
	}
	for _, identity := range [][4]string{
		{"remote", "worktree", "codex", "conversation"},
		{"local", "other", "codex", "conversation"},
		{"local", "worktree", "opencode", "conversation"},
		{"local", "worktree", "codex", "other"},
	} {
		if NativeRelative(identity[0], identity[1], identity[2], identity[3]) == base {
			t.Fatal("logical identities share a writable provider home")
		}
	}
}
