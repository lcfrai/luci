package product

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

type CoordinationConflict struct {
	IntentID      string   `json:"intentId"`
	OtherIntentID string   `json:"otherIntentId,omitempty"`
	Paths         []string `json:"paths"`
	Reason        string   `json:"reason"`
}

type CoordinationView struct {
	ProjectID      string                 `json:"projectId"`
	Revision       int                    `json:"revision"`
	ContextDigest  string                 `json:"contextDigest"`
	Intents        []CoordinationIntent   `json:"intents"`
	Events         []CoordinationEvent    `json:"events"`
	Conflicts      []CoordinationConflict `json:"conflicts"`
	AllowedActions []string               `json:"allowedActions"`
}

func (s *Server) registerCoordination(mux Registrar) {
	if wrapped, ok := mux.(*branchSourceRegistrar); ok {
		mux = wrapped.base
	}
	mux.HandleFunc("GET /v1/product/projects/{id}/orchestrator/coordination", s.getCoordination)
	mux.HandleFunc("POST /v1/product/projects/{id}/orchestrator/coordination/intents", s.createCoordinationIntent)
	mux.HandleFunc("POST /v1/product/projects/{id}/orchestrator/coordination/intents/{intentId}/resolve", s.resolveCoordinationIntent)
}

func coordinationTurn(saved orchestratorLedger, id string) *storedOrchestratorTurn {
	for i := range saved.Turns {
		if saved.Turns[i].ID == id {
			return &saved.Turns[i]
		}
	}
	return nil
}
func coordinationIntent(saved *coordinationLedger, turn string) *storedCoordinationIntent {
	for i := range saved.Intents {
		if saved.Intents[i].TurnID == turn {
			return &saved.Intents[i]
		}
	}
	return nil
}
func coordinationChanges(turn *storedOrchestratorTurn) coordinationCells {
	if turn == nil || turn.Answer == nil || turn.Answer.Proposal == nil {
		return nil
	}
	target := turn.Answer.Proposal.Target
	return coordinationDiff(coordinationObject(coordinationDocument(turn.Context, nil, target)), coordinationObject(coordinationDocument(turn.Context, turn.Answer.Proposal, target)))
}

// Completion captures an origin automatically, without a second user handoff.
// The model job was already admitted with this authenticated stored actor. This
// records its proposal only; current edit/policy authority is checked at apply.
// Legacy embeddings without the coordinator authorizer retain their old API.
func (s *Server) recordCoordinationProposalLocked(p project.Project, turn storedOrchestratorTurn) error {
	if s.coordinatorAuthorize == nil || turn.Status != "succeeded" || turn.Answer == nil || turn.Answer.Proposal == nil {
		return nil
	}
	if ValidateOrchestratorAnswer(turn.Context, *turn.Answer) != nil {
		return ErrOrchestratorInvalid
	}
	saved, err := s.readCoordination(p)
	if err != nil {
		return err
	}
	if existing := coordinationIntent(&saved, turn.ID); existing != nil {
		if existing.ProposalDigest == reviewDigest(turn.Answer.Proposal) && existing.Actor.ID == turn.Actor.ID && existing.BaseContextDigest == turn.ContextDigest {
			return nil
		}
		return ErrOrchestratorConflict
	}
	appendCoordinationOrigin(&saved, turn)
	return s.saveCoordination(saved)
}

func appendCoordinationOrigin(saved *coordinationLedger, turn storedOrchestratorTurn) {
	requirements, tasks, all := coordinationImpact(turn)
	intent := storedCoordinationIntent{CoordinationIntent: CoordinationIntent{ID: newID("intent-"), TurnID: turn.ID, Actor: turn.Actor, LuciID: coordinationLuciID(saved.AccountScope, turn.Actor.ID), BaseContextDigest: turn.ContextDigest, ProposalDigest: reviewDigest(turn.Answer.Proposal), Target: turn.Answer.Proposal.Target, RequestedChange: turn.Message, ChangedPaths: coordinationPaths(coordinationChanges(&turn)), RequirementIDs: requirements, TaskIDs: tasks, AffectsAllTasks: all, DependsOn: []string{}, Status: "pending", CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}, Key: reviewDigest([]string{"completed-proposal", turn.Actor.ID, turn.ID}), Request: reviewDigest([]any{turn.Actor.ID, turn.ID, []string{}})}
	saved.Intents = append(saved.Intents, intent)
	coordinationEvent(saved, intent.CoordinationIntent, "proposed", turn.ContextDigest, turn.Actor)
}

func (s *Server) ensureCoordinationOriginsLocked(p project.Project) error {
	saved, err := s.readCoordination(p)
	if err != nil {
		return err
	}
	turns, err := s.readOrchestrator(p)
	if err != nil {
		return err
	}
	changed := false
	for _, turn := range turns.Turns {
		if turn.Status != "succeeded" || turn.Answer == nil || turn.Answer.Proposal == nil || turn.AppliedRevision > 0 || coordinationIntent(&saved, turn.ID) != nil {
			continue
		}
		if ValidateOrchestratorAnswer(turn.Context, *turn.Answer) != nil {
			return ErrOrchestratorInvalid
		}
		appendCoordinationOrigin(&saved, turn)
		changed = true
	}
	// One bounded durable write restores all retained pending proposals, so a
	// prior failed origin write cannot hide an opposing person's known intent.
	if changed {
		return s.saveCoordination(saved)
	}
	return nil
}

func coordinationImpact(turn storedOrchestratorTurn) (requirements, tasks []string, all bool) {
	reqs, selected := map[string]bool{}, map[string]bool{}
	changes := coordinationChanges(&turn)
	target := turn.Answer.Proposal.Target
	for path := range changes {
		parts := strings.SplitN(path, "/", 2)
		if len(parts) == 2 && parts[0] == "requirements" {
			reqs[parts[1]] = true
			continue
		}
		if len(parts) == 2 && parts[0] == "tasks" {
			selected[parts[1]] = true
			continue
		}
		if len(parts) == 2 && parts[0] == "screens" {
			for _, value := range []ProjectDesignPlan{turn.Context.DesignPlan, *turn.Answer.Proposal.DesignPlan} {
				for _, screen := range value.Screens {
					if screen.ID == parts[1] {
						for _, id := range screen.RequirementIDs {
							reqs[id] = true
						}
					}
				}
			}
			continue
		}
		all = true
	}
	planTasks := append([]ProjectTask{}, turn.Context.Plan.Tasks...)
	if target == "plan" {
		planTasks = append(planTasks, turn.Answer.Proposal.Plan.Tasks...)
	}
	for _, task := range planTasks {
		for _, id := range task.RequirementIDs {
			if reqs[id] {
				selected[task.ID] = true
			}
		}
		if selected[task.ID] {
			for _, id := range task.RequirementIDs {
				reqs[id] = true
			}
		}
	}
	// Bounded transitive dependency closure. It marks affected scope; it never
	// cancels a worker or releases a dependency merely because an intent exists.
	for n := 0; n < len(planTasks); n++ {
		changed := false
		for _, task := range planTasks {
			for _, dep := range task.DependsOn {
				if selected[dep] && !selected[task.ID] {
					selected[task.ID] = true
					changed = true
				}
			}
		}
		if !changed {
			break
		}
	}
	requirements, tasks = []string{}, []string{}
	for id := range reqs {
		requirements = append(requirements, id)
	}
	for id := range selected {
		tasks = append(tasks, id)
	}
	sort.Strings(requirements)
	sort.Strings(tasks)
	if len(tasks) > 200 {
		tasks = []string{}
		all = true
	}
	return
}

func coordinationConflicts(saved coordinationLedger, turns orchestratorLedger, current OrchestratorContext) []CoordinationConflict {
	out := []CoordinationConflict{}
	for i, intent := range saved.Intents {
		if intent.Status != "pending" {
			continue
		}
		turn := coordinationTurn(turns, intent.TurnID)
		if turn == nil || turn.Answer == nil || turn.Answer.Proposal == nil || reviewDigest(turn.Answer.Proposal) != intent.ProposalDigest {
			out = append(out, CoordinationConflict{IntentID: intent.ID, Paths: []string{}, Reason: "origin-unavailable"})
			continue
		}
		_, paths := coordinationRebase(*turn, current)
		if len(paths) > 0 {
			out = append(out, CoordinationConflict{IntentID: intent.ID, Paths: paths, Reason: "current-context-conflict"})
		} else if merged, _ := coordinationRebase(*turn, current); ValidateOrchestratorAnswer(current, merged) != nil {
			out = append(out, CoordinationConflict{IntentID: intent.ID, Paths: []string{}, Reason: "merged-context-invalid"})
		}
		for _, dep := range intent.DependsOn {
			satisfied := false
			for _, peer := range saved.Intents {
				if peer.ID == dep && peer.Status == "applied" {
					satisfied = true
				}
			}
			if !satisfied {
				out = append(out, CoordinationConflict{IntentID: intent.ID, OtherIntentID: dep, Paths: []string{}, Reason: "dependency-unresolved"})
			}
		}
		for j := i + 1; j < len(saved.Intents); j++ {
			peer := saved.Intents[j]
			if peer.Status != "pending" || peer.Target != intent.Target {
				continue
			}
			other := coordinationTurn(turns, peer.TurnID)
			if other == nil || other.Answer == nil || other.Answer.Proposal == nil || reviewDigest(other.Answer.Proposal) != peer.ProposalDigest {
				out = append(out, CoordinationConflict{IntentID: intent.ID, OtherIntentID: peer.ID, Paths: []string{}, Reason: "origin-unavailable"})
				continue
			}
			paths := coordinationOverlaps(coordinationChanges(turn), coordinationChanges(other))
			if len(paths) > 0 {
				out = append(out, CoordinationConflict{IntentID: intent.ID, OtherIntentID: peer.ID, Paths: paths, Reason: "opposing-intent"})
			}
		}
	}
	return out
}

func (s *Server) coordinationViewLocked(ctx context.Context, p project.Project) (CoordinationView, error) {
	saved, err := s.readCoordination(p)
	if err != nil {
		return CoordinationView{}, err
	}
	turns, err := s.readOrchestrator(p)
	if err != nil {
		return CoordinationView{}, err
	}
	snapshot, err := s.orchestratorSnapshotLocked(ctx, p)
	if err != nil {
		return CoordinationView{}, err
	}
	view := CoordinationView{ProjectID: p.ID, Revision: saved.Revision, ContextDigest: snapshot.Digest, Intents: []CoordinationIntent{}, Events: saved.Events, Conflicts: coordinationConflicts(saved, turns, snapshot), AllowedActions: []string{}}
	for _, intent := range saved.Intents {
		view.Intents = append(view.Intents, intent.CoordinationIntent)
	}
	if s.coordinatorAuthority(ctx, p.ID) == nil {
		view.AllowedActions = []string{"propose-own-turn", "supersede-own-intent"}
	}
	return view, nil
}

// CoordinationViewForProject is a read-only continuation input. Accepted events
// identify affected work but do not confer execution, review or deploy authority.
// Consumers must persist their event cursor/idempotency key and use current typed
// operation admission; reading/reopening never dispatches or rewrites context.
func (s *Server) CoordinationViewForProject(ctx context.Context, id string) (CoordinationView, error) {
	if _, ok := authenticatedContextActor(ctx); !ok {
		return CoordinationView{}, errCoordinatorAuthority
	}
	release, err := s.AcquireGitSessionAdmission(ctx, id)
	if err != nil {
		return CoordinationView{}, err
	}
	defer release()
	p, err := s.resolveProject(id)
	if err != nil {
		return CoordinationView{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.coordinationViewLocked(ctx, p)
}
func (s *Server) getCoordination(w http.ResponseWriter, r *http.Request) {
	view, err := s.CoordinationViewForProject(r.Context(), r.PathValue("id"))
	if err != nil {
		coordinatorPolicyProblem(w, err)
		return
	}
	journeyJSON(w, 200, view)
}

func (s *Server) createCoordinationIntent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ExpectedRevision      int      `json:"expectedRevision"`
		ExpectedContextDigest string   `json:"expectedContextDigest"`
		TurnID                string   `json:"turnId"`
		IdempotencyKey        string   `json:"idempotencyKey"`
		DependsOn             []string `json:"dependsOn"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	if req.ExpectedRevision < 0 || !hexDigest(req.ExpectedContextDigest) || !validRunKey(req.IdempotencyKey) || len(req.DependsOn) > 16 {
		orchestratorProblem(w, ErrOrchestratorInvalid)
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	if err := s.coordinatorAuthority(r.Context(), p.ID); err != nil {
		coordinatorPolicyProblem(w, err)
		return
	}
	release, err := s.AcquireGitSessionAdmission(r.Context(), p.ID)
	if err != nil {
		orchestratorProblem(w, err)
		return
	}
	defer release()
	s.mu.Lock()
	defer s.mu.Unlock()
	if err = s.coordinatorAuthority(r.Context(), p.ID); err != nil {
		coordinatorPolicyProblem(w, err)
		return
	}
	actor, _ := authenticatedContextActor(r.Context())
	saved, err := s.readCoordination(p)
	if err != nil {
		orchestratorProblem(w, err)
		return
	}
	key := reviewDigest([]string{actor.ID, req.IdempotencyKey})
	request := reviewDigest([]any{actor.ID, req.TurnID, req.DependsOn})
	for _, intent := range saved.Intents {
		if intent.Key == key {
			if intent.Request != request {
				orchestratorProblem(w, ErrOrchestratorConflict)
				return
			}
			journeyJSON(w, 200, intent.CoordinationIntent)
			return
		}
	}
	turns, err := s.readOrchestrator(p)
	if err != nil {
		orchestratorProblem(w, err)
		return
	}
	turn := coordinationTurn(turns, req.TurnID)
	if turn != nil && turn.Actor.ID != actor.ID {
		coordinatorPolicyProblem(w, errCoordinatorAuthority)
		return
	}
	if turn == nil || turn.Status != "succeeded" || turn.Answer == nil || turn.Answer.Proposal == nil || turn.AppliedRevision != 0 || turn.ApplyIntent != "" || ValidateOrchestratorAnswer(turn.Context, *turn.Answer) != nil {
		orchestratorProblem(w, ErrOrchestratorInvalid)
		return
	}
	snapshot, err := s.orchestratorSnapshotLocked(r.Context(), p)
	if err != nil {
		orchestratorProblem(w, err)
		return
	}
	if saved.Revision != req.ExpectedRevision || snapshot.Digest != req.ExpectedContextDigest || coordinationIntent(&saved, turn.ID) != nil {
		orchestratorProblem(w, ErrOrchestratorConflict)
		return
	}
	deps := map[string]bool{}
	for _, id := range req.DependsOn {
		found := false
		for _, other := range saved.Intents {
			if other.ID == id {
				found = true
			}
		}
		if !found || deps[id] {
			orchestratorProblem(w, ErrOrchestratorInvalid)
			return
		}
		deps[id] = true
	}
	requirements, tasks, all := coordinationImpact(*turn)
	intent := storedCoordinationIntent{CoordinationIntent: CoordinationIntent{ID: newID("intent-"), TurnID: turn.ID, Actor: actor, LuciID: coordinationLuciID(saved.AccountScope, actor.ID), BaseContextDigest: turn.ContextDigest, ProposalDigest: reviewDigest(turn.Answer.Proposal), Target: turn.Answer.Proposal.Target, RequestedChange: turn.Message, ChangedPaths: coordinationPaths(coordinationChanges(turn)), RequirementIDs: requirements, TaskIDs: tasks, AffectsAllTasks: all, DependsOn: append([]string{}, req.DependsOn...), Status: "pending", CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}, Key: key, Request: request}
	saved.Intents = append(saved.Intents, intent)
	coordinationEvent(&saved, intent.CoordinationIntent, "proposed", snapshot.Digest, actor)
	if err = s.saveCoordination(saved); err != nil {
		orchestratorProblem(w, err)
		return
	}
	journeyJSON(w, 201, intent.CoordinationIntent)
}

func (s *Server) resolveCoordinationIntent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ExpectedRevision      int    `json:"expectedRevision"`
		ExpectedContextDigest string `json:"expectedContextDigest"`
		SupersededBy          string `json:"supersededBy"`
		Reason                string `json:"reason"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	if req.ExpectedRevision < 0 || !hexDigest(req.ExpectedContextDigest) || strings.TrimSpace(req.Reason) == "" || len(req.Reason) > 2000 {
		orchestratorProblem(w, ErrOrchestratorInvalid)
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	if err := s.coordinatorAuthority(r.Context(), p.ID); err != nil {
		coordinatorPolicyProblem(w, err)
		return
	}
	release, err := s.AcquireGitSessionAdmission(r.Context(), p.ID)
	if err != nil {
		orchestratorProblem(w, err)
		return
	}
	defer release()
	s.mu.Lock()
	defer s.mu.Unlock()
	if err = s.coordinatorAuthority(r.Context(), p.ID); err != nil {
		coordinatorPolicyProblem(w, err)
		return
	}
	saved, err := s.readCoordination(p)
	if err != nil {
		orchestratorProblem(w, err)
		return
	}
	actor, _ := authenticatedContextActor(r.Context())
	var intent *storedCoordinationIntent
	replacement := false
	var replacementIntent *storedCoordinationIntent
	for i := range saved.Intents {
		if saved.Intents[i].ID == r.PathValue("intentId") {
			intent = &saved.Intents[i]
		}
		if saved.Intents[i].ID == req.SupersededBy && saved.Intents[i].Status != "superseded" {
			replacement = true
			replacementIntent = &saved.Intents[i]
		}
	}
	if intent == nil || intent.Actor.ID != actor.ID {
		coordinatorPolicyProblem(w, errCoordinatorAuthority)
		return
	}
	if replacementIntent != nil {
		if replacementIntent.Target != intent.Target {
			orchestratorProblem(w, ErrOrchestratorConflict)
			return
		}
		blocked := map[string]bool{intent.ID: true}
		for range saved.Intents {
			for _, other := range saved.Intents {
				for _, dep := range other.DependsOn {
					if blocked[dep] {
						blocked[other.ID] = true
					}
				}
			}
		}
		if blocked[replacementIntent.ID] {
			orchestratorProblem(w, ErrOrchestratorConflict)
			return
		}
	}
	if intent.Status == "superseded" && intent.SupersededBy == req.SupersededBy && intent.ResolutionReason == req.Reason {
		journeyJSON(w, 200, intent.CoordinationIntent)
		return
	}
	turns, err := s.readOrchestrator(p)
	if err != nil {
		orchestratorProblem(w, err)
		return
	}
	turn := coordinationTurn(turns, intent.TurnID)
	if turn == nil || turn.ApplyIntent != "" || turn.AppliedRevision > 0 {
		orchestratorProblem(w, ErrOrchestratorConflict)
		return
	}
	snapshot, err := s.orchestratorSnapshotLocked(r.Context(), p)
	if err != nil {
		orchestratorProblem(w, err)
		return
	}
	if intent.Status != "pending" || saved.Revision != req.ExpectedRevision || snapshot.Digest != req.ExpectedContextDigest || !replacement || intent.ID == req.SupersededBy {
		orchestratorProblem(w, ErrOrchestratorConflict)
		return
	}
	intent.Status = "superseded"
	intent.SupersededBy = req.SupersededBy
	intent.ResolutionReason = req.Reason
	value := intent.CoordinationIntent
	coordinationEvent(&saved, value, "superseded", snapshot.Digest, actor)
	if err = s.saveCoordination(saved); err != nil {
		orchestratorProblem(w, err)
		return
	}
	journeyJSON(w, 200, value)
}

// Caller holds the existing branch admission and s.mu. This does not persist a
// replacement conversation/model response or apply canonical context itself.
func (s *Server) prepareCoordinationApplyLocked(ctx context.Context, p project.Project, turn storedOrchestratorTurn, current OrchestratorContext) (OrchestratorAnswer, error) {
	if turn.Answer == nil || turn.Answer.Proposal == nil {
		return OrchestratorAnswer{}, ErrOrchestratorInvalid
	}
	if turn.ApplyIntent != "" {
		return OrchestratorAnswer{}, ErrOrchestratorConflict
	}
	if s.coordinatorAuthorize != nil {
		if err := s.coordinatorAuthority(ctx, p.ID); err != nil {
			return OrchestratorAnswer{}, err
		}
		// A completion write may have succeeded before its coordination write
		// failed. Recover the exact captured origin, never use the legacy bypass.
		if err := s.ensureCoordinationOriginsLocked(p); err != nil {
			return OrchestratorAnswer{}, err
		}
	}
	saved, err := s.readCoordination(p)
	if err != nil {
		return OrchestratorAnswer{}, err
	}
	intent := coordinationIntent(&saved, turn.ID)
	if intent == nil {
		// Even an unregistered proposal cannot bypass a relevant durable opposing
		// intent. No authority is acquired by omitting a client linkage field.
		turns, err := s.readOrchestrator(p)
		if err != nil {
			return OrchestratorAnswer{}, err
		}
		for _, other := range saved.Intents {
			if other.Status == "pending" && other.Target == turn.Answer.Proposal.Target {
				peer := coordinationTurn(turns, other.TurnID)
				if peer == nil || len(coordinationOverlaps(coordinationChanges(&turn), coordinationChanges(peer))) > 0 {
					return OrchestratorAnswer{}, ErrOrchestratorConflict
				}
			}
		}
		if turn.ContextDigest != current.Digest {
			return OrchestratorAnswer{}, ErrOrchestratorConflict
		}
		return *cloneOrchestratorAnswer(turn.Answer), nil
	}
	if err = s.coordinatorAuthority(ctx, p.ID); err != nil {
		return OrchestratorAnswer{}, err
	}
	if intent.Status != "pending" || intent.ProposalDigest != reviewDigest(turn.Answer.Proposal) || intent.BaseContextDigest != turn.ContextDigest {
		return OrchestratorAnswer{}, ErrOrchestratorConflict
	}
	turns, err := s.readOrchestrator(p)
	if err != nil {
		return OrchestratorAnswer{}, err
	}
	for _, conflict := range coordinationConflicts(saved, turns, current) {
		if conflict.IntentID == intent.ID || conflict.OtherIntentID == intent.ID {
			return OrchestratorAnswer{}, ErrOrchestratorConflict
		}
	}
	answer, conflicts := coordinationRebase(turn, current)
	if len(conflicts) > 0 || ValidateOrchestratorAnswer(current, answer) != nil {
		return OrchestratorAnswer{}, ErrOrchestratorConflict
	}
	// Freeze reconciliation provenance before the canonical engine writes its
	// own apply intent. Reopening does not assume this candidate was applied.
	candidate := reviewDigest(answer.Proposal)
	if intent.ApplyBaseContextDigest != current.Digest || intent.CandidateProposalDigest != candidate {
		intent.ApplyBaseContextDigest = current.Digest
		intent.CandidateProposalDigest = candidate
		saved.Revision++
		if err = s.saveCoordination(saved); err != nil {
			return OrchestratorAnswer{}, err
		}
	}
	return answer, nil
}

func (s *Server) finishCoordinationApplyLocked(ctx context.Context, p project.Project, turn storedOrchestratorTurn) error {
	saved, err := s.readCoordination(p)
	if err != nil {
		return err
	}
	intent := coordinationIntent(&saved, turn.ID)
	if intent == nil {
		return nil
	}
	if err = s.coordinatorAuthority(ctx, p.ID); err != nil {
		return err
	}
	if intent.Status == "applied" {
		if intent.AppliedRevision == turn.AppliedRevision {
			return nil
		}
		return ErrOrchestratorConflict
	}
	if intent.Status != "pending" || turn.AppliedRevision < 1 || intent.ProposalDigest != reviewDigest(turn.Answer.Proposal) {
		return ErrOrchestratorConflict
	}
	// This is a confirmed exact canonical target from the existing durable apply
	// receipt. Do not manufacture acceptance from a proposal or current row alone.
	if !hexDigest(turn.ApplyTargetDigest) || turn.ApplyTargetRevision != turn.AppliedRevision || !hexDigest(intent.ApplyBaseContextDigest) || !hexDigest(intent.CandidateProposalDigest) {
		return ErrOrchestratorInvalid
	}
	intent.Status = "applied"
	intent.AppliedRevision = turn.AppliedRevision
	intent.AppliedTargetDigest = turn.ApplyTargetDigest
	// The existing apply receipt proves the target, not every other document
	// at that instant. A delayed finalize must not label today's whole context
	// as yesterday's acceptance. Consumers fetch and admit current context.
	value := intent.CoordinationIntent
	actor, _ := authenticatedContextActor(ctx)
	coordinationEvent(&saved, value, "applied", intent.ApplyBaseContextDigest, actor)
	return s.saveCoordination(saved)
}

func (s *Server) validateCoordinationRecoveryLocked(ctx context.Context, p project.Project, turn storedOrchestratorTurn) error {
	if s.coordinatorAuthorize != nil {
		if err := s.coordinatorAuthority(ctx, p.ID); err != nil {
			return err
		}
	}
	saved, err := s.readCoordination(p)
	if err != nil {
		return err
	}
	if intent := coordinationIntent(&saved, turn.ID); intent != nil {
		if err = s.coordinatorAuthority(ctx, p.ID); err != nil {
			return err
		}
		if intent.Status == "superseded" || turn.Answer == nil || intent.ProposalDigest != reviewDigest(turn.Answer.Proposal) {
			return ErrOrchestratorConflict
		}
	} else if s.coordinatorAuthorize != nil && turn.ApplyIntent != "" {
		return ErrOrchestratorInvalid
	}
	return nil
}

// Retain all bounded origin receipts while their causal coordination records
// remain authoritative, including applied receipts used by dependencies.
func (s *Server) retainedCoordinationTurnsLocked(p project.Project) (map[string]bool, error) {
	saved, err := s.readCoordination(p)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, intent := range saved.Intents {
		out[intent.TurnID] = true
	}
	return out, nil
}
