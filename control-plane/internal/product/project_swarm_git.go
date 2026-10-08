package product

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Swarm workers own their Git metadata inside Docker. Importing their results
// must not turn repository configuration into controller execution, a network
// transport, or a replacement for the commit the operator actually reviewed.
// This policy is deliberately scoped to controller swarm operations; ordinary
// explicit remote Git account/publishing workflows retain their own policy.
func (s *Server) validateSwarmGit(ctx context.Context, dir string) error {
	bounded, err := journeyGitBoundary(ctx, s.projectRoot(), dir, []string{"status"})
	if err != nil {
		return err
	}
	gitDir := strings.TrimPrefix(bounded[0], "--git-dir=")
	common := gitDir
	if raw, err := os.ReadFile(filepath.Join(gitDir, "commondir")); err == nil {
		common = strings.TrimSpace(string(raw))
		if !filepath.IsAbs(common) {
			common = filepath.Join(gitDir, common)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if _, err := os.Lstat(filepath.Join(common, "info", "grafts")); !errors.Is(err, os.ErrNotExist) {
		return errors.New("Git graft metadata changes commit interpretation. Remove it and review the actual task history before using swarm integration.")
	}
	// Probe without following includes before running any command that reads
	// the effective configuration. Parsed keys also cover config spellings
	// (such as a UTF-8 BOM) beyond the shared boundary's textual include guard.
	config, err := s.journeyGit(ctx, dir, "config", "--null", "--list", "--no-includes")
	if err != nil {
		return err
	}
	if len(config) >= 2<<20 {
		return errors.New("The repository configuration exceeds the complete swarm review limit.")
	}
	for _, entry := range strings.Split(config, "\x00") {
		if entry == "" {
			continue
		}
		key, _, _ := strings.Cut(entry, "\n")
		key = strings.ToLower(key)
		if unsafeSwarmGitSetting(key) {
			// Do not include configuration values: helpers and URLs may embed
			// credentials or arbitrary terminal text.
			return errors.New("This repository configures a Git include, URL rewrite, external helper, custom merge driver, signature command or partial clone. Swarm integration requires ordinary local Git objects and built-in merging; preserve the work and remove that repository configuration before reviewing again.")
		}
	}
	// for-each-ref covers packed as well as loose refs. Merely inspecting a
	// refs/replace directory would miss a packed replacement attack.
	replacements, err := s.journeyGit(ctx, dir, "for-each-ref", "--format=%(refname)", "refs/replace/")
	if err != nil {
		return err
	}
	if strings.TrimSpace(replacements) != "" {
		return errors.New("Git replacement refs can hide the actual committed changes. Remove them and review the real task commits before using swarm integration.")
	}
	return nil
}

func unsafeSwarmGitSetting(key string) bool {
	return key == "include.path" || strings.HasPrefix(key, "includeif.") && strings.HasSuffix(key, ".path") ||
		strings.HasPrefix(key, "url.") && (strings.HasSuffix(key, ".insteadof") || strings.HasSuffix(key, ".pushinsteadof")) ||
		strings.HasPrefix(key, "merge.") && strings.HasSuffix(key, ".driver") ||
		strings.HasPrefix(key, "remote.") && (strings.HasSuffix(key, ".uploadpack") || strings.HasSuffix(key, ".receivepack") || strings.HasSuffix(key, ".vcs") || strings.HasSuffix(key, ".promisor")) ||
		strings.HasPrefix(key, "gpg.") && strings.HasSuffix(key, ".program") ||
		key == "uploadpack.packobjectshook" || key == "core.alternaterefscommand" || key == "extensions.partialclone"
}

func (s *Server) swarmGit(ctx context.Context, dir string, args ...string) (string, error) {
	settings, command := []string{}, args
	for len(command) >= 2 && command[0] == "-c" {
		settings = append(settings, command[:2]...)
		command = command[2:]
	}
	if len(command) == 0 || strings.HasPrefix(command[0], "-") {
		return "", errors.New("Swarm Git requires a controller-selected subcommand.")
	}
	_, gitErr := os.Lstat(filepath.Join(dir, ".git"))
	if !(command[0] == "init" && errors.Is(gitErr, os.ErrNotExist)) {
		if err := s.validateSwarmGit(ctx, dir); err != nil {
			return "", err
		}
	}
	if command[0] == "fetch" {
		// Only a full commit from a confined local owned repository is admitted.
		// The worker source is a separate Git process during file transport; its
		// configuration needs its own validation, not just destination checks.
		if len(command) != 6 || command[1] != "--no-tags" || command[2] != "--no-write-fetch-head" || command[3] != "--" || !filepath.IsAbs(command[4]) || !deliverySHA.MatchString(command[5]) {
			return "", errors.New("Swarm object transfer requires an exact commit from an owned local checkout.")
		}
		if err := s.validateSwarmGit(ctx, command[4]); err != nil {
			return "", err
		}
	}
	settings = append(settings,
		"-c", "protocol.allow=never", "-c", "protocol.file.allow=always", "-c", "protocol.ext.allow=never",
		"-c", "submodule.recurse=false", "-c", "fetch.recurseSubmodules=false",
		"-c", "merge.autoStash=false", "-c", "merge.verifySignatures=false")
	return s.journeyGit(ctx, dir, append(settings, command...)...)
}
