package registry

import (
	"errors"
	"reflect"
	"sort"
	"time"
)

const (
	maxTaskObservationScan   = 128
	maxTaskObservationResult = 8
)

// TaskObservationCandidate is a bounded identity snapshot, not a session or a
// claim that task output is ready. The observer must Get the current session
// and compare this generation before collecting output, then compare it again
// in the watermark Update. No context, prompt, token, or transcript is copied.
type TaskObservationCandidate struct {
	ID          string
	ProjectID   string
	RunID       string
	OperationID string
	SandboxID   string
	SandboxRef  string
	Status      Status
	UpdatedAt   time.Time
}

// TaskObservationReader is an optional capability. Store remains unchanged so
// callers cannot accidentally turn a bounded observation into a full List.
type TaskObservationReader interface {
	TaskObservationPage(cursor int, scanLimit, resultLimit int) ([]TaskObservationCandidate, int, error)
}

// ProjectRunSessionReader performs the exact candidate lookup used to detect
// duplicate/foreign operation bindings, including legacy unbound run rows.
type ProjectRunSessionReader interface {
	ProjectRunSessions(runID, operationID string) ([]Session, error)
}

var (
	_ TaskObservationReader   = (*FileStore)(nil)
	_ ProjectRunSessionReader = (*FileStore)(nil)
)

// The watermark describes one exact task generation. Keep this request-field
// comparison aligned with the reviewed starting request, including the private
// connection generation; accounting and display updates do not requeue output.
// Comparing the full implementation value also catches changes to its slices.
func sameTaskObservationGeneration(before, after Session) bool {
	return before.ProjectID == after.ProjectID && before.WorktreeID == after.WorktreeID &&
		before.ProjectRunID == after.ProjectRunID && before.ProjectRunOperationID == after.ProjectRunOperationID &&
		before.SandboxID == after.SandboxID && before.SandboxRef == after.SandboxRef &&
		before.Status == after.Status && before.ExitReason == after.ExitReason && before.StartFailure == after.StartFailure &&
		before.Harness == after.Harness && before.Model == after.Model && before.Effort == after.Effort &&
		before.PermissionMode == after.PermissionMode && before.AuthMode == after.AuthMode &&
		before.AgentConnectionID == after.AgentConnectionID && before.AgentConnectionKind == after.AgentConnectionKind &&
		before.AgentConnectionProtocol == after.AgentConnectionProtocol && before.AgentConnectionRevision == after.AgentConnectionRevision &&
		before.CodexConnectionID == after.CodexConnectionID && reflect.DeepEqual(before.ImplementationContext, after.ImplementationContext)
}

// TaskObservationPage visits at most 128 stored positions and returns at most
// eight ended task identities. Limits must be positive; larger limits clamp.
// The cursor is a best-effort offset into insertion order, not a durable page
// token. Concurrent deletion may shift an entry behind the cursor; the next
// wrap revisits it. Each call visits any position at most once, and mutations
// cannot change the order during a call. No record contexts are cloned or
// history sorted, even when there are no eligible rows.
func (s *FileStore) TaskObservationPage(cursor int, scanLimit, resultLimit int) ([]TaskObservationCandidate, int, error) {
	if cursor < 0 || scanLimit <= 0 || resultLimit <= 0 {
		return nil, cursor, errors.New("registry: observation cursor and limits are invalid")
	}
	scanLimit = min(scanLimit, maxTaskObservationScan)
	resultLimit = min(resultLimit, maxTaskObservationResult)
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.order) == 0 {
		return nil, 0, nil
	}
	if cursor >= len(s.order) {
		cursor = 0
	}
	out := make([]TaskObservationCandidate, 0, resultLimit)
	for scanned := 0; scanned < min(scanLimit, len(s.order)); scanned++ {
		row := s.sessions[s.order[cursor]]
		cursor = (cursor + 1) % len(s.order)
		if row.Status != StatusExited || row.ProjectRunID == "" || row.ProjectRunOperationID == "" || row.TaskObservationComplete {
			continue
		}
		out = append(out, TaskObservationCandidate{
			ID: row.ID, ProjectID: row.ProjectID, RunID: row.ProjectRunID,
			OperationID: row.ProjectRunOperationID, SandboxID: row.SandboxID,
			SandboxRef: row.SandboxRef, Status: row.Status, UpdatedAt: row.UpdatedAt,
		})
		if len(out) == resultLimit {
			break
		}
	}
	return out, cursor, nil
}

// ProjectRunSessions preserves the existing observer's candidate union:
// matching operation ID (even in a foreign run/project), plus matching run ID
// with no operation ID. Foreign rows must reach the caller's binding checks.
// The two index sets are disjoint because operationID must be nonempty.
func (s *FileStore) ProjectRunSessions(runID, operationID string) ([]Session, error) {
	if runID == "" || operationID == "" {
		return nil, errors.New("registry: project run and operation identities are required")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	operationRows, legacyRows := s.operationSessions[operationID], s.legacyRunSessions[runID]
	if len(operationRows)+len(legacyRows) > 2 {
		return nil, errors.New("multiple coding sessions claim this run operation")
	}
	out := make([]Session, 0, len(operationRows)+len(legacyRows))
	for id := range operationRows {
		out = append(out, s.sessions[id].Clone())
	}
	for id := range legacyRows {
		out = append(out, s.sessions[id].Clone())
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.Before(out[j].CreatedAt)
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// Callers hold mu (except the constructor's private load). Index maintenance
// occurs only after successful persistence, so failed writes need no rollback.
func (s *FileStore) indexTaskSessionLocked(row Session) {
	if row.ProjectRunOperationID != "" {
		addTaskSessionIndex(s.operationSessions, row.ProjectRunOperationID, row.ID)
	} else if row.ProjectRunID != "" {
		addTaskSessionIndex(s.legacyRunSessions, row.ProjectRunID, row.ID)
	}
}

func (s *FileStore) unindexTaskSessionLocked(row Session) {
	if row.ProjectRunOperationID != "" {
		removeTaskSessionIndex(s.operationSessions, row.ProjectRunOperationID, row.ID)
	} else if row.ProjectRunID != "" {
		removeTaskSessionIndex(s.legacyRunSessions, row.ProjectRunID, row.ID)
	}
}

func addTaskSessionIndex(index map[string]map[string]struct{}, key, id string) {
	if index[key] == nil {
		index[key] = make(map[string]struct{})
	}
	index[key][id] = struct{}{}
}

func removeTaskSessionIndex(index map[string]map[string]struct{}, key, id string) {
	delete(index[key], id)
	if len(index[key]) == 0 {
		delete(index, key)
	}
}
