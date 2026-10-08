package product

import (
	"context"
	"errors"
	"os/exec"
	"reflect"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

// installControlledProjectCheckRuntime substitutes only the typed execution
// boundary in product handler tests. It does not emulate the Docker adapter,
// shared machine admission, daemon ownership or hard resource enforcement.
// Native command fixtures supply a real bounded RunCommand and retain actual
// output/exit codes. A missing executor fails the test instead of inventing a
// passing command. Production source capture, approvals and receipt publication
// remain untouched.
func installControlledProjectCheckRuntime(t *testing.T, s *Server, run func(context.Context, ProjectCheckCommandRequest) (string, error)) {
	t.Helper()
	s.SetProjectCheckRuntime(ProjectCheckRuntime{
		CaptureActor: management.CaptureExecutionActor,
		AuthorizeControl: func(ctx context.Context, projectID string) error {
			if projectID == "" {
				return errors.New("controlled check fixture requires a project")
			}
			return ctx.Err()
		},
		Execute: func(ctx context.Context, req ProjectCheckCommandRequest, persist func(ProjectCheckCommandOutcome) error) (ProjectCheckCommandOutcome, error) {
			if run == nil {
				t.Errorf("unexpected check execution without an explicit controlled fixture: %s/%d", req.CheckID, req.CommandIndex)
				return ProjectCheckCommandOutcome{}, errors.New("controlled check executor was not configured")
			}
			if err := validateControlledCheckRequest(s, req); err != nil {
				t.Errorf("typed check request lost its saved identity: %v", err)
				return ProjectCheckCommandOutcome{}, err
			}
			output, err := run(ctx, req)
			out := ProjectCheckCommandOutcome{
				AllocationID: req.AllocationID, Stage: "controlled-process",
				Status: "unknown", Output: output, ResourcesHeld: true,
				Message: "Controlled product fixture; no Docker or resource-enforcement claim.",
			}
			if len(out.Output) > maxCheckOutput {
				out.Output, out.Truncated = out.Output[:maxCheckOutput], true
			}
			var exit *exec.ExitError
			switch {
			case ctx.Err() != nil:
				// The controlled runner has returned (RunCommand waits/reaps its
				// process). Cancellation records no invented numerical exit code.
				out.Status, out.CleanupConfirmed, out.ResourcesHeld = "cancelled", true, false
			case err == nil:
				zero := 0
				out.Status, out.ExitCode, out.CleanupConfirmed, out.ResourcesHeld = "passed", &zero, true, false
			case errors.As(err, &exit) && exit.ExitCode() >= 0:
				code := exit.ExitCode()
				out.Status, out.ExitCode, out.CleanupConfirmed, out.ResourcesHeld = "failed", &code, true, false
			default:
				// A transport/start error is not proof of completion or cleanup.
				out.ReasonCode = "controlled_process_unconfirmed"
			}
			if err := persist(out); err != nil {
				return out, err
			}
			return out, nil
		},
	})
}

func validateControlledCheckRequest(s *Server, req ProjectCheckCommandRequest) error {
	row, err := s.GetProjectCheckRun(req.ProjectID, req.CheckID)
	if err != nil {
		return err
	}
	owner, err := s.resolveProject(req.ProjectID)
	if err != nil {
		return err
	}
	if req.ProjectID == "" || req.CheckID == "" || req.AllocationID == "" || req.CommandIndex < 0 || req.CommandIndex >= len(row.Commands) ||
		req.Actor.ProjectID != req.ProjectID || req.SourceDirectory != owner.Path || !hexDigest(req.SourceDigest) ||
		row.SourceDigest != req.SourceDigest || row.MemoryMiB != req.MemoryMiB || row.CPUMillis != req.CPUMillis || req.MemoryMiB <= 0 || req.CPUMillis <= 0 ||
		row.Commands[req.CommandIndex].AllocationID != req.AllocationID || !reflect.DeepEqual(row.Commands[req.CommandIndex].CheckCommand, req.Command) {
		return errors.New("command does not match its exact private check receipt")
	}
	return nil
}
