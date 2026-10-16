package product

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"
)

// These tests need real Windows Git and the existing registered fixture owner.
// Future execution must select a short owned TMP root; there is no drive/junction
// fallback or skip that would turn an infeasible fixture into a passing result.
func bootstrapWindowsSource(t *testing.T) (*Server, string, string) {
	t.Helper()
	s, _ := journeyFixture(t)
	source := filepath.Join(s.projectRoot(), "source")
	if err := os.Mkdir(source, 0700); err != nil {
		t.Fatal(err)
	}
	confinedFixtureGit(t, source, "init", "--initial-branch=main")
	confinedFixtureGit(t, source, "config", "--local", "core.longpaths", "true")
	return s, source, commitConfinedFixture(t, source, "bootstrap fixture")
}

func TestSwarmCheckoutWindowsLocalLongpathsBeforeFetch(t *testing.T) {
	s, source, _ := bootstrapWindowsSource(t)
	destination := filepath.Join(s.projectRoot(), "task")
	if err := swarmBootstrapFeasibility(destination); err != nil {
		t.Fatalf("this real Windows fixture requires an explicitly short owned TMP root: %v", err)
	}
	relative := strings.Repeat("nested-segment-12345/", 12) + "result.txt"
	file := filepath.Join(source, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
		t.Fatal(err)
	}
	const contents = "exact long tracked file from the reviewed source\n"
	commit := swarmCommitFixture(t, source, relative, contents)
	if len(utf16.Encode([]rune(filepath.Join(destination, filepath.FromSlash(relative))))) <= 260 {
		t.Fatal("fixture did not exercise long tracked content")
	}
	if err := s.createSwarmCheckout(context.Background(), source, destination, commit); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(confinedFixtureGit(t, destination, "config", "--local", "--type=bool", "--get", "core.longpaths")); got != "true" {
		t.Fatal("isolated repository did not retain its own longpaths setting", got)
	}
	if got := strings.TrimSpace(confinedFixtureGit(t, destination, "rev-parse", "HEAD")); got != commit {
		t.Fatal("isolated checkout lost exact source commit", got, commit)
	}
	if data, err := os.ReadFile(filepath.Join(destination, filepath.FromSlash(relative))); err != nil || string(data) != contents {
		t.Fatal("long tracked source bytes were not checked out", err)
	}
	if _, err := os.Lstat(filepath.Join(destination, ".git", "objects", "info", "alternates")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("isolated repository borrowed source metadata", err)
	}
}

func TestSwarmCheckoutWindowsLocalLongpathsFailureStopsBeforeFetch(t *testing.T) {
	s, source, commit := bootstrapWindowsSource(t)
	for _, failure := range []string{"write-error", "read-error", "false", "missing", "malformed"} {
		t.Run(failure, func(t *testing.T) {
			destination := filepath.Join(s.projectRoot(), "refused-"+failure)
			if err := swarmBootstrapFeasibility(destination); err != nil {
				t.Fatalf("short owned fixture root required: %v", err)
			}
			original := s.runner
			defer func() { s.runner = original }()
			writes, reads, transfers := 0, 0, 0
			s.runner = func(ctx context.Context, executable string, args []string, dir string) (string, error) {
				if dir == destination {
					joined := strings.Join(args, "\x00")
					if strings.HasSuffix(joined, "config\x00--local\x00core.longpaths\x00true") {
						writes++
						if failure == "write-error" {
							return "", errors.New("controlled local configuration failure")
						}
					}
					if strings.HasSuffix(joined, "config\x00--local\x00--type=bool\x00--get\x00core.longpaths") {
						reads++
						switch failure {
						case "read-error":
							return "", errors.New("controlled local readback failure")
						case "false":
							return "false\n", nil
						case "missing":
							return "", nil
						case "malformed":
							return "true\nfalse\n", nil
						}
					}
					for _, arg := range args {
						if arg == "fetch" || arg == "checkout" {
							transfers++
						}
					}
				}
				return original(ctx, executable, args, dir)
			}
			if err := s.createSwarmCheckout(context.Background(), source, destination, commit); err == nil {
				t.Fatal("unconfirmed local setting allowed checkout")
			}
			if writes != 1 || transfers != 0 || failure == "write-error" && reads != 0 || failure != "write-error" && reads != 1 {
				t.Fatal("fixed configuration failure did not stop before source transfer", writes, reads, transfers)
			}
			if info, err := os.Lstat(filepath.Join(destination, ".git")); err != nil || !info.IsDir() {
				t.Fatal("partial owned repository was not retained", err)
			}
		})
	}
}

func TestSwarmCheckoutWindowsBootstrapRefusalBeforeMkdir(t *testing.T) {
	s, _ := journeyFixture(t)
	before := auditTree(t, s.projectRoot())
	s.runner = func(context.Context, string, []string, string) (string, error) {
		t.Fatal("deterministic bootstrap refusal invoked Git")
		return "", errors.New("unexpected Git call")
	}
	destination := filepath.Join(s.projectRoot(), strings.Repeat("long-root-", 30), "task")
	if err := s.createSwarmCheckout(context.Background(), "unused-source", destination, strings.Repeat("a", 40)); !errors.Is(err, errSwarmBootstrapPath) {
		t.Fatal("infeasible bootstrap was not refused before allocation", err)
	}
	if !reflect.DeepEqual(before, auditTree(t, s.projectRoot())) {
		t.Fatal("bootstrap refusal created a directory")
	}
}
