package product

import (
	"errors"
	"regexp"
	"strings"
)

const NativeDelegationCodexHelpers2 = "codex-native-helpers-2-v1"

var ErrNativeDelegation = errors.New("Native helpers require an explicitly approved local Codex Bypass task and its current immutable configured image. Review a new execution contract; ordinary tasks remain available without helpers.")
var nativeImageName = regexp.MustCompile(`^[a-z0-9]+([._-][a-z0-9]+)*(:[0-9]{1,5})?(/[a-z0-9]+([._-][a-z0-9]+)*)*$`)

// ImmutableNativeDelegationImage validates identity syntax, not image contents,
// CLI compatibility, current account access or readiness. The owned task's fixed
// image entry checks compatibility before invoking a model.
func ImmutableNativeDelegationImage(image string) bool {
	if len(image) > 330 { return false }
	name, digest, named := strings.Cut(image, "@")
	if !named { digest = image } else if len(name) > 255 || !nativeImageName.MatchString(name) { return false }
	return strings.HasPrefix(digest, "sha256:") && len(digest) == 71 && hexDigest(strings.TrimPrefix(digest, "sha256:"))
}

func validateNativeDelegationRequest(profile, harness, mode, location string) error {
	if profile == "" { return nil }
	if profile != NativeDelegationCodexHelpers2 || harness != "codex" || mode != "bypass" || location != "" && location != "local-docker" { return ErrNativeDelegation }
	return nil
}

// ValidateNativeDelegationRoute is also used by durable input validation. A
// legacy absent pair retains its historical bytes and does not gain a grant.
func ValidateNativeDelegationRoute(route ProjectRunRoute) error {
	if route.NativeDelegation == "" {
		if route.NativeDelegationImage != "" { return ErrNativeDelegation }
		return nil
	}
	if validateNativeDelegationRequest(route.NativeDelegation, route.Harness, route.PermissionMode, route.ExecutionLocation) != nil || !ImmutableNativeDelegationImage(route.NativeDelegationImage) { return ErrNativeDelegation }
	return nil
}

func ValidateNativeDelegationCurrent(route ProjectRunRoute, configuredImage string) error {
	if err := ValidateNativeDelegationRoute(route); err != nil { return err }
	if route.NativeDelegation != "" && route.NativeDelegationImage != configuredImage { return ErrNativeDelegation }
	return nil
}

func validateNativeDelegationInput(input ProjectRunInput) error {
	if err := ValidateNativeDelegationRoute(input.Route); err != nil { return err }
	if input.Route.NativeDelegation == "" { return nil }
	c := input.OrchestrationContract
	if c == nil || c.State != "approved" || c.Approval == nil || c.Digest != reviewDigest(c.Input) || c.Approval.Digest != c.Digest || c.Input.Route != input.Route { return ErrNativeDelegation }
	return nil
}

func (s *Server) captureNativeDelegation(profile string, route *ProjectRunRoute) error {
	if err := validateNativeDelegationRequest(profile, route.Harness, route.PermissionMode, route.ExecutionLocation); err != nil { return err }
	candidate := *route
	candidate.NativeDelegation, candidate.NativeDelegationImage = "", ""
	if profile != "" { candidate.NativeDelegation, candidate.NativeDelegationImage = profile, s.sandboxImage }
	if err := ValidateNativeDelegationCurrent(candidate, s.sandboxImage); err != nil { return err }
	*route = candidate
	return nil
}

// NativeDelegationForOperation derives a grant from the exact saved task and
// first operation, never a prompt or a request role. Reviewers and repair/retry
// operations retain the ordinary dual-disabled task mode.
func NativeDelegationForOperation(run ProjectRun, operationID, configuredImage string) (string, error) {
	if err := ValidateNativeDelegationCurrent(run.Input.Route, configuredImage); err != nil { return "", err }
	if run.Input.Route.NativeDelegation == "" { return "", nil }
	if err := validateNativeDelegationInput(run.Input); err != nil { return "", err }
	op := swarmOperation(&run, operationID)
	if op == nil || op.Kind != "session" || op.Status != "reserved" || op.Allocation != "ready" { return "", ErrNativeDelegation }
	task := swarmTask(&run, op.TaskID)
	if task == nil || task.OperationID != op.ID { return "", ErrNativeDelegation }
	matched := false
	for _, approved := range run.Input.OrchestrationContract.Input.Tasks {
		if approved.ID == task.ID && approved.Role == task.Role && approved.ReviewOfTaskID == task.ReviewOfTaskID { matched = true; break }
	}
	if !matched { return "", ErrNativeDelegation }
	index := -1
	for i := range run.Operations { if run.Operations[i].ID == op.ID { index = i; break } }
	if index < 0 || op.Attempt != index + 1 { return "", ErrNativeDelegation }
	if task.Role == "reviewer" && task.ReviewOnly { return "", nil }
	if task.Role != "coding" || task.ReviewOnly || task.ReviewOfTaskID != "" { return "", ErrNativeDelegation }
	if op.Checkpoint != nil { return "", nil }
	for _, prior := range run.Operations[:index] { if prior.TaskID == op.TaskID { return "", nil } }
	return run.Input.Route.NativeDelegation, nil
}
