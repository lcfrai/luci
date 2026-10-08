package api

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/project"
	"github.com/lcfrai/luci/control-plane/internal/worktree"
)

func TestSwarmCheckoutCatalogueAndSessionShareOriginalProject(t *testing.T) {
	env, owner, primary, snapshot := sessionsPrimaryBranchFixture(t)
	root := filepath.Dir(owner.Path)
	checkout := filepath.Join(root, ".luci-swarms", owner.ID, "run-one", "task-one")
	if err := os.MkdirAll(checkout, 0700); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "--initial-branch=luci-task"}, {"-c", "protocol.file.allow=always", "fetch", "--no-tags", "--", owner.Path, snapshot.Commit}, {"checkout", "--detach", snapshot.Commit}} {
		if _, err := product.RunCommand(context.Background(), "git", args, checkout); err != nil {
			t.Fatal(err)
		}
	}
	row := project.Worktree{ID: project.WorktreeID(checkout), ProjectID: owner.ID, Name: "Task independent-api", Path: checkout}
	owned := &swarmWorktrees{base: env.api.deps.Projects, resolver: env.api.deps.Worktrees, owned: func(_ context.Context, id string) ([]project.Worktree, error) {
		if id != owner.ID {
			return nil, project.ErrUnknownProject
		}
		return []project.Worktree{row}, nil
	}}
	env.api.deps.Projects, env.api.deps.Worktrees = owned, owned
	projects, err := owned.Projects(context.Background())
	if err != nil || len(projects) != 1 || projects[0].ID != owner.ID {
		t.Fatalf("task clone became a separate project: %#v %v", projects, err)
	}
	branches, err := owned.Branches(context.Background(), owner.ID)
	if err != nil || len(branches) != 2 || branches[1].ProjectID != owner.ID || branches[1].IsHead {
		t.Fatalf("task selection group is missing or claims the primary branch: %#v %v", branches, err)
	}
	w := env.do(t, "GET", "/v1/projects/"+owner.ID+"/worktrees?branchId="+branches[1].ID, "")
	var catalogue worktreesEnvelope
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &catalogue) != nil || len(catalogue.Worktrees) != 1 || catalogue.Worktrees[0].ID != row.ID || catalogue.Worktrees[0].ProjectID != owner.ID {
		t.Fatalf("task branch filter lost original ownership: %d %s", w.Code, w.Body.String())
	}
	request, _ := json.Marshal(map[string]string{"worktreeId": row.ID, "harness": "shell"})
	created := sessionsDecodeOne(t, env.do(t, "POST", "/v1/sessions", string(request)), 201)
	if created.ProjectID != owner.ID || created.WorktreeID != row.ID || created.BranchID != branches[1].ID {
		t.Fatalf("session lost its owned task selection: %#v", created)
	}
	spec := env.be.spec(t, 0)
	if len(spec.Mounts) == 0 || spec.Mounts[0].HostPath != checkout {
		t.Fatalf("task session did not mount its isolated source: %#v", spec.Mounts)
	}
	for _, mount := range spec.Mounts {
		if mount.HostPath == owner.Path || mount.HostPath == filepath.Join(owner.Path, ".git") {
			t.Fatal("task session received writable main source or Git metadata")
		}
	}
	if path, err := owned.Path(context.Background(), primary.ID); err != nil || path != owner.Path {
		t.Fatalf("original resolver changed: %q %v", path, err)
	}
	// Copying the checkout directory is not private-ledger authorization.
	if _, err := owned.Path(context.Background(), project.WorktreeID(filepath.Join(root, ".luci-swarms", "unrecorded"))); !errors.Is(err, worktree.ErrUnknownID) {
		t.Fatalf("unrecorded checkout resolved: %v", err)
	}
}

func TestSwarmCheckoutResolutionRejectsForeignAndCorruptOwnership(t *testing.T) {
	path := t.TempDir()
	owner := project.Project{ID: "p-owner", Path: t.TempDir()}
	base := &sessionsLocator{projects: []project.Project{owner}}
	row := project.Worktree{ID: project.WorktreeID(path), ProjectID: owner.ID, Name: "Task", Path: path}
	for _, test := range []struct {
		name string
		rows []project.Worktree
	}{
		{"foreign project", []project.Worktree{{ID: row.ID, ProjectID: "p-foreign", Path: path}}},
		{"forged identity", []project.Worktree{{ID: "w-forged", ProjectID: owner.ID, Path: path}}},
		{"primary impersonation", []project.Worktree{{ID: row.ID, ProjectID: owner.ID, Path: path, IsPrimary: true}}},
		{"duplicate ledger identity", []project.Worktree{row, row}},
	} {
		t.Run(test.name, func(t *testing.T) {
			p := &swarmWorktrees{base: base, resolver: worktree.Static{}, owned: func(context.Context, string) ([]project.Worktree, error) { return test.rows, nil }}
			if _, err := p.Path(context.Background(), row.ID); err == nil {
				t.Fatal("invalid private ownership became a session mount")
			}
			if _, err := p.Worktrees(context.Background(), owner.ID); err == nil {
				t.Fatal("invalid private ownership entered project grant lookup")
			}
		})
	}
	readErr := errors.New("private ledger is unreadable")
	p := &swarmWorktrees{base: base, resolver: worktree.Static{}, owned: func(context.Context, string) ([]project.Worktree, error) { return nil, readErr }}
	if _, err := p.Path(context.Background(), row.ID); !errors.Is(err, readErr) {
		t.Fatalf("failed ownership read became a missing/retryable checkout: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.Path(ctx, row.ID); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled resolution continued: %v", err)
	}
}
