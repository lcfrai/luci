package product

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/previewruntime"
)

type RuntimeBuildSelection struct {
	ServiceID        string            `json:"serviceId"`
	Dockerfile       string            `json:"dockerfile"`
	PackageJSON      string            `json:"packageJson,omitempty"`
	ArtifactPaths    map[string]string `json:"artifactPaths,omitempty"`
	IntegrationPaths []string          `json:"integrationPaths,omitempty"`
	MemoryMiB        int               `json:"memoryMiB"`
	CPUMillis        int               `json:"cpuMillis"`
}
type RuntimePreparationInput struct {
	Resources               *RuntimeResourceSelection `json:"resources,omitempty"`
	RequestKey              string                    `json:"requestKey"`
	BlueprintRevision       int                       `json:"blueprintRevision"`
	BlueprintDigest         string                    `json:"blueprintDigest"`
	RegistryID              string                    `json:"registryId"`
	RegistryRevision        int                       `json:"registryRevision"`
	Builds                  []RuntimeBuildSelection   `json:"builds"`
	MonthlyBudgetUSD        float64                   `json:"monthlyBudgetUsd"`
	AcknowledgeBuildAndPush bool                      `json:"acknowledgeBuildAndPush"`
}
type RuntimePreparedBuild struct {
	Selection    RuntimeBuildSelection `json:"selection"`
	Port         int                   `json:"port"`
	Artifacts    []TaskRuntimeArtifact `json:"artifacts"`
	Dependencies map[string]string     `json:"dependencies,omitempty"`
}
type RuntimePreparationPlan struct {
	Resources         *RuntimeResourcePlan         `json:"resources,omitempty"`
	ID                string                       `json:"id"`
	ProjectID         string                       `json:"projectId"`
	Actor             management.ExecutionActorRef `json:"actor"`
	BlueprintRevision int                          `json:"blueprintRevision"`
	BlueprintDigest   string                       `json:"blueprintDigest"`
	ContextDigest     string                       `json:"contextDigest"`
	Registry          RuntimeRegistry              `json:"registry"`
	Builds            []RuntimePreparedBuild       `json:"builds"`
	MonthlyBudgetUSD  float64                      `json:"monthlyBudgetUsd"`
	CreatedAt         string                       `json:"createdAt"`
	ExpiresAt         string                       `json:"expiresAt"`
	Digest            string                       `json:"digest"`
}
type RuntimeBuildAttempt struct {
	ServiceID      string                  `json:"serviceId"`
	RunID          string                  `json:"runId"`
	CheckID        string                  `json:"checkId"`
	Commit         string                  `json:"commit"`
	SourceDigest   string                  `json:"sourceDigest"`
	RunInputDigest string                  `json:"runInputDigest"`
	CheckDigest    string                  `json:"checkDigest"`
	Attempted      bool                    `json:"attempted"`
	Runtime        *previewruntime.Outcome `json:"runtime,omitempty"`
}
type RuntimePreparation struct {
	Resources      *RuntimeResourceOperation    `json:"resources,omitempty"`
	Plan           RuntimePreparationPlan       `json:"plan"`
	Revision       int                          `json:"revision"`
	Execution      *RuntimePreparationExecution `json:"execution,omitempty"`
	Builds         []RuntimeBuildAttempt        `json:"builds"`
	SuggestedTasks []ProjectTask                `json:"suggestedTasks"`
	// Derived only after every original build receipt confirms a registry digest.
	SavedBlueprintRevision int    `json:"savedBlueprintRevision,omitempty"`
	SavedBlueprintDigest   string `json:"savedBlueprintDigest,omitempty"`
}
type storedRuntimePreparation struct {
	PrivateSecrets []runtimePrivateSecret `json:"privateSecrets,omitempty"`
	RuntimePreparation
	Version       int                      `json:"version"`
	AccountScope  string                   `json:"accountScope"`
	ProjectScope  string                   `json:"projectScope"`
	RequestDigest string                   `json:"requestDigest"`
	Template      deploymentplan.Blueprint `json:"template"`
}

func runtimePreparationDigest(p RuntimePreparationPlan) string { p.Digest = ""; return reviewDigest(p) }
func runtimePreparationID(v string) bool {
	if len(v) != 37 || !strings.HasPrefix(v, "prep-") {
		return false
	}
	raw, err := hex.DecodeString(v[5:])
	return err == nil && len(raw) == 16 && hex.EncodeToString(raw) == v[5:]
}
func runtimeTaskID(prep, service string) string {
	return "runtime-" + prep[5:13] + "-" + reviewDigest(service)[:32]
}

func (s *Server) runtimePreparationPath(projectID, id string) (string, string, string, error) {
	if !runtimePreparationID(id) {
		return "", "", "", ErrRuntimePreparation
	}
	p, err := s.resolveProject(projectID)
	if err != nil {
		return "", "", "", err
	}
	s.mu.RLock()
	scope, err := s.approvalScopes(p.Path, p.ID)
	s.mu.RUnlock()
	if err != nil {
		return "", "", "", err
	}
	file, err := privateApplicationStatePathForRoot(s.config.ProjectsDir, s.stateDir, "runtime-preparation-"+id)
	return file, scope.AccountScope, scope.ProjectScope, err
}
func (s *Server) readRuntimePreparation(actor management.ExecutionActorRef, id string) (string, storedRuntimePreparation, error) {
	file, account, project, err := s.runtimePreparationPath(actor.ProjectID, id)
	if err != nil {
		return "", storedRuntimePreparation{}, err
	}
	var row storedRuntimePreparation
	if err = externalReadJSON(file, 1<<20, &row); err != nil {
		return "", row, err
	}
	if row.Version != 1 || row.AccountScope != account || row.ProjectScope != project || row.Plan.ID != id || row.Plan.ProjectID != actor.ProjectID || row.Plan.Actor != actor || row.Revision < 1 || row.Revision > 1_000_000_000 || row.Plan.Digest != runtimePreparationDigest(row.Plan) || len(row.Plan.Builds) > 8 || len(row.Plan.Builds) == 0 && row.Plan.Resources == nil || len(row.Builds) > 8 || len(row.SuggestedTasks) > 16 || !hexDigest(row.RequestDigest) || !blueprintStorageSafe(row.Template) {
		return "", row, ErrRuntimePreparation
	}
	if row.Template.ProjectID != actor.ProjectID || row.Plan.Registry.ProjectID != actor.ProjectID {
		return "", row, ErrRuntimePreparation
	}
	if !validRuntimePreparationExecution(row.RuntimePreparation) {
		return "", row, ErrRuntimePreparation
	}
	if err := validateStoredRuntimeResources(row); err != nil {
		return "", row, err
	}
	for _, task := range row.SuggestedTasks {
		if err := validateTaskRuntimePrerequisites(task.RuntimePrerequisites); err != nil {
			return "", row, err
		}
	}
	seen := map[string]bool{}
	for _, build := range row.Builds {
		if seen[build.ServiceID] || !build.Attempted || !deliverySHA.MatchString(build.Commit) || !hexDigest(build.SourceDigest) || !hexDigest(build.RunInputDigest) || !hexDigest(build.CheckDigest) {
			return "", row, ErrRuntimePreparation
		}
		seen[build.ServiceID] = true
		if build.Runtime != nil && (build.Runtime.ProjectID != actor.ProjectID || build.Runtime.ReleaseID != id+"-"+build.ServiceID || build.Runtime.Commit != build.Commit || build.Runtime.SourceDigest != build.SourceDigest || build.Runtime.Target != "native-docker") {
			return "", row, ErrRuntimePreparation
		}
	}
	return file, row, nil
}
func runtimePreparationBytes(row storedRuntimePreparation) error {
	raw, err := json.MarshalIndent(row, "", "  ")
	if err != nil || len(raw)+1 > 1<<20 {
		return ErrRuntimePreparation
	}
	return nil
}
func (s *Server) runtimePreparationTemplate(row storedRuntimePreparation) error {
	p, err := s.resolveProject(row.Plan.ProjectID)
	if err != nil {
		return err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ledger, err := s.readDeploymentBlueprintLedger(p)
	view := blueprintView(p.ID, ledger)
	if err != nil || view.Saved == nil || view.Saved.Revision != row.Plan.BlueprintRevision || view.Saved.Digest != row.Plan.BlueprintDigest || reviewDigest(view.Saved.Blueprint) != reviewDigest(row.Template) {
		return ErrRuntimePreparation
	}
	digest, err := s.foundationContextLocked(p, row.Template)
	if err != nil || digest != row.Plan.ContextDigest {
		return ErrRuntimePreparation
	}
	return nil
}
func (s *Server) runtimePreparationAuthority(ctx context.Context, row storedRuntimePreparation, mutation bool) error {
	if err := s.kubernetesAuthority(ctx, row.Plan.Actor, mutation, true, row.Plan.MonthlyBudgetUSD); err != nil {
		return err
	}
	if mutation {
		expires, err := time.Parse(time.RFC3339Nano, row.Plan.ExpiresAt)
		if err != nil || !time.Now().Before(expires) {
			return ErrRuntimePreparation
		}
	}
	if _, err := s.runtimeRegistryCredentials(row.Plan.ProjectID, row.Plan.Registry); err != nil {
		return err
	}
	return s.runtimePreparationTemplate(row)
}

func (s *Server) ReviewRuntimePreparation(ctx context.Context, actor management.ExecutionActorRef, input RuntimePreparationInput) (RuntimePreparation, error) {
	if err := s.kubernetesAuthority(ctx, actor, false, true, 0); err != nil {
		return RuntimePreparation{}, err
	}
	if !input.AcknowledgeBuildAndPush || !validConnectionToken(input.RequestKey, 128, false) || len(input.RequestKey) < 8 || len(input.Builds) > 8 || len(input.Builds) == 0 && input.Resources == nil {
		return RuntimePreparation{}, ErrRuntimePreparation
	}
	// Policy authorizes potential new build/registry cost before retaining consent.
	if err := s.kubernetesAuthority(ctx, actor, true, true, input.MonthlyBudgetUSD); err != nil {
		return RuntimePreparation{}, err
	}
	// Exact retained request recovery comes before newly generated private keys
	// or target reads. The same original consent never acquires replacement values.
	idEarly := "prep-" + reviewDigest([]string{actor.UserID, actor.TeamID, actor.ProjectID, input.RequestKey})[:32]
	if _, prior, readErr := s.readRuntimePreparation(actor, idEarly); readErr == nil {
		if prior.RequestDigest != reviewDigest(input) {
			return RuntimePreparation{}, ErrRuntimePreparation
		}
		if err := s.kubernetesAuthority(ctx, actor, false, true, 0); err != nil {
			return RuntimePreparation{}, err
		}
		return prior.RuntimePreparation, nil
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return RuntimePreparation{}, readErr
	}
	p, err := s.resolveProject(actor.ProjectID)
	if err != nil {
		return RuntimePreparation{}, err
	}
	s.mu.RLock()
	_, ledger, err := s.readDeploymentBlueprintLedger(p)
	view := blueprintView(p.ID, ledger)
	contextDigest := ""
	if err == nil && view.Saved != nil {
		contextDigest, err = s.foundationContextLocked(p, view.Saved.Blueprint)
	}
	s.mu.RUnlock()
	if err != nil || view.Saved == nil || view.Saved.Revision != input.BlueprintRevision || view.Saved.Digest != input.BlueprintDigest {
		return RuntimePreparation{}, ErrRuntimePreparation
	}
	_, registries, err := s.readRuntimeRegistries(p.ID)
	if err != nil {
		return RuntimePreparation{}, err
	}
	var registry RuntimeRegistry
	for _, candidate := range registries.Registries {
		if candidate.ID == input.RegistryID && candidate.Revision == input.RegistryRevision {
			registry = candidate.RuntimeRegistry
		}
	}
	if registry.ID == "" {
		return RuntimePreparation{}, ErrRuntimePreparation
	}
	id := "prep-" + reviewDigest([]string{actor.UserID, actor.TeamID, p.ID, input.RequestKey})[:32]
	file, account, project, err := s.runtimePreparationPath(p.ID, id)
	if err != nil {
		return RuntimePreparation{}, err
	}
	allArtifacts, err := deploymentplan.PreparationArtifacts(view.Saved.Blueprint)
	if err != nil {
		return RuntimePreparation{}, err
	}
	now := time.Now().UTC()
	row := storedRuntimePreparation{Version: 1, AccountScope: account, ProjectScope: project, RequestDigest: reviewDigest(input), Template: view.Saved.Blueprint, RuntimePreparation: RuntimePreparation{Revision: 1, Builds: []RuntimeBuildAttempt{}, SuggestedTasks: []ProjectTask{}, Plan: RuntimePreparationPlan{ID: id, ProjectID: p.ID, Actor: actor, BlueprintRevision: input.BlueprintRevision, BlueprintDigest: input.BlueprintDigest, ContextDigest: contextDigest, Registry: registry, MonthlyBudgetUSD: input.MonthlyBudgetUSD, Builds: []RuntimePreparedBuild{}, CreatedAt: now.Format(time.RFC3339Nano), ExpiresAt: now.Add(24 * time.Hour).Format(time.RFC3339Nano)}}}
	seen := map[string]bool{}
	destinations := map[string]bool{}
	for _, selection := range input.Builds {
		if seen[selection.ServiceID] || !runtimeArtifactPath(selection.Dockerfile) || selection.MemoryMiB < 128 || selection.MemoryMiB > 32768 || selection.CPUMillis < 100 || selection.CPUMillis > 16000 {
			return RuntimePreparation{}, ErrRuntimePreparation
		}
		seen[selection.ServiceID] = true
		var service *deploymentplan.Service
		for i := range row.Template.Services {
			if row.Template.Services[i].ID == selection.ServiceID {
				service = &row.Template.Services[i]
			}
		}
		if service == nil || service.Kind == "collector" || service.Port < 1 || service.Port > 65535 {
			return RuntimePreparation{}, ErrRuntimePreparation
		}
		build := RuntimePreparedBuild{Selection: selection, Port: service.Port, Artifacts: []TaskRuntimeArtifact{}}
		if service.Telemetry != nil && service.Telemetry.Instrumentation != nil {
			for _, p := range selection.IntegrationPaths {
				if !validProjectTaskWritePath(p) {
					return RuntimePreparation{}, ErrRuntimePreparation
				}
			}
			build.Dependencies = map[string]string{}
			for k, v := range service.Telemetry.Instrumentation.Dependencies {
				build.Dependencies[k] = v
			}
			if !runtimeArtifactPath(selection.PackageJSON) || len(selection.ArtifactPaths) != 4 || len(selection.IntegrationPaths) < 1 || len(selection.IntegrationPaths) > 16 {
				return RuntimePreparation{}, ErrRuntimePreparation
			}
			for _, a := range allArtifacts {
				if !strings.HasPrefix(a.Name, service.ID+".") {
					continue
				}
				destination := selection.ArtifactPaths[a.Name]
				if !runtimeArtifactPath(destination) || destinations[destination] {
					return RuntimePreparation{}, ErrRuntimePreparation
				}
				destinations[destination] = true
				build.Artifacts = append(build.Artifacts, TaskRuntimeArtifact{Path: destination, Content: a.Content, SHA256: a.SHA256})
			}
			if len(build.Artifacts) != 4 {
				return RuntimePreparation{}, ErrRuntimePreparation
			}
		} else if len(selection.ArtifactPaths) > 0 || selection.PackageJSON != "" {
			return RuntimePreparation{}, ErrRuntimePreparation
		}
		row.Plan.Builds = append(row.Plan.Builds, build)
	}
	if input.Resources != nil {
		plan, private, resourceErr := s.reviewRuntimeResourcePlan(ctx, actor, row, *input.Resources)
		if resourceErr != nil {
			return RuntimePreparation{}, resourceErr
		}
		row.Plan.Resources = plan
		row.PrivateSecrets = private
	}
	row.Plan.Digest = runtimePreparationDigest(row.Plan)
	for _, build := range row.Plan.Builds {
		id := runtimeTaskID(row.Plan.ID, build.Selection.ServiceID)
		var refs *[]TaskRuntimePrerequisite
		paths := []string{build.Selection.Dockerfile}
		if len(build.Artifacts) > 0 {
			rows := []TaskRuntimePrerequisite{{PreparationID: row.Plan.ID, PreparationDigest: row.Plan.Digest, ServiceID: build.Selection.ServiceID, Kind: "instrumentation", Artifacts: build.Artifacts, Dependencies: build.Dependencies}}
			refs = &rows
			paths = append(paths, build.Selection.PackageJSON)
			paths = append(paths, build.Selection.IntegrationPaths...)
			for _, a := range build.Artifacts {
				paths = append(paths, a.Path)
			}
		}
		row.SuggestedTasks = append(row.SuggestedTasks, ProjectTask{ID: id, Title: "Package and connect " + build.Selection.ServiceID, Status: "todo", Role: "coding", WritePaths: paths, Acceptance: "Create or update the selected stack-appropriate Dockerfile from this approved project's actual application and context; retain an existing appropriate Dockerfile. Verify the actual app and build behavior with meaningful checks. Do not fabricate an operator-authored substitute app. Where reviewed SDK inputs are present, integrate their exact files into existing startup and outcome handlers and preserve exact declared production dependency versions. Do not acquire credentials or claim telemetry is running from task completion alone.", RuntimePrerequisites: refs}, ProjectTask{ID: "review-" + id, Title: "Independently review packaging for " + build.Selection.ServiceID, Status: "todo", Role: "reviewer", ReviewOfTaskID: id, Acceptance: "Inspect the exact candidate Dockerfile against the approved actual application, source and meaningful checks. Review any exact SDK dependency/startup/shutdown/outcome integration, consent and data minimization. Do not edit the repository or claim deployed quality from generated files."})
	}
	if err = validatePlan(ProjectPlan{ProjectID: p.ID, Tasks: row.SuggestedTasks}); err != nil {
		return RuntimePreparation{}, err
	}
	if err = validateStoredRuntimeResources(row); err != nil {
		return RuntimePreparation{}, err
	}
	if err = runtimePreparationBytes(row); err != nil {
		return RuntimePreparation{}, err
	}
	s.runtimePreparationMu.Lock()
	defer s.runtimePreparationMu.Unlock()
	if _, old, readErr := s.readRuntimePreparation(actor, id); readErr == nil {
		if old.RequestDigest != row.RequestDigest {
			return RuntimePreparation{}, ErrRuntimePreparation
		}
		if err = s.kubernetesAuthority(ctx, actor, false, true, 0); err != nil {
			return RuntimePreparation{}, err
		}
		if err = s.indexRuntimePreparation(old); err != nil {
			return RuntimePreparation{}, err
		}
		if err = s.kubernetesAuthority(ctx, actor, false, true, 0); err != nil {
			return RuntimePreparation{}, err
		}
		return old.RuntimePreparation, nil
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return RuntimePreparation{}, readErr
	}
	if err = s.runtimePreparationAuthority(ctx, row, true); err != nil {
		return RuntimePreparation{}, err
	}
	if err = durableRunJSON(file, row); err != nil {
		return RuntimePreparation{}, err
	}
	if err = s.indexRuntimePreparation(row); err != nil {
		return RuntimePreparation{}, err
	}
	if err = s.kubernetesAuthority(ctx, actor, false, true, 0); err != nil {
		return RuntimePreparation{}, err
	}
	return row.RuntimePreparation, nil
}

func verifyRuntimeBuildFiles(root string, build RuntimePreparedBuild) error {
	for _, a := range build.Artifacts {
		file, err := guardedProjectPath(root, a.Path, false)
		if err != nil {
			return ErrRuntimePreparation
		}
		raw, err := readGitReviewFile(file, 16<<10)
		sum := sha256.Sum256(raw)
		if err != nil || hex.EncodeToString(sum[:]) != a.SHA256 || string(raw) != a.Content {
			return fmt.Errorf("Commit the exact reviewed runtime file %s before building", a.Path)
		}
	}
	if len(build.Dependencies) > 0 {
		file, err := guardedProjectPath(root, build.Selection.PackageJSON, false)
		if err != nil {
			return ErrRuntimePreparation
		}
		raw, err := readGitReviewFile(file, 1<<20)
		if err != nil {
			return err
		}
		var pkg struct {
			Dependencies    map[string]string `json:"dependencies"`
			DevDependencies map[string]string `json:"devDependencies"`
		}
		if json.Unmarshal(raw, &pkg) != nil {
			return ErrRuntimePreparation
		}
		for k, v := range build.Dependencies {
			if pkg.Dependencies[k] != v || pkg.DevDependencies[k] != "" && pkg.DevDependencies[k] != v {
				return fmt.Errorf("Commit exact production dependency %s@%s before building", k, v)
			}
		}
	}
	_, err := guardedProjectPath(root, build.Selection.Dockerfile, false)
	return err
}
