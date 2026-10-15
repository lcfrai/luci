package product

import (
	"errors"
	"net/http"
	"time"
)

func (s *Server) registerOrchestrationContracts(mux Registrar) {
	base := "/v1/product/projects/{id}/orchestration-contracts"
	mux.HandleFunc("POST "+base+"/prepare", s.prepareOrchestrationContract)
	mux.HandleFunc("GET "+base+"/{contractId}", s.getOrchestrationContract)
	mux.HandleFunc("POST "+base+"/{contractId}/approve", s.approveOrchestrationContract)
}

func orchestrationContractProblem(w http.ResponseWriter, err error) {
	if errors.Is(err, errCoordinatorAuthority) {
		journeyError(w, 403, "orchestration_contract_forbidden", "Current project edit authority is required.")
		return
	}
	if errors.Is(err, ErrProjectRunNotFound) {
		journeyError(w, 404, "orchestration_contract_not_found", "This execution contract is unavailable in your workspace.")
		return
	}
	if errors.Is(err, ErrProjectRunLimit) {
		journeyError(w, 409, "orchestration_contract_history_full", "This project's private execution contract history reached its verified size limit. Existing contracts remain readable; archive this project before preparing more work.")
		return
	}
	if errors.Is(err, ErrProjectRunInvalid) {
		journeyError(w, 503, "orchestration_contract_unavailable", "Private execution contract history is unavailable or invalid.")
		return
	}
	journeyError(w, 409, "orchestration_contract_changed", err.Error())
}

func (s *Server) prepareOrchestrationContract(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	actor, ok := authenticatedContextActor(r.Context())
	if !ok || s.coordinatorAuthority(r.Context(), p.ID) != nil {
		orchestrationContractProblem(w, errCoordinatorAuthority)
		return
	}
	var req OrchestrationPrepareRequest
	if !journeyDecode(w, r, &req) {
		return
	}
	if req.ImplementationContext.ProjectID != p.ID || normalizeOrchestrationPrepare(&req) != nil {
		journeyError(w, 400, "invalid_orchestration_contract", "Choose current saved tasks, a route, outcome and a fresh request key.")
		return
	}
	keyDigest, requestDigest := reviewDigest(req.IdempotencyKey), reviewDigest(req)
	s.orchestrationContractMu.Lock()
	_, ledger, err := s.readOrchestrationContracts(p)
	s.orchestrationContractMu.Unlock()
	if err != nil {
		orchestrationContractProblem(w, err)
		return
	}
	for _, row := range ledger.Records {
		if row.Contract.Actor.ID == actor.ID && row.KeyDigest == keyDigest {
			if row.RequestDigest != requestDigest {
				orchestrationContractProblem(w, ErrProjectRunKeyConflict)
				return
			}
			journeyJSON(w, 200, s.contractView(r.Context(), p, row.Contract))
			return
		}
	}
	input, err := s.prepareOrchestrationInput(r.Context(), p, req)
	if err != nil {
		orchestrationContractProblem(w, err)
		return
	}
	contract := OrchestrationContract{ID: newID("contract-"), Revision: 1, Digest: reviewDigest(input), State: "prepared", ProjectID: p.ID, Actor: actor, Input: input, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	if s.contractView(r.Context(), p, contract).Freshness.Status != "current" {
		orchestrationContractProblem(w, ErrProjectRunConflict)
		return
	}
	s.orchestrationContractMu.Lock()
	defer s.orchestrationContractMu.Unlock()
	path, ledger, err := s.readOrchestrationContracts(p)
	if err != nil {
		orchestrationContractProblem(w, err)
		return
	}
	for _, row := range ledger.Records {
		if row.Contract.Actor.ID == actor.ID && row.KeyDigest == keyDigest {
			if row.RequestDigest != requestDigest {
				orchestrationContractProblem(w, ErrProjectRunKeyConflict)
				return
			}
			journeyJSON(w, 200, s.contractView(r.Context(), p, row.Contract))
			return
		}
	}
	if len(ledger.Records) >= orchestrationContractLimit {
		orchestrationContractProblem(w, ErrProjectRunLimit)
		return
	}
	ledger.Records = append(ledger.Records, orchestrationContractRecord{Contract: contract, KeyDigest: keyDigest, RequestDigest: requestDigest})
	if err := writeOrchestrationContracts(path, ledger); err != nil {
		orchestrationContractProblem(w, err)
		return
	}
	journeyJSON(w, 201, s.contractView(r.Context(), p, contract))
}

func (s *Server) getOrchestrationContract(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	actor, ok := authenticatedContextActor(r.Context())
	if !ok {
		orchestrationContractProblem(w, errCoordinatorAuthority)
		return
	}
	s.orchestrationContractMu.Lock()
	_, ledger, err := s.readOrchestrationContracts(p)
	s.orchestrationContractMu.Unlock()
	if err != nil {
		orchestrationContractProblem(w, err)
		return
	}
	c, err := orchestrationContractRecordByID(ledger, r.PathValue("contractId"), actor.ID)
	if err != nil {
		orchestrationContractProblem(w, err)
		return
	}
	journeyJSON(w, 200, s.contractView(r.Context(), p, *c))
}

func (s *Server) approveOrchestrationContract(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	actor, ok := authenticatedContextActor(r.Context())
	if !ok || s.coordinatorAuthority(r.Context(), p.ID) != nil {
		orchestrationContractProblem(w, errCoordinatorAuthority)
		return
	}
	var req struct {
		ExpectedRevision int    `json:"expectedRevision"`
		ExpectedDigest   string `json:"expectedDigest"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	s.orchestrationContractMu.Lock()
	defer s.orchestrationContractMu.Unlock()
	path, ledger, err := s.readOrchestrationContracts(p)
	if err != nil {
		orchestrationContractProblem(w, err)
		return
	}
	for i := range ledger.Records {
		c := &ledger.Records[i].Contract
		if c.ID != r.PathValue("contractId") || c.Actor.ID != actor.ID {
			continue
		}
		if c.Digest != req.ExpectedDigest || req.ExpectedRevision != c.Revision && !(c.State == "approved" && req.ExpectedRevision == 1) {
			orchestrationContractProblem(w, ErrProjectRunConflict)
			return
		}
		if s.contractView(r.Context(), p, *c).Freshness.Status != "current" {
			orchestrationContractProblem(w, ErrProjectRunConflict)
			return
		}
		if !s.orchestrationSourceCurrent(r.Context(), *c) {
			orchestrationContractProblem(w, ErrProjectRunConflict)
			return
		}
		if c.State == "approved" {
			journeyJSON(w, 200, s.contractView(r.Context(), p, *c))
			return
		}
		c.Revision, c.State = 2, "approved"
		c.Approval = &OrchestrationApproval{ID: newID("approval-"), Actor: actor, At: time.Now().UTC().Format(time.RFC3339Nano), Digest: c.Digest}
		if err := writeOrchestrationContracts(path, ledger); err != nil {
			orchestrationContractProblem(w, err)
			return
		}
		journeyJSON(w, 200, s.contractView(r.Context(), p, *c))
		return
	}
	orchestrationContractProblem(w, ErrProjectRunNotFound)
}
