package product

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func swarmGitFixture(t *testing.T) (*Server, string, string, string) {
	t.Helper()
	s, _ := journeyFixture(t)
	target, worker := filepath.Join(s.projectRoot(), "integration"), filepath.Join(s.projectRoot(), "task")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	confinedFixtureGit(t, target, "init", "--initial-branch=main")
	base := commitConfinedFixture(t, target, "base")
	confinedFixtureGit(t, s.projectRoot(), "clone", "--no-local", target, worker)
	return s, target, worker, base
}

func swarmFetchArguments(source, commit string) []string {
	return []string{"-c", "protocol.file.allow=always", "fetch", "--no-tags", "--no-write-fetch-head", "--", source, commit}
}

func TestSwarmGitTransfersOwnedObjectsAndMergesBuiltInChanges(t *testing.T) {
	s, target, worker, _ := swarmGitFixture(t)
	commit := commitConfinedFixture(t, worker, "reviewed change")
	ctx := context.Background()
	if _, err := s.swarmGit(ctx, target, swarmFetchArguments(worker, commit)...); err != nil {
		t.Fatal(err)
	}
	if _, err := s.swarmGit(ctx, target, "merge", "--ff-only", commit); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(target, "README.md")); err != nil || string(got) != "reviewed change" {
		t.Fatal("reviewed objects did not reach the integration checkout", err)
	}
}

func TestSwarmGitRefusesConfiguredFetchExecutionBeforeImport(t *testing.T) {
	s, target, worker, _ := swarmGitFixture(t)
	commit := commitConfinedFixture(t, worker, "worker candidate")
	marker := filepath.Join(s.projectRoot(), "host-execution-marker")
	helper := filepath.Join(s.projectRoot(), "upload-probe.sh")
	body := "#!/bin/sh\nprintf invoked > '" + filepath.ToSlash(marker) + "'\nexec git-upload-pack \"$1\"\n"
	if err := os.WriteFile(helper, []byte(body), 0700); err != nil {
		t.Fatal(err)
	}
	confinedFixtureGit(t, target, "config", "protocol.ext.allow", "always")
	confinedFixtureGit(t, target, "config", "url.ext::"+filepath.ToSlash(helper)+" "+filepath.ToSlash(worker)+".insteadOf", filepath.ToSlash(worker))
	ctx := context.Background()
	// Establish that the prior controller flags do execute this benign host
	// marker; Windows still runs the refusal below without relying on a Unix
	// shebang to prove an external process launch.
	if runtime.GOOS != "windows" {
		if _, err := s.journeyGit(ctx, target, swarmFetchArguments(worker, commit)...); err != nil {
			t.Fatal("native adversarial fixture did not execute", err)
		}
		if _, err := os.Stat(marker); err != nil {
			t.Fatal("original fetch did not demonstrate the host-execution boundary", err)
		}
		if err := os.Remove(marker); err != nil {
			t.Fatal(err)
		}
		// A fresh object prevents Git from skipping transport because it already
		// possesses the first requested commit.
		commit = commitConfinedFixture(t, worker, "second worker candidate")
	}
	before, _ := os.ReadFile(filepath.Join(target, ".git", "config"))
	if _, err := s.swarmGit(ctx, target, swarmFetchArguments(worker, commit)...); err == nil {
		t.Fatal("controller accepted a repository-selected external transport")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("refused fetch still executed a host command", err)
	}
	after, _ := os.ReadFile(filepath.Join(target, ".git", "config"))
	if string(before) != string(after) {
		t.Fatal("refusal rewrote native repository configuration")
	}
}

func TestSwarmGitRefusesCustomMergeDriver(t *testing.T) {
	s, target, worker, _ := swarmGitFixture(t)
	if err := os.WriteFile(filepath.Join(target, ".gitattributes"), []byte("README.md merge=controllerProbe\n"), 0600); err != nil {
		t.Fatal(err)
	}
	confinedFixtureGit(t, target, "add", ".gitattributes")
	base := commitConfinedFixture(t, target, "common base")
	confinedFixtureGit(t, worker, "fetch", "--", target, base)
	confinedFixtureGit(t, worker, "reset", "--hard", base)
	commitConfinedFixture(t, target, "target edit")
	candidate := commitConfinedFixture(t, worker, "worker edit")
	marker := filepath.Join(s.projectRoot(), "merge-driver-marker")
	confinedFixtureGit(t, target, "config", "merge.controllerProbe.driver", "printf invoked > '"+filepath.ToSlash(marker)+"'; cp %B %A")
	ctx := context.Background()
	if _, err := s.swarmGit(ctx, target, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "merge", "--no-ff", "--no-edit", candidate); err == nil {
		t.Fatal("swarm merge accepted a repository-selected command")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("refused merge invoked its configured driver", err)
	}
	// The equivalent pre-hardening command proves the tracked attribute really
	// invokes the configured driver, rather than an inert config-only fixture.
	if _, err := s.journeyGit(ctx, target, swarmFetchArguments(worker, candidate)...); err != nil {
		t.Fatal(err)
	}
	if _, err := s.journeyGit(ctx, target, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "merge", "--no-ff", "--no-edit", candidate); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("native merge driver fixture did not demonstrate host execution", err)
	}
}

func TestSwarmGitRefusesSourceHelpersAndPartialClone(t *testing.T) {
	for _, key := range []string{"uploadpack.packObjectsHook", "remote.origin.uploadpack", "remote.origin.vcs", "remote.origin.promisor", "extensions.partialClone", "gpg.program", "gpg.ssh.program", "core.alternateRefsCommand"} {
		t.Run(key, func(t *testing.T) {
			s, target, worker, _ := swarmGitFixture(t)
			candidate := commitConfinedFixture(t, worker, "candidate")
			confinedFixtureGit(t, worker, "config", key, "true")
			if _, err := s.swarmGit(context.Background(), target, swarmFetchArguments(worker, candidate)...); err == nil {
				t.Fatal("worker-controlled server/helper config crossed local transfer boundary")
			}
		})
	}
}

func TestSwarmGitRejectsLooseAndPackedReplacementHistory(t *testing.T) {
	for _, packed := range []bool{false, true} {
		name := "loose"
		if packed {
			name = "packed"
		}
		t.Run(name, func(t *testing.T) {
			s, target, worker, base := swarmGitFixture(t)
			actual := commitConfinedFixture(t, worker, "actual unseen content")
			confinedFixtureGit(t, worker, "reset", "--hard", base)
			shown := commitConfinedFixture(t, worker, "benign shown content")
			confinedFixtureGit(t, worker, "reset", "--hard", actual)
			confinedFixtureGit(t, worker, "replace", actual, shown)
			confinedFixtureGit(t, worker, "reset", "--hard", "HEAD")
			if packed {
				confinedFixtureGit(t, worker, "pack-refs", "--all")
			}
			if status := confinedFixtureGit(t, worker, "status", "--porcelain"); status != "" {
				t.Fatal("replacement fixture must appear clean")
			}
			diff := confinedFixtureGit(t, worker, "diff", base, actual)
			if !strings.Contains(diff, "+benign shown content") || strings.Contains(diff, "+actual unseen content") {
				t.Fatal("fixture did not disguise the fetched commit")
			}
			if err := s.validateSwarmGit(context.Background(), worker); err == nil {
				t.Fatal("replacement history was accepted as an exact reviewed commit")
			}
			if _, err := s.swarmGit(context.Background(), target, swarmFetchArguments(worker, actual)...); err == nil {
				t.Fatal("fetch imported a commit hidden by worker replacement metadata")
			}
		})
	}
}

func TestSwarmGitRejectsGraftsAndUnownedObjectSources(t *testing.T) {
	s, target, worker, base := swarmGitFixture(t)
	if err := os.WriteFile(filepath.Join(worker, ".git", "info", "grafts"), []byte(base+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.validateSwarmGit(context.Background(), worker); err == nil {
		t.Fatal("grafted history was admitted")
	}
	for _, source := range []string{"../task", "ssh://example.invalid/project", t.TempDir()} {
		if _, err := s.swarmGit(context.Background(), target, swarmFetchArguments(source, base)...); err == nil {
			t.Fatal("non-owned/non-local object source was accepted")
		}
	}
}

func TestSwarmGitRefusesIncludedCommandsIncludingUTF8BOM(t *testing.T) {
	for _, bom := range []bool{false, true} {
		name := "ordinary"
		if bom {
			name = "utf8-bom"
		}
		t.Run(name, func(t *testing.T) {
			s, target, worker, _ := swarmGitFixture(t)
			candidate := commitConfinedFixture(t, worker, "candidate")
			outside := filepath.Join(t.TempDir(), "external-config")
			if err := os.WriteFile(outside, []byte("[merge \"probe\"]\n driver = printf untrusted\n"), 0600); err != nil {
				t.Fatal(err)
			}
			config := filepath.Join(worker, ".git", "config")
			original, err := os.ReadFile(config)
			if err != nil {
				t.Fatal(err)
			}
			prefix := "[include]\n path = " + filepath.ToSlash(outside) + "\n"
			if bom {
				prefix = "\xef\xbb\xbf" + prefix
			}
			if err := os.WriteFile(config, append([]byte(prefix), original...), 0600); err != nil {
				t.Fatal(err)
			}
			if actual := confinedFixtureGit(t, worker, "config", "--get", "merge.probe.driver"); actual != "printf untrusted" {
				t.Fatal("native config fixture did not follow the external include")
			}
			// Make external reads observable even without executing a command:
			// an effective-config probe would fail on these invalid bytes. The
			// include refusal must happen before reading or returning that error.
			if err := os.WriteFile(outside, []byte("EXTERNAL_INCLUDE_WAS_READ\n"), 0600); err != nil {
				t.Fatal(err)
			}
			_, err = s.swarmGit(context.Background(), target, swarmFetchArguments(worker, candidate)...)
			if err == nil || !(strings.Contains(err.Error(), "repository Git configuration cannot include host files") || strings.HasPrefix(err.Error(), "This repository configures a Git include")) || strings.Contains(err.Error(), "EXTERNAL_INCLUDE_WAS_READ") {
				t.Fatal("external included config was followed before refusal", err)
			}
		})
	}
}
