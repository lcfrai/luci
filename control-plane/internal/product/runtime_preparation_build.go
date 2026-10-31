package product

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/previewruntime"
)

func (s *Server) ReadRuntimePreparation(ctx context.Context, actor management.ExecutionActorRef, id string) (RuntimePreparation, error) {
	if err := s.kubernetesAuthority(ctx, actor, false, true, 0); err != nil {
		return RuntimePreparation{}, err
	}
	_, row, err := s.readRuntimePreparation(actor, id)
	if err != nil {
		return RuntimePreparation{}, err
	}
	if err = s.kubernetesAuthority(ctx, actor, false, true, 0); err != nil {
		return RuntimePreparation{}, err
	}
	return row.RuntimePreparation, nil
}
func (s *Server) runtimeBuildSource(ctx context.Context, row storedRuntimePreparation, runID, checkID string) (RuntimeBuildAttempt, error) {
	if err := s.runtimePreparationAuthority(ctx, row, true); err != nil {
		return RuntimeBuildAttempt{}, err
	}
	run, err := s.GetProjectSwarmRun(row.Plan.ProjectID, runID)
	if err != nil || run.ProjectID != row.Plan.ProjectID || run.Swarm == nil || !swarmAllIntegrated(run) || len(run.Input.Requirements) == 0 {
		return RuntimeBuildAttempt{}, ErrRuntimePreparation
	}
	check, err := s.SnapshotProjectRunCheck(ctx, row.Plan.ProjectID, checkID, run.Input.Approvals, run.Input.Requirements)
	if err != nil || validProjectRunCheck(run.Input, check) != nil {
		return RuntimeBuildAttempt{}, ErrRuntimePreparation
	}
	p, err := s.resolveProject(row.Plan.ProjectID)
	if err != nil {
		return RuntimeBuildAttempt{}, err
	}
	s.mu.RLock()
	state, readErr := s.readApprovedJourney(p.Path, p.ID)
	fresh := projectRunContextFreshness(run, state, readErr)
	plan := currentPlan(state, p.ID)
	s.mu.RUnlock()
	if fresh.Status != "current" {
		return RuntimeBuildAttempt{}, ErrRuntimePreparation
	}
	for _, wanted := range row.SuggestedTasks {
		if wanted.Role == "reviewer" {
			continue
		}
		selected := false
		for _, id := range run.Input.Implementation.TaskIDs {
			if id == wanted.ID {
				selected = true
			}
		}
		if !selected {
			return RuntimeBuildAttempt{}, errors.New("Include every reviewed runtime prerequisite task and its independent review in the checked run")
		}
		found := false
		for _, task := range plan.Tasks {
			if task.ID == wanted.ID && task.Role == wanted.Role && task.Title == wanted.Title && task.Acceptance == wanted.Acceptance && reflect.DeepEqual(task.WritePaths, wanted.WritePaths) && reflect.DeepEqual(task.RuntimePrerequisites, wanted.RuntimePrerequisites) {
				found = true
			}
		}
		if !found {
			return RuntimeBuildAttempt{}, ErrRuntimePreparation
		}
	}
	if err = s.ValidateDeploymentSource(ctx, p.ID, check.Commit); err != nil {
		return RuntimeBuildAttempt{}, err
	}
	latest, err := s.GetProjectSwarmRun(p.ID, runID)
	if err != nil || reviewDigest(latest.Input) != reviewDigest(run.Input) || !swarmAllIntegrated(latest) {
		return RuntimeBuildAttempt{}, ErrRuntimePreparation
	}
	if err = s.runtimePreparationAuthority(ctx, row, true); err != nil {
		return RuntimeBuildAttempt{}, err
	}
	return RuntimeBuildAttempt{RunID: runID, CheckID: checkID, Commit: check.Commit, SourceDigest: check.SourceDigest, RunInputDigest: reviewDigest(run.Input), CheckDigest: kubeCheckDigest(check), Attempted: true}, nil
}
func sameRuntimeBuildSource(a, b RuntimeBuildAttempt) bool {
	return a.RunID == b.RunID && a.CheckID == b.CheckID && a.Commit == b.Commit && a.SourceDigest == b.SourceDigest && a.RunInputDigest == b.RunInputDigest && a.CheckDigest == b.CheckDigest
}
func (s *Server) persistRuntimeBuild(ctx context.Context, before storedRuntimePreparation, index int, out previewruntime.Outcome) (storedRuntimePreparation, error) {
	s.runtimePreparationMu.Lock()
	defer s.runtimePreparationMu.Unlock()
	file, current, err := s.readRuntimePreparation(before.Plan.Actor, before.Plan.ID)
	if err != nil {
		return before, err
	}
	if current.Revision != before.Revision || reviewDigest(current) != reviewDigest(before) || index < 0 || index >= len(current.Builds) {
		return before, ErrRuntimePreparation
	}
	attempt := current.Builds[index]
	if attempt.Runtime != nil && attempt.Runtime.ImmutableImage != "" && attempt.Runtime.ImmutableImage != out.ImmutableImage {
		return before, ErrRuntimePreparation
	}
	if out.ProjectID != current.Plan.ProjectID || out.ReleaseID != current.Plan.ID+"-"+attempt.ServiceID || out.Commit != attempt.Commit || out.SourceDigest != attempt.SourceDigest || out.Target != "native-docker" || attempt.Runtime != nil && !previewruntime.Continues(attempt.Runtime.Identity, out.Identity) {
		return before, ErrRuntimePreparation
	}
	if out.ImmutableImage != "" && (!kubeImmutableImage.MatchString(out.ImmutableImage) || !strings.HasPrefix(out.ImmutableImage, current.Plan.Registry.Repository+"@")) {
		return before, ErrRuntimePreparation
	}
	current.Builds[index].Runtime = &out
	current.Revision++
	if err = runtimePreparationBytes(current); err != nil {
		return before, err
	}
	// This writes correlation/settlement only, never new external work. It may
	// retain an original response after revoked access; reads still require grants.
	if err = durableRunJSON(file, current); err != nil {
		return before, err
	}
	return current, nil
}

type runtimeImagePermit struct {
	planDigest string
	source     RuntimeBuildAttempt
	used       atomic.Bool
}

func (s *Server) BuildRuntimePreparationService(ctx context.Context, actor management.ExecutionActorRef, id, serviceID, runID, checkID string) (RuntimePreparation, error) {
	return s.executeRuntimePreparationService(ctx, actor, id, serviceID, runID, checkID, nil)
}

func (s *Server) executeRuntimePreparationService(ctx context.Context, actor management.ExecutionActorRef, id, serviceID, runID, checkID string, permit *runtimeImagePermit) (RuntimePreparation, error) {
	if permit != nil && !permit.used.CompareAndSwap(false, true) {
		return RuntimePreparation{}, ErrRuntimePreparation
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	_, row, err := s.readRuntimePreparation(actor, id)
	if err != nil {
		return RuntimePreparation{}, err
	}
	if err = s.runtimePreparationAuthority(ctx, row, true); err != nil {
		return RuntimePreparation{}, err
	}
	var build *RuntimePreparedBuild
	for i := range row.Plan.Builds {
		if row.Plan.Builds[i].Selection.ServiceID == serviceID {
			build = &row.Plan.Builds[i]
		}
	}
	if build == nil {
		return RuntimePreparation{}, ErrRuntimePreparation
	}
	for _, old := range row.Builds {
		if old.ServiceID == serviceID {
			if permit == nil {
				if s.runtimePreparationActive(id) {
					return row.RuntimePreparation, nil
				}
				return s.ObserveRuntimePreparationService(ctx, actor, id, serviceID)
			}
			if permit.planDigest != row.Plan.Digest || !sameRuntimeBuildSource(old, permit.source) || permit.source.ServiceID != serviceID || old.Runtime != nil {
				return row.RuntimePreparation, ErrRuntimePreparation
			}
		}
	}
	source, err := s.runtimeBuildSource(ctx, row, runID, checkID)
	if err != nil {
		return RuntimePreparation{}, err
	}
	source.ServiceID = serviceID
	if permit != nil && !sameRuntimeBuildSource(source, permit.source) {
		return row.RuntimePreparation, ErrRuntimePreparation
	}
	runtime := s.PreviewRuntime()
	if runtime.BuildAndPublish == nil || runtime.Observe == nil {
		return RuntimePreparation{}, errors.New("This engine does not support admitted image build and registry publication")
	}
	p, err := s.resolveProject(actor.ProjectID)
	if err != nil {
		return RuntimePreparation{}, err
	}
	directory, err := os.MkdirTemp("", "luci-runtime-build-*")
	if err != nil {
		return RuntimePreparation{}, err
	}
	defer os.RemoveAll(directory)
	if err = archiveCommittedProject(ctx, p.Path, source.Commit, directory); err != nil {
		return RuntimePreparation{}, err
	}
	if err = verifyRuntimeBuildFiles(directory, *build); err != nil {
		return RuntimePreparation{}, err
	}
	credentials, err := s.runtimeRegistryCredentials(actor.ProjectID, row.Plan.Registry)
	if err != nil {
		return RuntimePreparation{}, err
	}
	// Exclusive durable attempt marker before invoking even the shared admission
	// pipeline. Crash in this gap leaves unknown, never a fresh build allocation.
	s.runtimePreparationMu.Lock()
	file, current, err := s.readRuntimePreparation(actor, id)
	if err == nil && (current.Revision != row.Revision || reviewDigest(current) != reviewDigest(row)) {
		err = ErrRuntimePreparation
	}
	if err == nil {
		err = s.runtimePreparationAuthority(ctx, current, true)
	}
	index := len(current.Builds)
	if permit == nil {
		if err == nil {
			current.Builds = append(current.Builds, source)
			current.Revision++
			err = runtimePreparationBytes(current)
		}
		if err == nil {
			err = durableRunJSON(file, current)
		}
	} else if err == nil {
		index = -1
		for i, old := range current.Builds {
			if old.ServiceID == serviceID {
				index = i
				if old.Runtime != nil || !sameRuntimeBuildSource(old, permit.source) || current.Plan.Digest != permit.planDigest {
					err = ErrRuntimePreparation
				}
			}
		}
		if index < 0 {
			err = ErrRuntimePreparation
		}
	}
	s.runtimePreparationMu.Unlock()
	if err != nil {
		return row.RuntimePreparation, err
	}
	row = current
	request := previewruntime.Request{Identity: previewruntime.Identity{ProjectID: actor.ProjectID, ReleaseID: id + "-" + serviceID, Target: "native-docker", Commit: source.Commit, SourceDigest: source.SourceDigest}, SourceDirectory: directory, Dockerfile: build.Selection.Dockerfile, Port: build.Port, Actor: actor, MemoryMiB: build.Selection.MemoryMiB, CPUMillis: build.Selection.CPUMillis, Network: "bridge", Platform: "linux/amd64"}
	request.ValidateSource = func(call context.Context) error {
		now, err := s.runtimeBuildSource(call, row, runID, checkID)
		if err != nil {
			return err
		}
		if !sameRuntimeBuildSource(now, source) {
			return ErrRuntimePreparation
		}
		return verifyRuntimeBuildFiles(directory, *build)
	}
	// Task data and server-private credentials remain separated; neither image
	// layers nor model/task context receive registry or application secret values.
	publication := previewruntime.Publication{Repository: row.Plan.Registry.Repository, RegistryBinding: row.Plan.Registry.Fingerprint, RegistryAuth: credentials}
	out, buildErr := runtime.BuildAndPublish(ctx, request, publication, func(out previewruntime.Outcome) error {
		record, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		next, err := s.persistRuntimeBuild(record, row, index, out)
		if err == nil {
			row = next
		}
		return err
	})
	if index >= len(row.Builds) || row.Builds[index].Runtime == nil || reviewDigest(*row.Builds[index].Runtime) != reviewDigest(out) {
		record, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		next, persistErr := s.persistRuntimeBuild(record, row, index, out)
		cancel()
		if persistErr != nil {
			return row.RuntimePreparation, persistErr
		}
		row = next
	}
	if err = s.kubernetesAuthority(ctx, actor, false, true, 0); err != nil {
		return RuntimePreparation{}, err
	}
	return row.RuntimePreparation, buildErr
}

func (s *Server) ObserveRuntimePreparationService(ctx context.Context, actor management.ExecutionActorRef, id, serviceID string) (RuntimePreparation, error) {
	if err := s.kubernetesAuthority(ctx, actor, false, true, 0); err != nil {
		return RuntimePreparation{}, err
	}
	_, row, err := s.readRuntimePreparation(actor, id)
	if err != nil {
		return RuntimePreparation{}, err
	}
	index := -1
	for i, b := range row.Builds {
		if b.ServiceID == serviceID {
			index = i
		}
	}
	if index < 0 {
		return row.RuntimePreparation, ErrRuntimePreparation
	}
	before := row.Builds[index].Runtime
	// Missing acknowledgement is not proof admission never committed. No restart,
	// reconstruction, tag lookup or equivalent replacement operation is allowed.
	if before == nil || !before.Retained() {
		return row.RuntimePreparation, ErrRuntimePreparation
	}
	runtime := s.PreviewRuntime()
	if runtime.Observe == nil {
		return row.RuntimePreparation, ErrRuntimePreparation
	}
	out, err := runtime.Observe(ctx, before.Identity)
	if err != nil {
		return row.RuntimePreparation, err
	}
	if !previewruntime.Continues(before.Identity, out.Identity) {
		return row.RuntimePreparation, ErrRuntimePreparation
	}
	if err = s.kubernetesAuthority(ctx, actor, false, true, 0); err != nil {
		return RuntimePreparation{}, err
	}
	updated, err := s.persistRuntimeBuild(ctx, row, index, out)
	if err != nil {
		return row.RuntimePreparation, err
	}
	if err = s.kubernetesAuthority(ctx, actor, false, true, 0); err != nil {
		return RuntimePreparation{}, err
	}
	return updated.RuntimePreparation, nil
}
