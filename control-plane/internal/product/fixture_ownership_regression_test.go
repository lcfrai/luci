package product

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// This child runs only inside a disposable parent repository. If a regression
// reintroduces empty-cwd or upward Git discovery, it can damage only that
// fixture, and the parent detects every changed byte and HEAD/index change.
func TestSwarmFixtureRefusalProcess(t *testing.T) {
	mode := os.Getenv("LUCI_TEST_FIXTURE_REFUSAL")
	if mode == "" {
		t.Skip("subprocess fixture")
	}
	switch mode {
	case "empty":
		swarmCommitFixture(t, "", "alpha.txt", "must never be written\n")
	case "relative":
		swarmCommitFixture(t, ".", "alpha.txt", "must never be written\n")
	case "foreign":
		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		swarmCommitFixture(t, cwd, "alpha.txt", "must never be written\n")
	case "no-metadata":
		swarmCommitFixture(t, ownedFixtureTempDir(t), "alpha.txt", "must never be written\n")
	case "git-ceiling":
		confinedFixtureGit(t, ownedFixtureTempDir(t), "add", "--all")
	case "git-ceiling-child":
		child := filepath.Join(ownedFixtureTempDir(t), "child")
		if err := os.Mkdir(child, 0700); err != nil {
			t.Fatal(err)
		}
		confinedFixtureGit(t, child, "add", "--all")
	default:
		t.Fatal("unknown fixture refusal case")
	}
	t.Fatal("unsafe fixture operation unexpectedly returned")
}

func TestSwarmFixtureRefusesUnsafePathsWithoutMutatingParentRepository(t *testing.T) {
	parent := ownedFixtureTempDir(t)
	confinedFixtureGit(t, parent, "init", "--initial-branch=main")
	commitConfinedFixture(t, parent, "disposable parent repository")
	tmp := filepath.Join(parent, "nested-temporary-root")
	if err := os.Mkdir(tmp, 0700); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"empty", "relative", "foreign", "no-metadata", "git-ceiling", "git-ceiling-child"} {
		t.Run(mode, func(t *testing.T) {
			before := auditTree(t, parent)
			cmd := exec.Command(executable, "-test.run=^TestSwarmFixtureRefusalProcess$", "-test.v")
			cmd.Dir = parent
			cmd.Env = append(os.Environ(), "LUCI_TEST_FIXTURE_REFUSAL="+mode, "TMPDIR="+tmp, "TMP="+tmp, "TEMP="+tmp)
			output, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatal("unsafe fixture helper did not stop the subprocess", string(output))
			}
			want := "refuse unsafe swarm fixture commit"
			if strings.HasPrefix(mode, "git-ceiling") {
				want = "not a git repository"
			}
			if !strings.Contains(string(output), want) {
				t.Fatalf("wrong refusal: %v\n%s", err, output)
			}
			if !reflect.DeepEqual(before, auditTree(t, parent)) {
				t.Fatal("refused fixture operation changed disposable parent files or Git metadata")
			}
		})
	}
}

func TestSwarmFixtureOwnershipRejectsUnregisteredAndEscapingPaths(t *testing.T) {
	owned := ownedFixtureTempDir(t)
	foreign := t.TempDir()
	for _, path := range []string{"", ".", foreign, filepath.Dir(owned), filepath.Join(owned, "missing")} {
		if err := validateOwnedFixtureDirectory(t, path); err == nil {
			t.Fatalf("accepted unowned fixture path %q", path)
		}
	}
	if err := validateOwnedFixtureDirectory(t, owned); err != nil {
		t.Fatal("fresh registered directory not recognized", err)
	}
	if err := validateOwnedFixtureWorktree(t, owned); err == nil {
		t.Fatal("commit accepted a directory without its own repository")
	}
	t.Run("parent-owned fixture", func(t *testing.T) {
		if err := validateOwnedFixtureDirectory(t, owned); err != nil {
			t.Fatal("subtest could not use its parent's retained fixture", err)
		}
	})
}
