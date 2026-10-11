package product

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

const orchestratorTurnLimit = 200
const orchestratorFileLimit = 16 << 20
const orchestratorContextLimit = 224 << 10
const orchestratorAnswerLimit = 384 << 10

type storedOrchestratorTurn struct {
	OrchestratorTurn
	AdmissionRevision   *int   `json:"admissionRevision,omitempty"`
	KeyDigest           string `json:"keyDigest"`
	RequestDigest       string `json:"requestDigest"`
	ApplyIntent         string `json:"applyIntent,omitempty"`
	ApplyTargetRevision int    `json:"applyTargetRevision,omitempty"`
	ApplyTargetDigest   string `json:"applyTargetDigest,omitempty"`
}
type orchestratorLedger struct {
	Version       int                      `json:"version"`
	ArchiveDigest string                   `json:"archiveDigest,omitempty"`
	AccountScope  string                   `json:"accountScope"`
	ProjectScope  string                   `json:"projectScope"`
	ProjectID     string                   `json:"projectId"`
	Revision      int                      `json:"revision"`
	Turns         []storedOrchestratorTurn `json:"turns"`
	NoDispatch    []orchestratorNoDispatch `json:"noDispatch,omitempty"`
}

func (s *Server) orchestratorPath(id string, create bool) (string, error) {
	if !prototypeReleaseID.MatchString(id) {
		return "", ErrOrchestratorInvalid
	}
	root, err := guardedProjectPath(s.stateDir, "orchestrator", true)
	if err != nil {
		return "", ErrOrchestratorInvalid
	}
	if create {
		if err = os.Mkdir(root, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return "", ErrOrchestratorInvalid
		}
	}
	if _, err = guardedProjectPath(s.stateDir, "orchestrator", false); err != nil {
		return "", err
	}
	if create {
		if runtime.GOOS == "windows" {
			err = restrictPrivateFile(root)
		} else {
			err = os.Chmod(root, 0700)
		}
		if err != nil {
			return "", ErrOrchestratorInvalid
		}
	}
	return guardedProjectPath(s.stateDir, "orchestrator/"+id+".json", create)
}
func (s *Server) readOrchestrator(p project.Project) (orchestratorLedger, error) {
	scope, err := s.approvalScopes(p.Path, p.ID)
	out := orchestratorLedger{Version: 1, AccountScope: scope.AccountScope, ProjectScope: scope.ProjectScope, ProjectID: p.ID, Turns: []storedOrchestratorTurn{}}
	if err != nil {
		return out, ErrOrchestratorInvalid
	}
	path, err := s.orchestratorPath(p.ID, false)
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return out, ErrOrchestratorInvalid
	}
	if err = hostpath.OneName(path); errors.Is(err, os.ErrNotExist) {
		return out, nil
	} else if err != nil {
		return out, ErrOrchestratorInvalid
	}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return out, ErrOrchestratorInvalid
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > orchestratorFileLimit {
		return out, ErrOrchestratorInvalid
	}
	data, err := io.ReadAll(io.LimitReader(f, orchestratorFileLimit+1))
	if err != nil || len(data) > orchestratorFileLimit {
		return out, ErrOrchestratorInvalid
	}
	return decodeOrchestratorLedger(data, out)
}

func decodeOrchestratorLedger(data []byte, out orchestratorLedger) (orchestratorLedger, error) {
	var saved orchestratorLedger
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&saved) != nil || decoder.Decode(new(any)) != io.EOF || saved.Version != 1 || saved.AccountScope != out.AccountScope || saved.ProjectScope != out.ProjectScope || saved.ProjectID != out.ProjectID || saved.Revision < 0 || (saved.ArchiveDigest != "" && !hexDigest(saved.ArchiveDigest)) || len(saved.Turns) > orchestratorTurnLimit {
		return out, ErrOrchestratorInvalid
	}
	seen := map[string]bool{}
	if validateOrchestratorNoDispatch(saved.NoDispatch) != nil {
		return out, ErrOrchestratorInvalid
	}
	previousSequence := 0
	for _, turn := range saved.Turns {
		if turn.AdmissionRevision != nil && (*turn.AdmissionRevision < 0 || *turn.AdmissionRevision >= saved.Revision) {
			return out, ErrOrchestratorInvalid
		}
		if turn.ProjectID != out.ProjectID || !strings.HasPrefix(turn.ID, "turn-") || !prototypeReleaseID.MatchString(turn.ID) || !prototypeReleaseID.MatchString(turn.JobID) || turn.Sequence <= previousSequence || seen[turn.ID] || !hexDigest(turn.ContextDigest) || turn.Context.Digest != turn.ContextDigest || turn.Context.ProjectID != out.ProjectID || !hexDigest(turn.KeyDigest) || !hexDigest(turn.RequestDigest) || len(turn.Message) == 0 || len(turn.Message) > 16000 || !validConnectionToken(turn.Actor.ID, 128, false) || turn.Actor.DisplayName == "" || !validOrchestratorStatus(turn.Status) {
			return out, ErrOrchestratorInvalid
		}
		if _, err := time.Parse(time.RFC3339Nano, turn.CreatedAt); err != nil {
			return out, ErrOrchestratorInvalid
		}
		if turn.Answer != nil && ValidateOrchestratorAnswer(turn.Context, *turn.Answer) != nil {
			return out, ErrOrchestratorInvalid
		}
		seen[turn.ID] = true
		previousSequence = turn.Sequence
	}
	return saved, nil
}
func validOrchestratorStatus(status string) bool {
	switch status {
	case "reserved", "queued", "running", "succeeded", "failed", "cancelled", "unknown":
		return true
	}
	return false
}
func terminalOrchestrator(status string) bool {
	return status == "succeeded" || status == "failed" || status == "cancelled" || status == "unknown"
}
func (s *Server) saveOrchestrator(saved orchestratorLedger) error {
	if err := validateOrchestratorNoDispatch(saved.NoDispatch); err != nil {
		return err
	}
	data, err := json.MarshalIndent(saved, "", "  ")
	if err != nil || len(data)+1 > orchestratorFileLimit || len(saved.Turns) > orchestratorTurnLimit {
		return ErrOrchestratorCapacity
	}
	path, err := s.orchestratorPath(saved.ProjectID, true)
	if err != nil {
		return ErrOrchestratorInvalid
	}
	if _, err := os.Lstat(path); err == nil {
		if hostpath.OneName(path) != nil {
			return ErrOrchestratorInvalid
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return ErrOrchestratorInvalid
	}
	if err = atomicJSON(path, saved); err != nil {
		return ErrOrchestratorInvalid
	}
	if err = syncProjectRunDirectory(s.stateDir); err != nil {
		return ErrOrchestratorInvalid
	}
	if err = syncProjectRunDirectory(filepath.Dir(path)); err != nil {
		return ErrOrchestratorInvalid
	}
	// Secondary overview index only; missing/stale projections stay explicitly
	// partial and never change the canonical turn's admission or outcome.
	_ = s.writeOverviewQuestions(saved, path)
	return nil
}

// Caller holds source admission and s.mu. This snapshot is an input, never
// authority to run tools or a claim that cached checks/releases are current.
func (s *Server) orchestratorSnapshotLocked(ctx context.Context, p project.Project) (OrchestratorContext, error) {
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		return OrchestratorContext{}, ErrOrchestratorInvalid
	}
	value := OrchestratorContext{ProjectID: p.ID, Brief: state.Brief, Specification: currentSpecification(state), DesignPlan: currentDesignPlan(state), Plan: currentPlan(state, p.ID), SketchRevision: state.Design.Revision, ChecksSummary: "No recorded verification is available.", ReleasesSummary: "No recorded release is available."}
	normalizeOrchestratorCollections(&value)
	value.Digest = reviewDigest(struct {
		Brief  Brief
		Spec   ProjectSpecification
		Design ProjectDesignPlan
		Plan   ProjectPlan
		Sketch Design
	}{value.Brief, value.Specification, value.DesignPlan, value.Plan, state.Design})
	readCtx, cancel := context.WithTimeout(context.WithValue(ctx, journeyGitRootContextKey{}, s.config.ProjectsDir), 3*time.Second)
	defer cancel()
	head, headErr := s.journeyGit(readCtx, p.Path, "rev-parse", "--verify", "HEAD")
	status, statusErr := s.journeyGit(readCtx, p.Path, "status", "--porcelain=v1", "--untracked-files=normal")
	if statusErr == nil {
		value.SourceKnown = true
		value.SourceDirty = strings.TrimSpace(status) != ""
	}
	if headErr == nil && deliverySHA.MatchString(strings.TrimSpace(head)) {
		value.SourceCommit = strings.TrimSpace(head)
	}
	if checks, err := s.readCheckHistory(p.ID); err == nil && len(checks) > 0 {
		last := checks[0]
		value.ChecksSummary = fmt.Sprintf("%d recorded check runs; last stored status %s at %s. Reconcile exact-source evidence before treating it as current.", len(checks), last.Status, last.UpdatedAt)
	}
	if releases, err := s.readReleases(p.ID); err == nil && len(releases) > 0 {
		last := releases[0]
		value.ReleasesSummary = fmt.Sprintf("%d recorded releases; last stored status %s at %s. A historical receipt does not prove current health.", len(releases), last.Status, last.CreatedAt)
	}
	if s.projectRuns != nil {
		runs, err := s.projectRuns.List(p.ID, p.Path)
		if err != nil {
			return value, ErrOrchestratorInvalid
		}
		value.TaskHandoffs = s.projectRuns.orchestratorHandoffs(runs)
		handoff, _ := json.MarshalIndent(value.TaskHandoffs, "", "  ")
		value.CurrentRun, value.ActiveRuns = orchestratorRunProjectionWithHandoffs(runs, state, len(handoff))
		value.NeedsAttention = projectRunAttentionProjection(runs, state)
	}
	return value, nil
}
func (s *Server) OrchestratorSnapshot(ctx context.Context, id string) (OrchestratorContext, error) {
	release, err := s.AcquireGitSessionAdmission(ctx, id)
	if err != nil {
		return OrchestratorContext{}, err
	}
	defer release()
	p, err := s.resolveProject(id)
	if err != nil {
		return OrchestratorContext{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.orchestratorSnapshotLocked(ctx, p)
}
func (s *Server) ReserveOrchestratorTurn(ctx context.Context, id string, req OrchestratorTurnRequest, jobID string) (OrchestratorTurn, OrchestratorContext, bool, error) {
	actor, ok := authenticatedContextActor(ctx)
	if !ok {
		return OrchestratorTurn{}, OrchestratorContext{}, false, ErrOrchestratorInvalid
	}
	req.Message = strings.TrimSpace(req.Message)
	if !validRunKey(req.IdempotencyKey) || !hexDigest(req.ExpectedContextDigest) || req.ExpectedRevision < 0 || len(req.Message) == 0 || len(req.Message) > 16000 || !strings.HasPrefix(jobID, "assist-") || !prototypeReleaseID.MatchString(jobID) || !hexDigest(req.RouteDigest) {
		return OrchestratorTurn{}, OrchestratorContext{}, false, ErrOrchestratorInvalid
	}
	release, err := s.AcquireGitSessionAdmission(ctx, id)
	if err != nil {
		return OrchestratorTurn{}, OrchestratorContext{}, false, err
	}
	defer release()
	p, err := s.resolveProject(id)
	if err != nil {
		return OrchestratorTurn{}, OrchestratorContext{}, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	saved, err := s.readOrchestrator(p)
	if err != nil {
		return OrchestratorTurn{}, OrchestratorContext{}, false, err
	}
	key := reviewDigest(actor.ID + ":" + req.IdempotencyKey)
	digest := reviewDigest([]any{actor.ID, req.Message, req.ReplyToTurnID, req.ExpectedContextDigest, req.RouteDigest})
	storage, err := s.orchestratorStorageLocked(p, saved)
	if err != nil {
		return OrchestratorTurn{}, OrchestratorContext{}, false, err
	}
	for _, fence := range saved.NoDispatch {
		if fence.KeyDigest == key {
			return OrchestratorTurn{}, OrchestratorContext{}, false, ErrOrchestratorNotSent
		}
	}
	previous, err := findOrchestratorRequest(storage, key, digest)
	if err == nil && previous == nil {
		previous, err = s.archivedCoordinationRequestLocked(p, key, digest)
	}
	if err != nil {
		return OrchestratorTurn{}, OrchestratorContext{}, false, err
	}
	if previous != nil {
		return previous.OrchestratorTurn, previous.Context, true, nil
	}
	// Bound the set that must survive rotation, while always allowing an explicit answer.
	if len(orchestratorQuestions(saved)) >= 16 && req.ReplyToTurnID == "" {
		return OrchestratorTurn{}, OrchestratorContext{}, false, ErrOrchestratorQuestion
	}
	if saved.Revision != req.ExpectedRevision {
		return OrchestratorTurn{}, OrchestratorContext{}, false, ErrOrchestratorConflict
	}
	for _, turn := range saved.Turns {
		if !terminalOrchestrator(turn.Status) {
			return OrchestratorTurn{}, OrchestratorContext{}, false, ErrOrchestratorBusy
		}
	}
	snapshot, err := s.orchestratorSnapshotLocked(ctx, p)
	if err != nil {
		return OrchestratorTurn{}, snapshot, false, err
	}
	if snapshot.Digest != req.ExpectedContextDigest {
		return OrchestratorTurn{}, snapshot, false, ErrOrchestratorConflict
	}
	raw, _ := json.MarshalIndent(snapshot, "", "  ")
	if len(raw) > orchestratorContextLimit {
		return OrchestratorTurn{}, snapshot, false, errors.New("The saved context and unfinished build summaries exceed the assistant input bound. Inspect or reconcile existing builds, cancel unused reservations, or focus the saved phase context before asking Luci.")
	}
	if req.ReplyToTurnID != "" {
		found := false
		for _, turn := range saved.Turns {
			if turn.ID == req.ReplyToTurnID && turn.Answer != nil && turn.Answer.Question != "" && turn.QuestionResolvedBy == "" {
				found = true
			}
		}
		if !found {
			return OrchestratorTurn{}, snapshot, false, ErrOrchestratorConflict
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	turn := OrchestratorTurn{ID: newID("turn-"), ProjectID: id, Sequence: nextOrchestratorSequence(saved), Actor: actor, Message: req.Message, JobID: jobID, ReplyToTurnID: req.ReplyToTurnID, ContextDigest: snapshot.Digest, Context: snapshot, Status: "reserved", CreatedAt: now, UpdatedAt: now}
	saved.Turns = append(saved.Turns, storedOrchestratorTurn{OrchestratorTurn: turn, AdmissionRevision: &req.ExpectedRevision, KeyDigest: key, RequestDigest: digest})
	saved.Revision++
	// Reserve room for the bounded terminal answer before model work starts.
	encoded, marshalErr := json.MarshalIndent(saved, "", "  ")
	if marshalErr != nil || len(encoded)+orchestratorAnswerLimit+(64<<10) > orchestratorFileLimit {
		return OrchestratorTurn{}, snapshot, false, ErrOrchestratorCapacity
	}
	if err = s.saveOrchestrator(saved); err != nil {
		return OrchestratorTurn{}, snapshot, false, err
	}
	return turn, snapshot, false, nil
}

// The exclusive source-admission lock serializes publishing a new blocking
// question with the final admission/creation of a coordinated coding session.
func (s *Server) CompleteOrchestratorTurn(id, turnID, jobID, status string, answer *OrchestratorAnswer) error {
	if !terminalOrchestrator(status) {
		return ErrOrchestratorInvalid
	}
	p, err := s.resolveProject(id)
	if err != nil {
		return err
	}
	guard := s.branchGuard(id)
	guard.Lock()
	defer guard.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	saved, err := s.readOrchestrator(p)
	if err != nil {
		return err
	}
	for i := range saved.Turns {
		turn := &saved.Turns[i]
		if turn.ID != turnID {
			continue
		}
		if turn.JobID != jobID {
			return ErrOrchestratorInvalid
		}
		if status == "succeeded" {
			if answer == nil || ValidateOrchestratorAnswer(turn.Context, *answer) != nil {
				return ErrOrchestratorInvalid
			}
		} else if answer != nil {
			return ErrOrchestratorInvalid
		}
		if terminalOrchestrator(turn.Status) {
			if turn.Status == status && reviewDigest(turn.Answer) == reviewDigest(answer) {
				return s.recordCoordinationProposalLocked(p, *turn)
			}
			return ErrOrchestratorConflict
		}
		turn.Status, turn.Answer, turn.UpdatedAt = status, cloneOrchestratorAnswer(answer), time.Now().UTC().Format(time.RFC3339Nano)
		if status == "succeeded" && turn.ReplyToTurnID != "" {
			for j := range saved.Turns {
				if saved.Turns[j].ID == turn.ReplyToTurnID {
					saved.Turns[j].QuestionResolvedBy = turn.ID
				}
			}
		}
		saved.Revision++
		if err := s.saveOrchestrator(saved); err != nil {
			return err
		}
		return s.recordCoordinationProposalLocked(p, *turn)
	}
	return ErrOrchestratorInvalid
}
func (s *Server) OrchestratorHistory(id string) ([]OrchestratorTurn, error) {
	p, err := s.resolveProject(id)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	saved, err := s.readOrchestrator(p)
	if err != nil {
		return nil, err
	}
	start := len(saved.Turns) - 8
	if start < 0 {
		start = 0
	}
	out := []OrchestratorTurn{}
	for _, turn := range saved.Turns[start:] {
		out = append(out, turn.OrchestratorTurn)
	}
	return out, nil
}
func (s *Server) OrchestratorPendingTurns() ([]OrchestratorTurn, error) {
	path, err := guardedProjectPath(s.stateDir, "orchestrator", false)
	if errors.Is(err, os.ErrNotExist) {
		return []OrchestratorTurn{}, nil
	}
	if err != nil {
		return nil, ErrOrchestratorInvalid
	}
	folder, err := os.Open(path)
	if err != nil {
		return nil, ErrOrchestratorInvalid
	}
	defer folder.Close()
	entries, err := folder.ReadDir(1025)
	if err != nil && !errors.Is(err, io.EOF) || len(entries) > 1024 {
		return nil, ErrOrchestratorCapacity
	}
	out := []OrchestratorTurn{}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		p, err := s.resolveProject(id)
		if err != nil {
			continue
		}
		s.mu.RLock()
		saved, err := s.readOrchestrator(p)
		s.mu.RUnlock()
		if err != nil {
			return nil, err
		}
		for _, turn := range saved.Turns {
			if !terminalOrchestrator(turn.Status) {
				out = append(out, turn.OrchestratorTurn)
			}
		}
	}
	return out, nil
}
func orchestratorQuestions(saved orchestratorLedger) []OrchestratorQuestion {
	out := []OrchestratorQuestion{}
	for _, turn := range saved.Turns {
		if turn.Status == "succeeded" && turn.Answer != nil && turn.Answer.Blocking && turn.Answer.Question != "" && turn.QuestionResolvedBy == "" {
			out = append(out, OrchestratorQuestion{TurnID: turn.ID, Question: turn.Answer.Question, Rationale: turn.Answer.Rationale, Options: turn.Answer.Options})
		}
	}
	return out
}
func (s *Server) OrchestratorExecutionGate(id string) error {
	p, err := s.resolveProject(id)
	if err != nil {
		return err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	saved, err := s.readOrchestrator(p)
	if err != nil {
		return err
	}
	if len(orchestratorQuestions(saved)) > 0 {
		return ErrOrchestratorQuestion
	}
	return nil
}

func nonnilOrchestrator[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}
func normalizeOrchestratorCollections(value *OrchestratorContext) {
	value.Brief.Criteria = nonnilOrchestrator(value.Brief.Criteria)
	p := &value.Specification
	p.OutOfScope = nonnilOrchestrator(p.OutOfScope)
	p.Requirements = nonnilOrchestrator(p.Requirements)
	p.Journeys = nonnilOrchestrator(p.Journeys)
	p.Roles = nonnilOrchestrator(p.Roles)
	p.Entities = nonnilOrchestrator(p.Entities)
	p.Integrations = nonnilOrchestrator(p.Integrations)
	p.Glossary = nonnilOrchestrator(p.Glossary)
	p.Decisions = nonnilOrchestrator(p.Decisions)
	p.Questions = nonnilOrchestrator(p.Questions)
	value.DesignPlan.Screens = nonnilOrchestrator(value.DesignPlan.Screens)
	value.DesignPlan.Tokens = nonnilOrchestrator(value.DesignPlan.Tokens)
	value.DesignPlan.Assets = nonnilOrchestrator(value.DesignPlan.Assets)
	value.Plan.Tasks = nonnilOrchestrator(value.Plan.Tasks)
	value.ActiveRuns = nonnilOrchestrator(value.ActiveRuns)
}

func nextOrchestratorSequence(saved orchestratorLedger) int {
	if len(saved.Turns) == 0 {
		return 1
	}
	return saved.Turns[len(saved.Turns)-1].Sequence + 1
}
