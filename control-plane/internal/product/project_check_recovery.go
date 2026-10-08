package product

import (
	"context"
	"errors"
	"net/http"
	"time"
)

type ProjectCheckRecoveryView struct {
	ProjectID      string  `json:"projectId"`
	CheckID        string  `json:"checkId"`
	Revision       int     `json:"revision"`
	State          string  `json:"state"`
	Method         *string `json:"method"`
	CommandIndex   *int    `json:"commandIndex"`
	RecoveryDigest *string `json:"recoveryDigest"`
	ReasonCode     string  `json:"reasonCode"`
	Message        string  `json:"message"`
}

type ProjectCheckRecoveryRequest struct {
	ExpectedRevision int    `json:"expectedRevision"`
	RecoveryDigest   string `json:"recoveryDigest"`
	AcknowledgeStop  bool   `json:"acknowledgeStop"`
}

type CheckControlError struct {
	Status        int
	Code, Message string
}

func (e *CheckControlError) Error() string { return e.Message }

func (s *Server) GetProjectCheckRun(projectID, checkID string) (ProjectCheckRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.readCheckHistory(projectID)
	if err != nil {
		return ProjectCheckRun{}, err
	}
	for _, row := range rows {
		if row.ID == checkID {
			return snapshotProjectCheck(row), nil
		}
	}
	return ProjectCheckRun{}, ErrProjectRunNotFound
}

func (s *Server) BeginProjectCheckRecovery(projectID, checkID string, revision int) (ProjectCheckRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if job := s.projectChecks[checkID]; job != nil && job.cancel != nil {
		return ProjectCheckRun{}, ErrProjectRunConflict
	}
	rows, err := s.readCheckHistory(projectID)
	if err != nil {
		return ProjectCheckRun{}, err
	}
	for _, row := range rows {
		if row.ID == checkID {
			if row.Revision != revision || revision < 1 {
				return ProjectCheckRun{}, ErrProjectRunConflict
			}
			row.Status, row.Stage, row.ReasonCode, row.Message = "unknown", "recovery", "check_recovery_pending", "The reviewed exact check stop and cleanup is pending. No command will be replayed."
			held := true
			row.ResourcesHeld = &held
			row.Revision++
			row.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
			if err = s.storeCheckRun(row); err != nil {
				return ProjectCheckRun{}, err
			}
			return snapshotProjectCheck(row), nil
		}
	}
	return ProjectCheckRun{}, ErrProjectRunNotFound
}

func writeCheckControlError(w http.ResponseWriter, err error) {
	var known *CheckControlError
	if errors.As(err, &known) && known.Status >= 400 && known.Status <= 503 {
		problem(w, known.Status, known.Code, known.Message)
		return
	}
	if errors.Is(err, ErrProjectRunConflict) {
		problem(w, 409, "check_recovery_conflict", "The check changed or still has a current controller. Read its current state before reviewing another recovery.")
		return
	}
	if errors.Is(err, ErrProjectRunNotFound) {
		problem(w, 404, "check_not_found", "The check was not found.")
		return
	}
	problem(w, 503, "check_recovery_unavailable", "The exact check recovery could not be verified. Its retained capacity has not been released.")
}

func (s *Server) getProjectCheckRecovery(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	if s.projectCheckRuntime.Recovery == nil {
		problem(w, 503, "check_recovery_unavailable", "The check execution controller is not ready.")
		return
	}
	view, err := s.projectCheckRuntime.Recovery(r.Context(), p.ID, r.PathValue("runId"))
	if err != nil {
		writeCheckControlError(w, err)
		return
	}
	writeJSON(w, 200, view)
}

func (s *Server) recoverProjectCheck(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	var req ProjectCheckRecoveryRequest
	if !journeyDecode(w, r, &req) {
		return
	}
	if req.ExpectedRevision < 1 || !hexDigest(req.RecoveryDigest) || !req.AcknowledgeStop {
		problem(w, 400, "check_recovery_review_required", "Review the current exact check and acknowledge stopping its uncertain execution.")
		return
	}
	if s.projectCheckRuntime.Recover == nil {
		problem(w, 503, "check_recovery_unavailable", "The check execution controller is not ready.")
		return
	}
	row, err := s.projectCheckRuntime.Recover(r.Context(), p.ID, r.PathValue("runId"), req)
	if err != nil {
		writeCheckControlError(w, err)
		return
	}
	writeJSON(w, 200, row)
}

// Explicit read/control callbacks are separate from background observations;
// they always receive current request identity, never the original actor token.
type ProjectCheckRecoveryReader func(context.Context, string, string) (ProjectCheckRecoveryView, error)
type ProjectCheckRecoveryWriter func(context.Context, string, string, ProjectCheckRecoveryRequest) (ProjectCheckRun, error)
