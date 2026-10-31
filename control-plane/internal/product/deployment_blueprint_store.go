package product

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

const blueprintHistoryLimit = 50
const blueprintLedgerLimit = 8 << 20
const blueprintRevisionLimit = 1_000_000_000

var ErrBlueprintConflict = errors.New("The saved service blueprint changed. Keep your draft and review the current saved revision.")
var ErrBlueprintUnavailable = errors.New("The private service blueprint is unavailable or invalid.")
var ErrBlueprintContextChanged = errors.New("Project context changed. Refresh the current source and context before saving this blueprint.")

// BlueprintAccess checks current read/edit permission only, never execution or
// deployment authority. The API rehydrates current project grants on each call.
type BlueprintAccess func(context.Context, string, bool) error

type SavedDeploymentBlueprint struct {
	ProjectID  string                   `json:"projectId"`
	Revision   int                      `json:"revision"`
	Digest     string                   `json:"digest"`
	Blueprint  deploymentplan.Blueprint `json:"blueprint"`
	PlanDigest string                   `json:"planDigest"`
	Actor      ContextActor             `json:"actor"`
	At         string                   `json:"at"`
}

// The private record embeds the public record but retains retry witnesses only
// in private state, not in browser responses or a project repository.
type storedDeploymentBlueprint struct {
	SavedDeploymentBlueprint
	RequestKey  string `json:"requestKeyDigest"`
	RequestHash string `json:"requestDigest"`
}

type deploymentBlueprintLedger struct {
	Version      int                         `json:"version"`
	ProjectID    string                      `json:"projectId"`
	AccountScope string                      `json:"accountScope"`
	ProjectScope string                      `json:"projectScope"`
	Revisions    []storedDeploymentBlueprint `json:"revisions"`
}

type DeploymentBlueprintView struct {
	ProjectID      string                    `json:"projectId"`
	Saved          *SavedDeploymentBlueprint `json:"saved"`
	SourceVerified bool                      `json:"sourceVerified"`
	EffectsApplied bool                      `json:"effectsApplied"`
}

type DeploymentBlueprintSave struct {
	ExpectedRevision int                      `json:"expectedRevision"`
	ExpectedDigest   string                   `json:"expectedDigest"`
	IdempotencyKey   string                   `json:"idempotencyKey"`
	Blueprint        deploymentplan.Blueprint `json:"blueprint"`
}

func savedBlueprintDigest(row SavedDeploymentBlueprint) string {
	row.Digest = ""
	return reviewDigest(row)
}

func blueprintStorageSafe(b deploymentplan.Blueprint) bool {
	// Storage accepts resource references, not a new credential/configuration
	// store. Reject common inline credential forms; arbitrary instructions still
	// remain private project content and are never executed by saving.
	for _, service := range b.Services {
		for _, value := range append(append([]string{}, service.Command...), service.Args...) {
			lower := strings.ToLower(strings.TrimSpace(value))
			for _, flag := range []string{"--password", "--token", "--api-key", "--apikey", "--secret", "password=", "token=", "api_key=", "apikey=", "secret="} {
				if lower == flag || strings.HasPrefix(lower, flag+"=") || strings.HasSuffix(flag, "=") && strings.HasPrefix(lower, flag) {
					return false
				}
			}
		}
	}
	return true
}

func (s *Server) readDeploymentBlueprintLedger(p project.Project) (string, deploymentBlueprintLedger, error) {
	scope, err := s.approvalScopes(p.Path, p.ID)
	empty := deploymentBlueprintLedger{Version: 1, ProjectID: p.ID, Revisions: []storedDeploymentBlueprint{}}
	if err != nil {
		return "", empty, err
	}
	empty.AccountScope, empty.ProjectScope = scope.AccountScope, scope.ProjectScope
	path, err := guardedProjectPath(s.stateDir, "deployment-blueprint-"+p.ID+".json", true)
	if err != nil {
		return "", empty, err
	}
	var saved deploymentBlueprintLedger
	err = externalReadJSON(path, blueprintLedgerLimit, &saved)
	if errors.Is(err, os.ErrNotExist) {
		return path, empty, nil
	}
	if err != nil || saved.Version != 1 || saved.ProjectID != p.ID || saved.AccountScope != empty.AccountScope || saved.ProjectScope != empty.ProjectScope || saved.Revisions == nil || len(saved.Revisions) > blueprintHistoryLimit {
		return "", empty, ErrBlueprintUnavailable
	}
	keys := map[string]bool{}
	for index, row := range saved.Revisions {
		plan, compileErr := deploymentplan.Compile(row.Blueprint)
		_, timeErr := time.Parse(time.RFC3339Nano, row.At)
		if compileErr != nil || timeErr != nil || row.ProjectID != p.ID || row.Blueprint.ProjectID != p.ID || row.Revision < 1 || row.Revision > blueprintRevisionLimit || row.Revision != saved.Revisions[0].Revision+index || row.Digest != savedBlueprintDigest(row.SavedDeploymentBlueprint) || row.PlanDigest != plan.Digest || !blueprintStorageSafe(row.Blueprint) || !hexDigest(row.RequestKey) || !hexDigest(row.RequestHash) || keys[row.RequestKey] || row.Actor.ID == "" || row.Actor.DisplayName == "" {
			return "", empty, ErrBlueprintUnavailable
		}
		keys[row.RequestKey] = true
	}
	return path, saved, nil
}

func blueprintView(id string, ledger deploymentBlueprintLedger) DeploymentBlueprintView {
	view := DeploymentBlueprintView{ProjectID: id}
	if len(ledger.Revisions) > 0 {
		row := ledger.Revisions[len(ledger.Revisions)-1].SavedDeploymentBlueprint
		view.Saved = &row
	}
	return view
}

func (s *Server) ReadDeploymentBlueprint(ctx context.Context, id string, access BlueprintAccess) (DeploymentBlueprintView, error) {
	if access == nil {
		return DeploymentBlueprintView{}, ErrBlueprintUnavailable
	}
	if err := access(ctx, id, false); err != nil {
		return DeploymentBlueprintView{}, err
	}
	p, err := s.resolveProject(id)
	if err != nil {
		return DeploymentBlueprintView{}, err
	}
	s.mu.RLock()
	_, ledger, err := s.readDeploymentBlueprintLedger(p)
	s.mu.RUnlock()
	if err != nil {
		return DeploymentBlueprintView{}, err
	}
	if err := access(ctx, id, false); err != nil {
		return DeploymentBlueprintView{}, err
	}
	return blueprintView(id, ledger), nil
}

func (s *Server) SaveDeploymentBlueprint(ctx context.Context, id string, req DeploymentBlueprintSave, access BlueprintAccess) (DeploymentBlueprintView, error) {
	actor, ok := authenticatedContextActor(ctx)
	if !ok || access == nil {
		return DeploymentBlueprintView{}, ErrBlueprintUnavailable
	}
	if err := access(ctx, id, true); err != nil {
		return DeploymentBlueprintView{}, err
	}
	if req.ExpectedRevision < 0 || req.ExpectedRevision >= blueprintRevisionLimit || (req.ExpectedRevision == 0 && req.ExpectedDigest != "") || (req.ExpectedRevision > 0 && !hexDigest(req.ExpectedDigest)) || len(req.IdempotencyKey) < 8 || !validConnectionToken(req.IdempotencyKey, 128, false) || req.Blueprint.ProjectID != id || !blueprintStorageSafe(req.Blueprint) {
		return DeploymentBlueprintView{}, ErrBlueprintUnavailable
	}
	plan, err := deploymentplan.Compile(req.Blueprint)
	if err != nil {
		return DeploymentBlueprintView{}, err
	}
	p, err := s.resolveProject(id)
	if err != nil {
		return DeploymentBlueprintView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	path, ledger, err := s.readDeploymentBlueprintLedger(p)
	if err != nil {
		return DeploymentBlueprintView{}, err
	}
	key, requestDigest := reviewDigest([]string{id, actor.ID, req.IdempotencyKey}), reviewDigest(req)
	for index, row := range ledger.Revisions {
		if row.RequestKey == key {
			if row.RequestHash != requestDigest || index != len(ledger.Revisions)-1 {
				return DeploymentBlueprintView{}, ErrBlueprintConflict
			}
			if err := access(ctx, id, true); err != nil {
				return DeploymentBlueprintView{}, err
			}
			return blueprintView(id, ledger), nil
		}
	}
	view := blueprintView(id, ledger)
	latest := 0
	if view.Saved != nil {
		latest = view.Saved.Revision
	}
	if req.ExpectedRevision != latest || (view.Saved != nil && req.ExpectedDigest != view.Saved.Digest) {
		return DeploymentBlueprintView{}, ErrBlueprintConflict
	}
	// A saved topology records current context, not a deployment approval or a
	// verified Git commit. This reads existing state only; no Git/provider work.
	if _, err := s.foundationContextLocked(p, plan.Blueprint); err != nil {
		return DeploymentBlueprintView{}, ErrBlueprintContextChanged
	}
	row := SavedDeploymentBlueprint{ProjectID: id, Revision: latest + 1, Blueprint: plan.Blueprint, PlanDigest: plan.Digest, Actor: actor, At: time.Now().UTC().Format(time.RFC3339Nano)}
	row.Digest = savedBlueprintDigest(row)
	ledger.Revisions = append(ledger.Revisions, storedDeploymentBlueprint{SavedDeploymentBlueprint: row, RequestKey: key, RequestHash: requestDigest})
	if len(ledger.Revisions) > blueprintHistoryLimit {
		ledger.Revisions = ledger.Revisions[len(ledger.Revisions)-blueprintHistoryLimit:]
	}
	raw, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil || len(raw)+1 > blueprintLedgerLimit {
		return DeploymentBlueprintView{}, ErrBlueprintUnavailable
	}
	if err := access(ctx, id, true); err != nil {
		return DeploymentBlueprintView{}, err
	}
	if err := durableRunJSON(path, ledger); err != nil {
		return DeploymentBlueprintView{}, err
	}
	return blueprintView(id, ledger), nil
}
