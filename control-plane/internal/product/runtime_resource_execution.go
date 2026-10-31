package product

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/previewruntime"
)

// Only a successful exclusive reservation in THIS invocation grants the right
// to execute. A retained record, even one with no provider acknowledgement,
// never reconstructs that ephemeral permit after a crash or repeated action.
func (s *Server) RunRuntimeResourcePreparation(ctx context.Context, actor management.ExecutionActorRef, id, runID, checkID string) (RuntimePreparation, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	ctx = WithKubernetesTargetResolution(ctx)
	if err := s.kubernetesAuthority(ctx, actor, false, true, 0); err != nil {
		return RuntimePreparation{}, err
	}
	_, row, err := s.readRuntimePreparation(actor, id)
	if err != nil {
		return RuntimePreparation{}, err
	}
	if row.Plan.Resources == nil {
		return row.RuntimePreparation, nil
	}
	if row.Resources != nil {
		return s.ObserveRuntimeResourcePreparation(ctx, actor, id)
	}
	key := "runtime-resources-" + id
	s.mu.Lock()
	if s.coordinatorActions == nil {
		s.coordinatorActions = map[string]bool{}
	}
	live := s.coordinatorActions[key]
	if !live {
		s.coordinatorActions[key] = true
	}
	s.mu.Unlock()
	if live {
		return row.RuntimePreparation, nil
	}
	defer func() { s.mu.Lock(); delete(s.coordinatorActions, key); s.mu.Unlock() }()
	source, err := s.runtimeBuildSource(ctx, row, runID, checkID)
	if err != nil {
		return row.RuntimePreparation, err
	}
	if err = runtimeResourceSourceMatchesBuilds(row, source); err != nil {
		return row.RuntimePreparation, err
	}
	next := &RuntimeResourceOperation{Source: source, Revision: 1, State: "reserved", ObservedAt: coordinatorPreviewNow(), Resources: []RuntimeOwnedResource{}, Models: []RuntimeModelTransfer{}}
	for _, volume := range row.Plan.Resources.Volumes {
		next.Resources = append(next.Resources, RuntimeOwnedResource{Kind: "PersistentVolumeClaim", Name: volume.ClaimName, DesiredDigest: runtimeVolumeDigest(volume), State: "reserved"})
	}
	for _, secret := range row.Plan.Resources.Secrets {
		next.Resources = append(next.Resources, RuntimeOwnedResource{Kind: "Secret", Name: secret.Name, DesiredDigest: runtimeSecretDigest(secret), State: "reserved"})
	}
	for _, model := range row.Plan.Resources.Models {
		next.Models = append(next.Models, RuntimeModelTransfer{ServiceID: model.ServiceID, OperationID: id + "-model-" + reviewDigest(model.ServiceID)[:12], PodName: runtimeTransferPod(id, model.ServiceID), State: "reserved"})
	}
	// persistRuntimeResource requires the entire prior record and revision, so
	// two simultaneous Start/continuation calls cannot both own this operation.
	row, err = s.persistRuntimeResource(ctx, row, next)
	if err != nil {
		return row.RuntimePreparation, err
	}
	writer := &runtimeResourceWriter{server: s, row: row}
	if len(row.Plan.Resources.Models) > 0 {
		if err = s.buildRuntimeModelSink(ctx, writer); err != nil {
			return s.runtimeResourceResult(ctx, actor, writer, err)
		}
	}
	if err = s.runRuntimeResourceWrites(ctx, writer); err != nil {
		return s.runtimeResourceResult(ctx, actor, writer, err)
	}
	return s.runtimeResourceResult(ctx, actor, writer, nil)
}

func (s *Server) runtimeResourceResult(ctx context.Context, actor management.ExecutionActorRef, w *runtimeResourceWriter, cause error) (RuntimePreparation, error) {
	row := w.snapshot()
	if cause != nil {
		record, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		_ = w.mutate(record, func(op *RuntimeResourceOperation) error {
			if op.State != "ready" {
				op.State = "unknown"
			}
			return nil
		})
		cancel()
		row = w.snapshot()
	}
	if err := s.kubernetesAuthority(ctx, actor, false, true, 0); err != nil {
		return RuntimePreparation{}, err
	}
	return row.RuntimePreparation, cause
}

func (s *Server) buildRuntimeModelSink(ctx context.Context, w *runtimeResourceWriter) error {
	row := w.snapshot()
	runtime := s.PreviewRuntime()
	if runtime.BuildAndPublish == nil || runtime.Observe == nil || row.Resources == nil || row.Resources.HelperBuildAttempted {
		return ErrRuntimePreparation
	}
	dockerfile, err := runtimeModelSinkDockerfile(row.Plan.Resources.ModelSinkBuilderImage)
	if err != nil {
		return err
	}
	directory, err := os.MkdirTemp("", "luci-model-sink-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(directory)
	for name, content := range map[string]string{"main.go": runtimeModelSinkSource, "Dockerfile": dockerfile} {
		if err = os.WriteFile(filepath.Join(directory, name), []byte(content), 0600); err != nil {
			return err
		}
	}
	credentials, err := s.runtimeRegistryCredentials(row.Plan.ProjectID, row.Plan.Registry)
	if err != nil {
		return err
	}
	if err = s.runtimeResourceMutationAuthority(ctx, row); err != nil {
		return err
	}
	if err = w.mutate(ctx, func(op *RuntimeResourceOperation) error {
		if op.HelperBuildAttempted {
			return ErrRuntimePreparation
		}
		op.HelperBuildAttempted = true
		op.State = "building-helper"
		return nil
	}); err != nil {
		return err
	}
	row = w.snapshot()
	// The controller source profile is distinct from application source. It is
	// not an agent-authored substitute app, and has no model/provider tools.
	request := previewruntime.Request{Identity: previewruntime.Identity{ProjectID: row.Plan.ProjectID, ReleaseID: row.Plan.ID + "-model-sink", Target: "native-docker", Commit: row.Resources.Source.Commit, SourceDigest: runtimeModelSinkProfile()}, SourceDirectory: directory, Dockerfile: "Dockerfile", Port: 9090, Actor: row.Plan.Actor, MemoryMiB: 1024, CPUMillis: 1000, Network: "bridge", Platform: "linux/amd64"}
	request.ValidateSource = func(call context.Context) error {
		current := w.snapshot()
		if current.Plan.Resources.ModelSinkSourceSHA256 != runtimeModelSinkProfile() {
			return ErrRuntimePreparation
		}
		for name, expected := range map[string]string{"main.go": runtimeModelSinkSource, "Dockerfile": dockerfile} {
			raw, err := os.ReadFile(filepath.Join(directory, name))
			if err != nil || string(raw) != expected {
				return ErrRuntimePreparation
			}
		}
		return s.runtimeResourceMutationAuthority(call, current)
	}
	retain := func(out previewruntime.Outcome) error {
		record, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		return w.mutate(record, func(op *RuntimeResourceOperation) error {
			if out.ProjectID != row.Plan.ProjectID || out.ReleaseID != row.Plan.ID+"-model-sink" || out.Target != "native-docker" || out.Commit != op.Source.Commit || out.SourceDigest != runtimeModelSinkProfile() || op.HelperBuild != nil && !previewruntime.Continues(op.HelperBuild.Identity, out.Identity) {
				return ErrRuntimePreparation
			}
			if op.HelperBuild != nil && op.HelperBuild.ImmutableImage != "" && op.HelperBuild.ImmutableImage != out.ImmutableImage {
				return ErrRuntimePreparation
			}
			if out.ImmutableImage != "" && (!kubeImmutableImage.MatchString(out.ImmutableImage) || !runtimeImageInRegistry(row.Plan.Registry, out.ImmutableImage)) {
				return ErrRuntimePreparation
			}
			copy := out
			op.HelperBuild = &copy
			return nil
		})
	}
	out, callErr := runtime.BuildAndPublish(ctx, request, previewruntime.Publication{Repository: row.Plan.Registry.Repository, RegistryBinding: row.Plan.Registry.Fingerprint, RegistryAuth: credentials}, retain)
	current := w.snapshot()
	if current.Resources.HelperBuild == nil || reviewDigest(*current.Resources.HelperBuild) != reviewDigest(out) {
		if err = retain(out); err != nil {
			return err
		}
	}
	if callErr != nil {
		return callErr
	}
	_, err = runtimeHelperImage(w.snapshot())
	return err
}

func runtimeImageInRegistry(registry RuntimeRegistry, image string) bool {
	return len(image) > len(registry.Repository)+8 && image[:len(registry.Repository)+8] == registry.Repository+"@sha256:"
}

// A normal Observe never creates a Secret, PVC, helper, upload or replacement
// build. Positive original daemon/Kubernetes observations may settle receipts.
func (s *Server) ObserveRuntimeResourcePreparation(ctx context.Context, actor management.ExecutionActorRef, id string) (RuntimePreparation, error) {
	ctx = WithKubernetesTargetResolution(ctx)
	if err := s.kubernetesAuthority(ctx, actor, false, true, 0); err != nil {
		return RuntimePreparation{}, err
	}
	_, row, err := s.readRuntimePreparation(actor, id)
	if err != nil {
		return RuntimePreparation{}, err
	}
	if row.Plan.Resources == nil {
		return row.RuntimePreparation, nil
	}
	s.mu.RLock()
	live := s.coordinatorActions["runtime-resources-"+id]
	s.mu.RUnlock()
	if live {
		return row.RuntimePreparation, nil
	}
	if row.Resources == nil {
		return row.RuntimePreparation, ErrRuntimePreparation
	}
	w := &runtimeResourceWriter{server: s, row: row}
	if row.Resources.HelperBuildAttempted {
		if row.Resources.HelperBuild == nil || !row.Resources.HelperBuild.Identity.Retained() || s.PreviewRuntime().Observe == nil {
			return row.RuntimePreparation, ErrRuntimePreparation
		}
		original := row.Resources.HelperBuild
		out, callErr := s.PreviewRuntime().Observe(ctx, original.Identity)
		if callErr != nil {
			return row.RuntimePreparation, callErr
		}
		if !previewruntime.Continues(original.Identity, out.Identity) || original.ImmutableImage != "" && original.ImmutableImage != out.ImmutableImage {
			return row.RuntimePreparation, ErrRuntimePreparation
		}
		if err = w.mutate(ctx, func(op *RuntimeResourceOperation) error { op.HelperBuild = &out; return nil }); err != nil {
			return row.RuntimePreparation, err
		}
	}
	if err = s.observeRuntimeResourceObjects(ctx, w); err != nil {
		return s.runtimeResourceResult(ctx, actor, w, err)
	}
	return s.runtimeResourceResult(ctx, actor, w, nil)
}

func runtimePreparationPause(ctx context.Context) error {
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

var errRuntimeResourcePending = errors.New("The original resources are not yet confirmed; observe their retained identities without repeating effects")
