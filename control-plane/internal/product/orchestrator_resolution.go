package product

import (
	"context"
	"errors"
	"strings"
	"time"
)

var ErrOrchestratorNotSent = errors.New("This request was confirmed not sent and closed. Review the current context and use a new request key to send a new message.")

const orchestratorNoDispatchLimit = 1024

// These fences are retained in the active ledger through every history rotation.
// They contain hashes only and never authorize or describe a provider operation.
type orchestratorNoDispatch struct {
	KeyDigest     string `json:"keyDigest"`
	RequestDigest string `json:"requestDigest"`
	CreatedAt     string `json:"createdAt"`
}

type OrchestratorRequestResolution struct {
	Status  string
	Turn    *OrchestratorTurn
	Message string
}

func validateOrchestratorNoDispatch(rows []orchestratorNoDispatch) error {
	if len(rows) > orchestratorNoDispatchLimit {
		return ErrOrchestratorCapacity
	}
	seen := map[string]bool{}
	for _, row := range rows {
		_, err := time.Parse(time.RFC3339Nano, row.CreatedAt)
		if err != nil || !hexDigest(row.KeyDigest) || !hexDigest(row.RequestDigest) || seen[row.KeyDigest] {
			return ErrOrchestratorInvalid
		}
		seen[row.KeyDigest] = true
	}
	return nil
}

// Current and archived copies of the same turn may differ in completion state.
// Their original request and child identities must agree; prefer the active copy.
func findOrchestratorRequest(storage orchestratorExport, key, digest string) (*storedOrchestratorTurn, error) {
	ledgers := []orchestratorLedger{storage.Ledger}
	for _, archive := range storage.Archive {
		ledgers = append(ledgers, archive.Ledger)
	}
	var found *storedOrchestratorTurn
	for _, ledger := range ledgers {
		for _, turn := range ledger.Turns {
			if turn.KeyDigest != key {
				continue
			}
			if turn.RequestDigest != digest || found != nil && (turn.ID != found.ID || turn.JobID != found.JobID || turn.Actor.ID != found.Actor.ID) {
				return nil, ErrOrchestratorConflict
			}
			if found == nil {
				copy := turn
				found = &copy
			}
		}
	}
	return found, nil
}

// A gap means a prior explicit history rotation discarded receipts. Absence
// from the latest page/archive cannot then establish that an old key was unused.
func completeOrchestratorRequestHistory(storage orchestratorExport) bool {
	if len(storage.Staging) != 0 || storage.Ledger.Revision == 0 && len(storage.Archive) != 0 {
		return false
	}
	sequences := map[int]string{}
	maximum := 0
	ledgers := []orchestratorLedger{storage.Ledger}
	for _, archive := range storage.Archive {
		ledgers = append(ledgers, archive.Ledger)
	}
	for _, ledger := range ledgers {
		for _, turn := range ledger.Turns {
			if previous, ok := sequences[turn.Sequence]; ok && previous != turn.ID {
				return false
			}
			sequences[turn.Sequence] = turn.ID
			if turn.Sequence > maximum {
				maximum = turn.Sequence
			}
		}
	}
	return len(sequences) == maximum
}

// ResolveOrchestratorRequest never creates a turn or job. The API holds its
// admission mutex across this operation and the matching private job read.
// A durable fence, written under the same ledger lock used by reservation,
// makes a confirmed absence safe even when the original POST arrives late.
func (s *Server) ResolveOrchestratorRequest(ctx context.Context, id string, req OrchestratorTurnRequest) (OrchestratorRequestResolution, error) {
	unknown := OrchestratorRequestResolution{Status: "unknown", Message: "This request could not be confirmed. Keep its original request and check again; no inference was repeated."}
	actor, ok := authenticatedContextActor(ctx)
	req.Message = strings.TrimSpace(req.Message)
	if !ok || !validRunKey(req.IdempotencyKey) || !hexDigest(req.ExpectedContextDigest) || req.ExpectedRevision < 0 || len(req.Message) == 0 || len(req.Message) > 16000 || !hexDigest(req.RouteDigest) {
		return unknown, ErrOrchestratorInvalid
	}
	p, err := s.resolveProject(id)
	if err != nil {
		return unknown, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	saved, err := s.readOrchestrator(p)
	if err != nil {
		return unknown, err
	}
	storage, err := s.orchestratorStorageLocked(p, saved)
	if err != nil {
		return unknown, err
	}
	key := reviewDigest(actor.ID + ":" + req.IdempotencyKey)
	digest := reviewDigest([]any{actor.ID, req.Message, req.ReplyToTurnID, req.ExpectedContextDigest, req.RouteDigest})
	exactDigest := reviewDigest([]any{digest, req.ExpectedRevision})
	for _, fence := range saved.NoDispatch {
		if fence.KeyDigest == key {
			if fence.RequestDigest != exactDigest {
				return unknown, ErrOrchestratorConflict
			}
			return OrchestratorRequestResolution{Status: "not-sent", Message: "This exact request was not accepted and is now closed. It cannot start later. Review the current context before sending a new message."}, nil
		}
	}
	turn, err := findOrchestratorRequest(storage, key, digest)
	if err == nil && turn == nil {
		turn, err = s.archivedCoordinationRequestLocked(p, key, digest)
	}
	if err != nil {
		return unknown, err
	}
	if turn != nil {
		unknown.Turn = &turn.OrchestratorTurn
		if turn.Actor.ID != actor.ID || turn.AdmissionRevision == nil || *turn.AdmissionRevision != req.ExpectedRevision {
			unknown.Message = "A retained request has this identity, but its exact original revision could not be confirmed. Keep the original request; no inference was repeated."
			return unknown, nil
		}
		return OrchestratorRequestResolution{Status: "accepted", Turn: &turn.OrchestratorTurn, Message: "The original request was accepted. Its existing receipt is returned without starting work."}, nil
	}
	// A copied key from another actor is not proof of this actor's request.
	// Disclose no foreign identity and do not fence or alter either receipt.
	ledgers := []orchestratorLedger{saved}
	for _, archive := range storage.Archive {
		ledgers = append(ledgers, archive.Ledger)
	}
	for _, ledger := range ledgers {
		for _, other := range ledger.Turns {
			if other.Actor.ID != actor.ID && other.KeyDigest == reviewDigest(other.Actor.ID+":"+req.IdempotencyKey) {
				return unknown, nil
			}
		}
	}
	foreign, err := s.archivedCoordinationForeignRequestLocked(p, actor.ID, req.IdempotencyKey)
	if err != nil {
		return unknown, err
	}
	if foreign {
		return unknown, nil
	}
	if !completeOrchestratorRequestHistory(storage) {
		unknown.Message = "Retained history cannot establish whether this request was accepted. Keep the original request; no inference was repeated."
		return unknown, nil
	}
	if len(saved.NoDispatch) >= orchestratorNoDispatchLimit {
		unknown.Message = "Request recovery storage is full. Ask the workspace operator to retain and reconcile its receipts; no request was discarded or repeated."
		return unknown, nil
	}
	saved.NoDispatch = append(saved.NoDispatch, orchestratorNoDispatch{KeyDigest: key, RequestDigest: exactDigest, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)})
	saved.Revision++
	if err = s.saveOrchestrator(saved); err != nil {
		return unknown, err
	}
	return OrchestratorRequestResolution{Status: "not-sent", Message: "This exact request was not accepted and is now closed. It cannot start later. Review the current context before sending a new message."}, nil
}
