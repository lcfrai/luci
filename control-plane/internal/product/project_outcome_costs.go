package product

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

type OutcomeCostInput struct {
	RunID             string `json:"runId"`
	OperationID       string `json:"operationId"`
	Category          string `json:"category"`
	Provider          string `json:"provider"`
	Currency          string `json:"currency"`
	AmountMicros      string `json:"amountMicros"`
	BilledOn          string `json:"billedOn"`
	SourceFileName    string `json:"sourceFileName"`
	SourceSHA256      string `json:"sourceSha256"`
	SourceRow         int    `json:"sourceRow"`
	ProviderReference string `json:"providerReference"`
}
type OutcomeCostReceipt struct {
	OutcomeCostInput
	ID                     string `json:"id"`
	Revision               int    `json:"revision"`
	RunInputDigest         string `json:"runInputDigest"`
	OperationRequestDigest string `json:"operationRequestDigest"`
	ChildID                string `json:"childId"`
	Harness                string `json:"harness"`
	Model                  string `json:"model"`
	RecordedAt             string `json:"recordedAt"`
	UpdatedAt              string `json:"updatedAt"`
	Attribution            string `json:"attribution"`
}
type outcomeCostLedger struct {
	Version             int                          `json:"version"`
	Actor               management.ExecutionActorRef `json:"actor"`
	AccountScope        string                       `json:"accountScope"`
	ProjectScope        string                       `json:"projectScope"`
	Revision            int                          `json:"revision"`
	LatestRequestDigest string                       `json:"latestRequestDigest"`
	Rows                []OutcomeCostReceipt         `json:"rows"`
	OmittedRows         int                          `json:"omittedRows"`
}
type OutcomeCostTotal struct {
	Currency     string `json:"currency"`
	AmountMicros string `json:"amountMicros"`
	ReceiptRows  int    `json:"receiptRows"`
}
type OutcomeCosts struct {
	ProjectID        string               `json:"projectId"`
	Revision         int                  `json:"revision"`
	Rows             []OutcomeCostReceipt `json:"rows"`
	Totals           []OutcomeCostTotal   `json:"totals"`
	OmittedRows      int                  `json:"omittedRows"`
	Attribution      string               `json:"attribution"`
	ProviderVerified bool                 `json:"providerVerified"`
	InputTokens      *int64               `json:"inputTokens"`
	OutputTokens     *int64               `json:"outputTokens"`
}
type OutcomeCostSave struct {
	ExpectedRevision      int               `json:"expectedRevision"`
	ID                    string            `json:"id"`
	Remove                bool              `json:"remove"`
	Input                 *OutcomeCostInput `json:"input"`
	AcknowledgeActualBill bool              `json:"acknowledgeActualBill"`
}

var outcomeCurrency = regexp.MustCompile(`^[A-Z]{3}$`)
var outcomeMoney = regexp.MustCompile(`^(0|[1-9][0-9]{0,15})$`)
var outcomeCostID = regexp.MustCompile(`^cost-[a-zA-Z0-9-]{1,80}$`)

func validOutcomeCostInput(row OutcomeCostInput) bool {
	if !validRunID(row.RunID) || !prototypeReleaseID.MatchString(row.OperationID) || row.Category != "model-api" && row.Category != "infrastructure" || strings.TrimSpace(row.Provider) == "" || len(row.Provider) > 64 || !validArtifactStoredText(row.Provider) || !outcomeCurrency.MatchString(row.Currency) || !outcomeMoney.MatchString(row.AmountMicros) || !hexDigest(row.SourceSHA256) || row.SourceRow < 1 || row.SourceRow > 1e6 || row.SourceFileName == "" || len(row.SourceFileName) > 128 || strings.ContainsAny(row.SourceFileName, "/\\\r\n\x00\x7f") || !validArtifactStoredText(row.SourceFileName) || len(row.ProviderReference) > 128 || !validArtifactStoredText(row.ProviderReference) {
		return false
	}
	amount, err := strconv.ParseInt(row.AmountMicros, 10, 64)
	if err != nil || amount < 0 || amount > 1e15 {
		return false
	}
	date, err := time.Parse("2006-01-02", row.BilledOn)
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return err == nil && date.Year() >= 2000 && !date.After(today)
}
func costSourceKey(row OutcomeCostInput) string {
	return reviewDigest([]string{row.SourceSHA256, strconv.Itoa(row.SourceRow), row.Category})
}
func (s *Server) readOutcomeCostLedger(actor management.ExecutionActorRef) (string, outcomeCostLedger, error) {
	p, err := s.resolveProject(actor.ProjectID)
	empty := outcomeCostLedger{Version: 1, Actor: actor, Rows: []OutcomeCostReceipt{}}
	if err != nil {
		return "", empty, err
	}
	s.mu.RLock()
	scopes, err := s.approvalScopes(p.Path, p.ID)
	s.mu.RUnlock()
	if err != nil {
		return "", empty, err
	}
	empty.AccountScope, empty.ProjectScope = scopes.AccountScope, scopes.ProjectScope
	path, err := privateApplicationStatePathForRoot(s.config.ProjectsDir, s.stateDir, "outcome-costs-"+reviewDigest([]string{actor.UserID, actor.TeamID, actor.ProjectID}))
	if err != nil {
		return "", empty, err
	}
	var saved outcomeCostLedger
	err = externalReadJSON(path, 1<<20, &saved)
	if errors.Is(err, os.ErrNotExist) {
		return path, empty, nil
	}
	if err != nil || saved.Version != 1 || saved.Actor != actor || saved.AccountScope != empty.AccountScope || saved.ProjectScope != empty.ProjectScope || saved.Revision < 1 || saved.Revision > 1e9 || !hexDigest(saved.LatestRequestDigest) || saved.Rows == nil || len(saved.Rows) > 256 || saved.OmittedRows < 0 || saved.OmittedRows > 1e9 {
		return "", empty, ErrProjectRunInvalid
	}
	ids, source := map[string]bool{}, map[string]bool{}
	for _, row := range saved.Rows {
		if !validOutcomeCostInput(row.OutcomeCostInput) || !outcomeCostID.MatchString(row.ID) || ids[row.ID] || source[costSourceKey(row.OutcomeCostInput)] || row.Revision < 1 || row.Revision > 1e9 || !hexDigest(row.RunInputDigest) || !hexDigest(row.OperationRequestDigest) || row.ChildID == "" || len(row.ChildID) > 128 || row.Attribution != "operator-reconciled" || len(row.Harness) > 128 || len(row.Model) > 256 {
			return "", empty, ErrProjectRunInvalid
		}
		for _, at := range []string{row.RecordedAt, row.UpdatedAt} {
			if _, err := time.Parse(time.RFC3339Nano, at); err != nil {
				return "", empty, ErrProjectRunInvalid
			}
		}
		ids[row.ID] = true
		source[costSourceKey(row.OutcomeCostInput)] = true
	}
	return path, saved, nil
}
func outcomeCostView(actor management.ExecutionActorRef, ledger outcomeCostLedger) OutcomeCosts {
	totals := map[string]int64{}
	counts := map[string]int{}
	for _, row := range ledger.Rows {
		amount, _ := strconv.ParseInt(row.AmountMicros, 10, 64)
		totals[row.Currency] += amount
		counts[row.Currency]++
	}
	result := OutcomeCosts{ProjectID: actor.ProjectID, Revision: ledger.Revision, Rows: append([]OutcomeCostReceipt{}, ledger.Rows...), Totals: []OutcomeCostTotal{}, OmittedRows: ledger.OmittedRows, Attribution: "operator-reconciled-retained-window", ProviderVerified: false}
	for currency, amount := range totals {
		result.Totals = append(result.Totals, OutcomeCostTotal{Currency: currency, AmountMicros: strconv.FormatInt(amount, 10), ReceiptRows: counts[currency]})
	}
	sort.Slice(result.Totals, func(i, j int) bool { return result.Totals[i].Currency < result.Totals[j].Currency })
	return result
}
func (s *Server) OutcomeCosts(ctx context.Context, actor management.ExecutionActorRef) (OutcomeCosts, error) {
	if s.feedbackAccess(ctx, actor, true) != nil {
		return OutcomeCosts{}, management.ErrExecutionAuthority
	}
	if s.projectRuns == nil {
		return OutcomeCosts{}, ErrProjectRunInvalid
	}
	s.outcomeCostsMu.Lock()
	_, ledger, err := s.readOutcomeCostLedger(actor)
	s.outcomeCostsMu.Unlock()
	if err != nil || s.feedbackAccess(ctx, actor, true) != nil {
		return OutcomeCosts{}, ErrProjectRunInvalid
	}
	return outcomeCostView(actor, ledger), nil
}
func (s *Server) SaveOutcomeCost(ctx context.Context, actor management.ExecutionActorRef, input OutcomeCostSave) (OutcomeCosts, error) {
	if s.feedbackAccess(ctx, actor, true) != nil {
		return OutcomeCosts{}, management.ErrExecutionAuthority
	}
	if s.projectRuns == nil {
		return OutcomeCosts{}, ErrProjectRunInvalid
	}
	if input.ExpectedRevision < 0 || input.ExpectedRevision >= 1e9 || input.ID != "" && !outcomeCostID.MatchString(input.ID) || input.Remove && (input.ID == "" || input.Input != nil) || !input.Remove && (input.Input == nil || !input.AcknowledgeActualBill || !validOutcomeCostInput(*input.Input)) {
		return OutcomeCosts{}, ErrProjectRunInvalid
	}
	var receipt OutcomeCostReceipt
	if !input.Remove {
		p, err := s.resolveProject(actor.ProjectID)
		if err != nil {
			return OutcomeCosts{}, err
		}
		run, err := s.projectRuns.Get(p.ID, p.Path, input.Input.RunID)
		if err != nil {
			return OutcomeCosts{}, err
		}
		var operation *ProjectRunOperation
		for i := range run.Operations {
			if run.Operations[i].ID == input.Input.OperationID {
				operation = &run.Operations[i]
				break
			}
		}
		if operation == nil || operation.Kind != "session" || operation.ChildID == "" {
			return OutcomeCosts{}, ErrProjectRunInvalid
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		receipt = OutcomeCostReceipt{OutcomeCostInput: *input.Input, ID: input.ID, Revision: 1, RunInputDigest: reviewDigest(run.Input), OperationRequestDigest: operation.RequestDigest, ChildID: operation.ChildID, Harness: run.Input.Route.Harness, Model: run.Input.Route.Model, RecordedAt: now, UpdatedAt: now, Attribution: "operator-reconciled"}
		if receipt.ID == "" {
			receipt.ID = newID("cost-")
		}
	}
	requestDigest := reviewDigest(input)
	s.outcomeCostsMu.Lock()
	path, ledger, err := s.readOutcomeCostLedger(actor)
	if err != nil {
		s.outcomeCostsMu.Unlock()
		return OutcomeCosts{}, err
	}
	if ledger.LatestRequestDigest == requestDigest {
		s.outcomeCostsMu.Unlock()
		if s.feedbackAccess(ctx, actor, true) != nil {
			return OutcomeCosts{}, management.ErrExecutionAuthority
		}
		return outcomeCostView(actor, ledger), nil
	}
	if ledger.Revision != input.ExpectedRevision {
		s.outcomeCostsMu.Unlock()
		return OutcomeCosts{}, ErrProjectRunConflict
	}
	if !input.Remove {
		p, resolveErr := s.resolveProject(actor.ProjectID)
		if resolveErr != nil {
			s.outcomeCostsMu.Unlock()
			return OutcomeCosts{}, ErrProjectRunInvalid
		}
		run, readErr := s.projectRuns.Get(p.ID, p.Path, receipt.RunID)
		matched := false
		if readErr == nil && reviewDigest(run.Input) == receipt.RunInputDigest {
			for _, operation := range run.Operations {
				if operation.ID == receipt.OperationID && operation.ChildID == receipt.ChildID && operation.RequestDigest == receipt.OperationRequestDigest {
					matched = true
					break
				}
			}
		}
		if !matched {
			s.outcomeCostsMu.Unlock()
			return OutcomeCosts{}, ErrProjectRunConflict
		}
	}
	found := false
	rows := []OutcomeCostReceipt{}
	for _, old := range ledger.Rows {
		if old.ID == input.ID {
			found = true
			if input.Remove {
				continue
			}
			receipt.Revision = old.Revision + 1
			receipt.RecordedAt = old.RecordedAt
			if receipt.Revision > 1e9 {
				s.outcomeCostsMu.Unlock()
				return OutcomeCosts{}, ErrProjectRunLimit
			}
			rows = append(rows, receipt)
			continue
		}
		if !input.Remove && costSourceKey(old.OutcomeCostInput) == costSourceKey(receipt.OutcomeCostInput) {
			s.outcomeCostsMu.Unlock()
			return OutcomeCosts{}, ErrProjectRunConflict
		}
		rows = append(rows, old)
	}
	if input.ID != "" && !found {
		s.outcomeCostsMu.Unlock()
		return OutcomeCosts{}, ErrProjectRunNotFound
	}
	if input.ID == "" {
		rows = append(rows, receipt)
	}
	if len(rows) > 256 {
		rows = append([]OutcomeCostReceipt{}, rows[len(rows)-256:]...)
		if ledger.OmittedRows < 1e9 {
			ledger.OmittedRows++
		}
	}
	ledger.Rows = rows
	ledger.Revision++
	ledger.LatestRequestDigest = requestDigest
	raw, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil || len(raw)+1 > 1<<20 || s.feedbackAccess(ctx, actor, true) != nil {
		s.outcomeCostsMu.Unlock()
		return OutcomeCosts{}, ErrProjectRunInvalid
	}
	err = durableRunJSON(path, ledger)
	s.outcomeCostsMu.Unlock()
	if err != nil || s.feedbackAccess(ctx, actor, true) != nil {
		return OutcomeCosts{}, ErrProjectRunInvalid
	}
	return outcomeCostView(actor, ledger), nil
}
