package product

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

const portfolioFocusOwnerLimit = 128
const portfolioFocusObservationLimit = 16

// Focus changes admission preferences, never execution consent or host limits.
// Each upstream reference means "wait until this exact run is accepted".
type PortfolioFocusRunSelection struct {
	ProjectID string `json:"projectId"`
	RunID     string `json:"runId"`
}

type PortfolioFocusDependency struct {
	ProjectID   string `json:"projectId"`
	RunID       string `json:"runId"`
	InputDigest string `json:"inputDigest"`
}

type PortfolioFocusAllocation struct {
	ProjectID         string                     `json:"projectId"`
	MaxActive         int                        `json:"maxActive"` // zero is unspecified, not a reservation
	WaitUntilAccepted []PortfolioFocusDependency `json:"waitUntilAccepted"`
}

type PortfolioFocusProfile struct {
	GoalID       string                     `json:"goalId"`
	GoalRevision int                        `json:"goalRevision"`
	GoalDigest   string                     `json:"goalDigest"`
	Allocations  []PortfolioFocusAllocation `json:"allocations"`
	Digest       string                     `json:"digest"`
}

type PortfolioFocusView struct {
	Revision int                    `json:"revision"`
	Profile  *PortfolioFocusProfile `json:"profile,omitempty"`
}

type PortfolioFocusPlan struct {
	Profile               PortfolioFocusProfile `json:"profile"`
	ExpectedFocusRevision int                   `json:"expectedFocusRevision"`
	Digest                string                `json:"digest"`
	ExecutionStarted      bool                  `json:"executionStarted"`
}

type PortfolioFocusRequest struct {
	ExpectedRevision      int                          `json:"expectedRevision"`
	ExpectedProfileDigest string                       `json:"expectedProfileDigest,omitempty"`
	UpstreamRuns          []PortfolioFocusRunSelection `json:"upstreamRuns,omitempty"`
}

// Actor/team identity is private scheduler metadata, not supplied by clients.
// Tombstones retain revision/key CAS after removal; old requests cannot restore it.
type swarmFocusRecord struct {
	ActorID       string                 `json:"actorId"`
	TeamID        string                 `json:"teamId,omitempty"`
	Revision      int                    `json:"revision"`
	Profile       *PortfolioFocusProfile `json:"profile,omitempty"`
	KeyDigest     string                 `json:"keyDigest"`
	RequestDigest string                 `json:"requestDigest"`
}

func focusProfileDigest(profile PortfolioFocusProfile) string {
	profile.Digest = ""
	return reviewDigest(profile)
}

func validFocusProfile(profile PortfolioFocusProfile) bool {
	if !prototypeReleaseID.MatchString(profile.GoalID) || profile.GoalRevision < 1 || !hexDigest(profile.GoalDigest) ||
		len(profile.Allocations) < 1 || len(profile.Allocations) > portfolioGoalProjectLimit || profile.Digest != focusProfileDigest(profile) {
		return false
	}
	projects := map[string]bool{}
	for _, allocation := range profile.Allocations {
		if !prototypeReleaseID.MatchString(allocation.ProjectID) || projects[allocation.ProjectID] || allocation.MaxActive < 0 || allocation.MaxActive > MaxSwarmParallelTasks || allocation.WaitUntilAccepted == nil || len(allocation.WaitUntilAccepted) > 15 {
			return false
		}
		projects[allocation.ProjectID] = true
	}
	graph := map[string][]string{}
	for _, allocation := range profile.Allocations {
		seen := map[string]bool{}
		for _, dependency := range allocation.WaitUntilAccepted {
			if !projects[dependency.ProjectID] || dependency.ProjectID == allocation.ProjectID || seen[dependency.ProjectID] || !validRunID(dependency.RunID) || !hexDigest(dependency.InputDigest) {
				return false
			}
			seen[dependency.ProjectID] = true
			graph[allocation.ProjectID] = append(graph[allocation.ProjectID], dependency.ProjectID)
		}
	}
	visiting, visited := map[string]bool{}, map[string]bool{}
	var visit func(string) bool
	visit = func(id string) bool {
		if visiting[id] {
			return false
		}
		if visited[id] {
			return true
		}
		visiting[id] = true
		for _, dependency := range graph[id] {
			if !visit(dependency) {
				return false
			}
		}
		visiting[id], visited[id] = false, true
		return true
	}
	for id := range projects {
		if !visit(id) {
			return false
		}
	}
	return true
}

func validSwarmFocusRecords(records []swarmFocusRecord) bool {
	if len(records) > portfolioFocusOwnerLimit {
		return false
	}
	seen := map[string]bool{}
	for _, record := range records {
		key := reviewDigest([]string{record.ActorID, record.TeamID})
		if !validConnectionToken(record.ActorID, 128, false) || !validConnectionToken(record.TeamID, 128, true) || seen[key] || record.Revision < 1 || !hexDigest(record.KeyDigest) || !hexDigest(record.RequestDigest) || record.Profile != nil && !validFocusProfile(*record.Profile) {
			return false
		}
		seen[key] = true
	}
	return true
}

func focusRecord(records []swarmFocusRecord, actor, team string) (swarmFocusRecord, int) {
	for i, record := range records {
		if record.ActorID == actor && record.TeamID == team {
			return record, i
		}
	}
	return swarmFocusRecord{ActorID: actor, TeamID: team}, -1
}

func focusProjects(profile *PortfolioFocusProfile) []string {
	ids := []string{}
	if profile != nil {
		for _, allocation := range profile.Allocations {
			ids = append(ids, allocation.ProjectID)
		}
	}
	return ids
}

func authorizeFocus(ctx context.Context, profile *PortfolioFocusProfile, access PortfolioGoalAccess, control bool) error {
	if access == nil {
		return management.ErrExecutionAuthority
	}
	for _, id := range focusProjects(profile) {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := access(ctx, id, control); err != nil {
			return err
		}
	}
	return ctx.Err()
}

func portfolioFocusTeam(ctx context.Context) (string, error) {
	team := management.TeamRuntimeIDFromContext(ctx)
	if access, ok := management.TeamAccessFromContext(ctx); ok {
		if access.Team.ID == "" || team != "" && access.Team.ID != team {
			return "", management.ErrExecutionAuthority
		}
		team = access.Team.ID
	}
	return team, nil
}

func (s *Server) SetPortfolioFocusReadAuthorizer(authorize management.ProjectReadAuthorizer) {
	s.swarmScheduler.mu.Lock()
	s.swarmScheduler.focusRead = authorize
	s.swarmScheduler.mu.Unlock()
}

func (s *Server) ReadPortfolioFocus(ctx context.Context, access PortfolioGoalAccess) (PortfolioFocusView, error) {
	actor, err := portfolioGoalActor(ctx)
	if err != nil {
		return PortfolioFocusView{}, err
	}
	team, err := portfolioFocusTeam(ctx)
	if err != nil {
		return PortfolioFocusView{}, err
	}
	s.swarmScheduler.mu.Lock()
	row, err := s.readSwarmScheduler()
	s.swarmScheduler.mu.Unlock()
	if err != nil {
		return PortfolioFocusView{}, err
	}
	record, _ := focusRecord(row.Focus, actor, team)
	if err := authorizeFocus(ctx, record.Profile, access, false); err != nil {
		return PortfolioFocusView{}, err
	}
	return PortfolioFocusView{Revision: record.Revision, Profile: record.Profile}, ctx.Err()
}

func (s *Server) ReviewPortfolioFocus(ctx context.Context, goalID string, revision int, request PortfolioFocusRequest, access PortfolioGoalAccess) (PortfolioFocusPlan, error) {
	actor, err := portfolioGoalActor(ctx)
	if err != nil {
		return PortfolioFocusPlan{}, err
	}
	// Copy the canonical goal before project/run reads; no scheduler lock spans them.
	s.portfolioGoalsMu.Lock()
	saved, err := s.readPortfolioGoals(actor)
	var goal PortfolioGoal
	if err == nil {
		selected := portfolioGoalByID(&saved, goalID)
		if selected == nil || selected.Revision != revision {
			err = ErrPortfolioGoalConflict
		} else {
			goal = *selected
		}
	}
	s.portfolioGoalsMu.Unlock()
	if err != nil {
		return PortfolioFocusPlan{}, err
	}
	if err = authorizePortfolioGoal(ctx, goal, access, false); err != nil {
		return PortfolioFocusPlan{}, err
	}
	if len(request.UpstreamRuns) > portfolioGoalProjectLimit {
		return PortfolioFocusPlan{}, ErrPortfolioGoalInvalid
	}
	selections := map[string]string{}
	for _, selected := range request.UpstreamRuns {
		if !prototypeReleaseID.MatchString(selected.ProjectID) || !validRunID(selected.RunID) || selections[selected.ProjectID] != "" {
			return PortfolioFocusPlan{}, ErrPortfolioGoalInvalid
		}
		selections[selected.ProjectID] = selected.RunID
	}
	needed := map[string]bool{}
	for _, target := range goal.Projects {
		for _, id := range target.DependsOn {
			needed[id] = true
		}
	}
	if len(needed) != len(selections) {
		return PortfolioFocusPlan{}, errors.New("Choose one exact upstream run for every wait-until-accepted dependency.")
	}
	dependencies := map[string]PortfolioFocusDependency{}
	for id := range needed {
		if selections[id] == "" {
			return PortfolioFocusPlan{}, ErrPortfolioGoalInvalid
		}
		if err := access(ctx, id, false); err != nil {
			return PortfolioFocusPlan{}, err
		}
		run, err := s.GetProjectSwarmRun(id, selections[id])
		if err != nil {
			return PortfolioFocusPlan{}, err
		}
		if err := access(ctx, id, false); err != nil {
			return PortfolioFocusPlan{}, err
		}
		dependencies[id] = PortfolioFocusDependency{ProjectID: id, RunID: run.ID, InputDigest: reviewDigest(run.Input)}
	}
	profile := PortfolioFocusProfile{GoalID: goal.ID, GoalRevision: goal.Revision, GoalDigest: reviewDigest(goal), Allocations: []PortfolioFocusAllocation{}}
	for _, target := range goal.Projects {
		allocation := PortfolioFocusAllocation{ProjectID: target.ProjectID, MaxActive: target.RequestedSlots, WaitUntilAccepted: []PortfolioFocusDependency{}}
		for _, id := range target.DependsOn {
			allocation.WaitUntilAccepted = append(allocation.WaitUntilAccepted, dependencies[id])
		}
		sort.Slice(allocation.WaitUntilAccepted, func(i, j int) bool {
			return allocation.WaitUntilAccepted[i].ProjectID < allocation.WaitUntilAccepted[j].ProjectID
		})
		profile.Allocations = append(profile.Allocations, allocation)
	}
	sort.Slice(profile.Allocations, func(i, j int) bool { return profile.Allocations[i].ProjectID < profile.Allocations[j].ProjectID })
	profile.Digest = focusProfileDigest(profile)
	if !validFocusProfile(profile) {
		return PortfolioFocusPlan{}, ErrPortfolioGoalInvalid
	}
	current, err := s.ReadPortfolioFocus(ctx, access)
	if err != nil {
		return PortfolioFocusPlan{}, err
	}
	plan := PortfolioFocusPlan{Profile: profile, ExpectedFocusRevision: current.Revision}
	plan.Digest = reviewDigest(plan)
	return plan, ctx.Err()
}

// One atomic ledger save installs/removes preferences. It starts nothing and
// never alters current children. Same-key readback never repeats the write.
func (s *Server) ApplyPortfolioFocus(ctx context.Context, request PortfolioGoalApply, access PortfolioGoalAccess) (PortfolioFocusView, error) {
	actor, err := portfolioGoalActor(ctx)
	if err != nil {
		return PortfolioFocusView{}, err
	}
	if request.Focus == nil || !validRunKey(request.IdempotencyKey) || request.Focus.ExpectedRevision < 0 || (request.Mode != "focus-install" && request.Mode != "focus-remove") {
		return PortfolioFocusView{}, ErrPortfolioGoalInvalid
	}
	team, err := portfolioFocusTeam(ctx)
	if err != nil {
		return PortfolioFocusView{}, err
	}
	key, digest := reviewDigest(request.IdempotencyKey), reviewDigest(request)
	s.swarmScheduler.mu.Lock()
	ledger, err := s.readSwarmScheduler()
	s.swarmScheduler.mu.Unlock()
	if err != nil {
		return PortfolioFocusView{}, err
	}
	previous, _ := focusRecord(ledger.Focus, actor, team)
	if previous.KeyDigest == key {
		if previous.RequestDigest != digest {
			return PortfolioFocusView{}, ErrPortfolioGoalConflict
		}
		if err := authorizeFocus(ctx, previous.Profile, access, false); err != nil {
			return PortfolioFocusView{}, err
		}
		return PortfolioFocusView{Revision: previous.Revision, Profile: previous.Profile}, nil
	}
	if previous.Revision != request.Focus.ExpectedRevision {
		return PortfolioFocusView{}, ErrPortfolioGoalConflict
	}
	var profile *PortfolioFocusProfile
	if request.Mode == "focus-install" {
		if request.Focus.ExpectedProfileDigest != "" {
			return PortfolioFocusView{}, ErrPortfolioGoalInvalid
		}
		plan, err := s.ReviewPortfolioFocus(ctx, request.GoalID, request.ExpectedGoalRevision, *request.Focus, access)
		if err != nil {
			return PortfolioFocusView{}, err
		}
		if plan.Digest != request.ExpectedPlanDigest || plan.ExpectedFocusRevision != previous.Revision {
			return PortfolioFocusView{}, ErrPortfolioGoalConflict
		}
		profile = &plan.Profile
	} else if previous.Profile == nil || previous.Profile.GoalID != request.GoalID || previous.Profile.GoalRevision != request.ExpectedGoalRevision || request.Focus.ExpectedProfileDigest != previous.Profile.Digest || len(request.Focus.UpstreamRuns) != 0 {
		return PortfolioFocusView{}, ErrPortfolioGoalConflict
	}
	// Replacing a profile also needs current control of the old scope.
	if err := authorizeFocus(ctx, previous.Profile, access, true); err != nil {
		return PortfolioFocusView{}, err
	}
	if err := authorizeFocus(ctx, profile, access, true); err != nil {
		return PortfolioFocusView{}, err
	}
	// Serialize against goal writes, then the short scheduler commit. No external
	// authorizer, run read or provider operation occurs under the scheduler mutex.
	s.portfolioGoalsMu.Lock()
	defer s.portfolioGoalsMu.Unlock()
	if profile != nil {
		saved, err := s.readPortfolioGoals(actor)
		if err != nil {
			return PortfolioFocusView{}, err
		}
		goal := portfolioGoalByID(&saved, profile.GoalID)
		if goal == nil || goal.Revision != profile.GoalRevision || reviewDigest(*goal) != profile.GoalDigest {
			return PortfolioFocusView{}, ErrPortfolioGoalConflict
		}
	}
	// The goal mutex may have been contended since the first authority pass.
	// Re-resolve both scopes after that wait, immediately before the short CAS.
	if err := authorizeFocus(ctx, previous.Profile, access, true); err != nil {
		return PortfolioFocusView{}, err
	}
	if err := authorizeFocus(ctx, profile, access, true); err != nil {
		return PortfolioFocusView{}, err
	}
	if err := ctx.Err(); err != nil {
		return PortfolioFocusView{}, err
	}
	s.swarmScheduler.mu.Lock()
	defer s.swarmScheduler.mu.Unlock()
	ledger, err = s.readSwarmScheduler()
	if err != nil {
		return PortfolioFocusView{}, err
	}
	current, index := focusRecord(ledger.Focus, actor, team)
	if current.Revision != previous.Revision {
		return PortfolioFocusView{}, ErrPortfolioGoalConflict
	}
	if index < 0 && len(ledger.Focus) >= portfolioFocusOwnerLimit {
		return PortfolioFocusView{}, ErrProjectRunLimit
	}
	record := swarmFocusRecord{ActorID: actor, TeamID: team, Revision: current.Revision + 1, Profile: profile, KeyDigest: key, RequestDigest: digest}
	if index < 0 {
		ledger.Focus = append(ledger.Focus, record)
	} else {
		ledger.Focus[index] = record
	}
	if err := s.saveSwarmScheduler(&ledger); err != nil {
		return PortfolioFocusView{}, err
	}
	return PortfolioFocusView{Revision: record.Revision, Profile: record.Profile}, nil
}

func acceptedFocusRun(run ProjectRun, dependency PortfolioFocusDependency) bool {
	if run.ProjectID != dependency.ProjectID || run.ID != dependency.RunID || reviewDigest(run.Input) != dependency.InputDigest || run.State != "complete" || run.Acceptance == nil || run.Observation == nil || !run.Observation.Valid {
		return false
	}
	if _, err := time.Parse(time.RFC3339Nano, run.Acceptance.AcceptedAt); err != nil {
		return false
	}
	for _, check := range run.Checks {
		if check.CheckID != run.Acceptance.CheckID || validProjectRunCheck(run.Input, check) != nil {
			continue
		}
		for _, release := range run.Releases {
			if release.ReleaseID == run.Acceptance.ReleaseID && release.CheckID == check.CheckID && validProjectRunRelease(run.Input, check, release) == nil {
				return true
			}
		}
	}
	return false
}

func focusAllocation(profile *PortfolioFocusProfile, projectID string) *PortfolioFocusAllocation {
	if profile != nil {
		for i := range profile.Allocations {
			if profile.Allocations[i].ProjectID == projectID {
				return &profile.Allocations[i]
			}
		}
	}
	return nil
}

func focusObservationKey(item swarmQueueRecord, profile *PortfolioFocusProfile) string {
	return reviewDigest([]string{item.Actor.UserID, item.Actor.TeamID, item.ProjectID, item.ID, profile.Digest})
}

// Rotate bounded observations independently of LastServed. Blocked upstream
// evidence must not permanently consume the first sixteen slots every turn.
func focusObservationWindow(row swarmSchedulerLedger, cursor int) (map[string]int, int) {
	set := map[string]bool{}
	for _, item := range row.Items {
		if item.Actor.Local || item.Actor.UserID == "" || item.State != "queued" && item.State != "claiming" && item.State != "reserved" {
			continue
		}
		record, _ := focusRecord(row.Focus, item.Actor.UserID, item.Actor.TeamID)
		if focusAllocation(record.Profile, item.ProjectID) != nil {
			set[focusObservationKey(item, record.Profile)] = true
		}
	}
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	window := map[string]int{}
	for i := 0; i < len(keys) && i < portfolioFocusObservationLimit; i++ {
		index := (cursor + i) % len(keys)
		window[keys[index]] = index
	}
	return window, len(keys)
}

func focusCapacityAllows(row swarmSchedulerLedger, item swarmQueueRecord, allocation PortfolioFocusAllocation, notStarted ...map[string]bool) bool {
	if allocation.MaxActive == 0 {
		return true
	}
	unclaimed := map[string]bool{}
	if len(notStarted) == 1 {
		unclaimed = notStarted[0]
	}
	active := map[string]bool{}
	pending := map[string]swarmQueueRecord{}
	identity := func(row swarmQueueRecord) string {
		if row.OperationID == "" {
			return "queue/" + row.ID
		}
		return row.RunID + "/" + row.OperationID
	}
	before := func(a, b swarmQueueRecord) bool {
		at, aErr := time.Parse(time.RFC3339Nano, a.QueuedAt)
		bt, bErr := time.Parse(time.RFC3339Nano, b.QueuedAt)
		if aErr != nil || bErr != nil {
			return false
		}
		if !at.Equal(bt) {
			return at.Before(bt)
		}
		return a.ID < b.ID
	}
	if unclaimed[item.ID] {
		if _, err := time.Parse(time.RFC3339Nano, item.QueuedAt); err != nil {
			return false
		}
	}
	for _, other := range row.Items {
		if other.ID == item.ID || other.ProjectID != item.ProjectID || other.Actor.UserID != item.Actor.UserID || other.Actor.TeamID != item.Actor.TeamID || !swarmQueueActive(other.State) {
			continue
		}
		if item.OperationID != "" && other.RunID == item.RunID && other.OperationID == item.OperationID {
			continue
		}
		key := identity(other)
		_, timeErr := time.Parse(time.RFC3339Nano, other.QueuedAt)
		if !unclaimed[other.ID] || timeErr != nil {
			active[key] = true
			delete(pending, key)
			continue
		}
		if !active[key] {
			if previous, exists := pending[key]; !exists || before(other, previous) {
				pending[key] = other
			}
		}
	}
	if !unclaimed[item.ID] {
		return len(active) < allocation.MaxActive
	}
	// Multiple old reservations can exceed a newly lowered ceiling. They keep
	// their identities and resource leases, but only the oldest unclaimed ones
	// receive remaining start entitlement. Once claimed, each becomes hard
	// active/uncertain and blocks later starts until ordinary settlement.
	ahead := 0
	for _, other := range pending {
		if before(other, item) {
			ahead++
		}
	}
	return len(active)+ahead < allocation.MaxActive
}

func focusUnclaimedReservation(item swarmQueueRecord, run ProjectRun) bool {
	if run.ProjectID != item.ProjectID || run.ID != item.RunID || queueInputDigest(run) != item.InputDigest || swarmTask(&run, item.TaskID) == nil {
		return false
	}
	if item.OperationID == "" {
		return item.ChildID == "" && (item.State == "queued" || item.State == "claiming")
	}
	op := swarmOperation(&run, item.OperationID)
	return op != nil && op.Kind == "session" && op.TaskID == item.TaskID && op.ChildID == item.ChildID && op.Allocation == "ready" && op.Status == "reserved"
}

// Both scheduler selection and the API's final allocation callback use this
// bounded read. Reads and authorizers run outside the scheduler lock; a final
// revision/capacity recheck rejects a concurrently replaced profile.
func (s *Server) CheckPortfolioFocusAdmission(ctx context.Context, queueID string) error {
	s.swarmScheduler.mu.Lock()
	row, err := s.readSwarmScheduler()
	authorize := s.swarmScheduler.focusRead
	s.swarmScheduler.mu.Unlock()
	if err != nil {
		return err
	}
	var item *swarmQueueRecord
	for i := range row.Items {
		if row.Items[i].ID == queueID {
			item = &row.Items[i]
			break
		}
	}
	if item == nil {
		return ErrProjectRunNotFound
	}
	if item.Actor.Local || item.Actor.UserID == "" {
		return ctx.Err()
	}
	record, _ := focusRecord(row.Focus, item.Actor.UserID, item.Actor.TeamID)
	allocation := focusAllocation(record.Profile, item.ProjectID)
	if allocation == nil {
		return ctx.Err()
	}
	deadline, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	// Positively classify retained reservations using canonical exact operations.
	// A queue's dispatching intent alone does not mean ClaimOperation occurred.
	// Any uncertain/claimed original remains hard-held, never assumed absent.
	unclaimed := map[string]bool{}
	runs := map[string]ProjectRun{}
	readRun := func(projectID, runID string) (ProjectRun, error) {
		actor := item.Actor
		actor.ProjectID = projectID
		if authorize == nil {
			return ProjectRun{}, management.ErrExecutionAuthority
		}
		if _, err := authorize(deadline, actor); err != nil {
			return ProjectRun{}, err
		}
		key := projectID + "/" + runID
		run, cached := runs[key]
		if !cached {
			var err error
			run, err = s.GetProjectSwarmRun(projectID, runID)
			if err != nil {
				return ProjectRun{}, err
			}
			runs[key] = run
		}
		if _, err := authorize(deadline, actor); err != nil {
			return ProjectRun{}, err
		}
		return run, deadline.Err()
	}
	if allocation.MaxActive > 0 {
		for _, candidate := range row.Items {
			if candidate.Actor != item.Actor || candidate.ProjectID != item.ProjectID || candidate.ID != item.ID && !swarmQueueActive(candidate.State) {
				continue
			}
			if candidate.State == "running" || candidate.State == "unknown" {
				continue
			}
			run, err := readRun(candidate.ProjectID, candidate.RunID)
			if err != nil {
				if candidate.ID == item.ID {
					return err
				}
				continue // retains hard-held capacity
			}
			unclaimed[candidate.ID] = focusUnclaimedReservation(candidate, run)
		}
		if !focusCapacityAllows(row, *item, *allocation, unclaimed) {
			return errors.New("Waiting for the applied focus ceiling or an earlier exact retained reservation; uncertain originals keep capacity.")
		}
	}
	for _, dependency := range allocation.WaitUntilAccepted {
		run, err := readRun(dependency.ProjectID, dependency.RunID)
		if err != nil {
			return err
		}
		if !acceptedFocusRun(run, dependency) {
			return errors.New("Waiting until the exact upstream run is accepted with matching check and release evidence.")
		}
	}
	if err := deadline.Err(); err != nil {
		return err
	}
	for _, original := range runs {
		actor := item.Actor
		actor.ProjectID = original.ProjectID
		if _, err := authorize(deadline, actor); err != nil {
			return err
		}
		current, err := s.GetProjectSwarmRun(original.ProjectID, original.ID)
		if err != nil {
			return err
		}
		if current.Revision != original.Revision || reviewDigest(current.Input) != reviewDigest(original.Input) {
			return ErrPortfolioGoalConflict
		}
		if _, err := authorize(deadline, actor); err != nil {
			return err
		}
	}
	if err := deadline.Err(); err != nil {
		return err
	}
	s.swarmScheduler.mu.Lock()
	latest, err := s.readSwarmScheduler()
	s.swarmScheduler.mu.Unlock()
	if err != nil {
		return err
	}
	current, _ := focusRecord(latest.Focus, item.Actor.UserID, item.Actor.TeamID)
	if current.Revision != record.Revision || current.Profile == nil || record.Profile == nil || current.Profile.Digest != record.Profile.Digest {
		return ErrPortfolioGoalConflict
	}
	unchanged := false
	for _, candidate := range latest.Items {
		if candidate.ID == item.ID {
			unchanged = candidate.Revision == item.Revision && candidate.Actor == item.Actor && candidate.ProjectID == item.ProjectID && candidate.RunID == item.RunID && candidate.OperationID == item.OperationID && candidate.ChildID == item.ChildID
		}
	}
	if !unchanged {
		return ErrPortfolioGoalConflict
	}
	// Every positively unclaimed peer must still have the same original row.
	// A concurrent claim invalidates its entitlement snapshot, not its ownership.
	for _, original := range row.Items {
		if !unclaimed[original.ID] {
			continue
		}
		matched := false
		for _, candidate := range latest.Items {
			if candidate.ID == original.ID && candidate == original {
				matched = true
				break
			}
		}
		if !matched {
			return ErrPortfolioGoalConflict
		}
	}
	if !focusCapacityAllows(latest, *item, *allocation, unclaimed) {
		return errors.New("Your applied focus ceiling changed before admission.")
	}
	return ctx.Err()
}
