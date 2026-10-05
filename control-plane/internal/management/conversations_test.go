package management

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/conversationpath"
)

const migratedConversation = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

func migrationFixture(t *testing.T) (string, string, string) {
	t.Helper()
	gateway := filepath.Join(t.TempDir(), "control-plane")
	worker := filepath.Join(gateway, "users", "user-fixture", "runtime")
	source := filepath.Join(worker, "state", "conversations")
	if err := os.MkdirAll(source, 0700); err != nil {
		t.Fatal(err)
	}
	return gateway, worker, source
}

func migrationWrite(t *testing.T, root, name, value string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0600); err != nil {
		t.Fatal(err)
	}
}

func migrationRead(t *testing.T, root, name, want string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, name))
	if err != nil || string(data) != want {
		t.Fatalf("migration did not preserve %s", name)
	}
}

func TestConversationMigrationPreservesIdentityAndNeverReimportsRetiredCredentials(t *testing.T) {
	gateway, worker, source := migrationFixture(t)
	oldNative := conversationpath.LegacyRelative("local", "setup-codex", "codex", migratedConversation)
	newNative := conversationpath.NativeRelative("local", "setup-codex", "codex", migratedConversation)
	for _, item := range []struct{ path, value string }{
		{filepath.Join(oldNative, "auth.json"), "old-owned-fixture-credential"},
		{filepath.Join(oldNative, "sessions", "2026", "10", "05", "rollout.jsonl"), "upstream-transcript"},
		{filepath.Join(oldNative, "state.sqlite-wal"), "upstream-state-wal"},
		{filepath.Join("local", "w-12345678", "session.jsonl"), "claude-transcript"},
		{filepath.Join("assistance", "codex-account.json"), `{"sessionId":"s-fixture","connectionId":"connection-fixture"}`},
	} {
		migrationWrite(t, source, item.path, item.value)
	}
	migrationWrite(t, worker, "state/sessions/sessions.json", `{"schemaVersion":1,"sessions":[{"status":"exited"}]}`)
	target, err := migrateWorkerConversations(gateway, worker, "user-fixture")
	if err != nil {
		t.Fatal(err)
	}
	if target != conversationpath.UserRoot(gateway, "user-fixture") || len(target) >= len(source) {
		t.Fatal("worker did not receive its compact account root")
	}
	migrationRead(t, target, filepath.Join(newNative, "auth.json"), "old-owned-fixture-credential")
	migrationRead(t, target, filepath.Join(newNative, "sessions", "2026", "10", "05", "rollout.jsonl"), "upstream-transcript")
	migrationRead(t, target, filepath.Join(newNative, "state.sqlite-wal"), "upstream-state-wal")
	migrationRead(t, target, "local/w-12345678/session.jsonl", "claude-transcript")
	migrationRead(t, target, "assistance/codex-account.json", `{"sessionId":"s-fixture","connectionId":"connection-fixture"}`)
	migrationRead(t, source, filepath.Join(oldNative, "auth.json"), "old-owned-fixture-credential")
	migrationWrite(t, target, filepath.Join(newNative, "auth.json"), "rotated-fixture-credential")
	if _, err = migrateWorkerConversations(gateway, worker, "user-fixture"); err != nil {
		t.Fatal(err)
	}
	migrationRead(t, target, filepath.Join(newNative, "auth.json"), "rotated-fixture-credential")
	other, err := migrateWorkerConversations(gateway, filepath.Join(gateway, "users", "other", "runtime"), "user-other")
	if err != nil || other == target {
		t.Fatal("accounts share private conversation storage")
	}
}

func TestConversationMigrationRefusesActiveAndUnreadableLegacyRegistries(t *testing.T) {
	for _, body := range []string{
		`{"schemaVersion":1,"sessions":[{"status":"starting"}]}`,
		`{"schemaVersion":1,"sessions":[{"status":"running"}]}`,
		`{"schemaVersion":1,"sessions":[{"status":"idle"}]}`,
		`{"schemaVersion":2,"sessions":[]}`,
		`not a registry`,
	} {
		t.Run(body, func(t *testing.T) {
			gateway, worker, _ := migrationFixture(t)
			migrationWrite(t, worker, "state/sessions/sessions.json", body)
			_, err := migrateWorkerConversations(gateway, worker, "user-fixture")
			if err == nil {
				t.Fatal("unverified active homes were copied")
			}
			if strings.Contains(body, `"status"`) && !errors.Is(err, errConversationsActive) {
				t.Fatal("active sessions did not get an actionable migration error")
			}
			if _, err := os.Stat(conversationpath.UserRoot(gateway, "user-fixture")); !os.IsNotExist(err) {
				t.Fatal("failed migration published a tree")
			}
		})
	}
}

func TestConversationMigrationRejectsUnboundTargetsAndLeavesLegacyIntact(t *testing.T) {
	for _, marker := range []string{"", `{"version":1,"userId":"other","source":"different"}`} {
		t.Run(marker, func(t *testing.T) {
			gateway, worker, source := migrationFixture(t)
			migrationWrite(t, source, "local/w-fixture/proof", "retained")
			target := conversationpath.UserRoot(gateway, "user-fixture")
			if err := os.MkdirAll(target, 0700); err != nil {
				t.Fatal(err)
			}
			if marker != "" {
				migrationWrite(t, target, conversationsMarker, marker)
			}
			if _, err := migrateWorkerConversations(gateway, worker, "user-fixture"); err == nil {
				t.Fatal("unbound private tree was adopted")
			}
			migrationRead(t, source, "local/w-fixture/proof", "retained")
		})
	}
}

func TestConversationMigrationRejectsHardLinksAndCleansOnlyItsFailedStagingTree(t *testing.T) {
	gateway, worker, source := migrationFixture(t)
	outside := filepath.Join(t.TempDir(), "private-fixture")
	if err := os.WriteFile(outside, []byte("never-copy"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(outside, filepath.Join(source, "linked-file")); err != nil {
		t.Skipf("hard links unavailable: %v", err)
	}
	if _, err := migrateWorkerConversations(gateway, worker, "user-fixture"); err == nil {
		t.Fatal("copied another file's credential identity")
	}
	entries, err := os.ReadDir(filepath.Join(gateway, "c"))
	if err != nil || len(entries) != 0 {
		t.Fatal("failed staging retained obsolete private data")
	}
	migrationRead(t, filepath.Dir(outside), filepath.Base(outside), "never-copy")
	if _, err := os.Stat(filepath.Join(source, "linked-file")); err != nil {
		t.Fatal("legacy link was modified")
	}
}

func TestConversationMigrationRejectsProviderContainerFiles(t *testing.T) {
	gateway, worker, source := migrationFixture(t)
	migrationWrite(t, source, "native/local/setup-codex/unexpected", "must not misbind")
	if _, err := migrateWorkerConversations(gateway, worker, "user-fixture"); err == nil {
		t.Fatal("unidentified provider file was migrated")
	}
	migrationRead(t, source, "native/local/setup-codex/unexpected", "must not misbind")
}
