package product

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

const overviewQuestionProjectionLimit = 32 << 10
const overviewLegacyConversationLimit = 256 << 10

var errOverviewQuestions = errors.New("bounded private conversation overview is unavailable")

type overviewQuestionProjection struct {
	Version        int                       `json:"version"`
	AccountScope   string                    `json:"accountScope"`
	ProjectScope   string                    `json:"projectScope"`
	ProjectID      string                    `json:"projectId"`
	Revision       int                       `json:"revision"`
	LedgerSize     int64                     `json:"ledgerSize"`
	LedgerModified string                    `json:"ledgerModified"`
	LedgerIdentity string                    `json:"ledgerIdentity"`
	Count          int                       `json:"count"`
	Items          []ProjectOverviewQuestion `json:"items"`
}

func (s *Server) overviewQuestionsPath(id string, create bool) (string, error) {
	if !prototypeReleaseID.MatchString(id) {
		return "", errOverviewQuestions
	}
	root, err := guardedProjectPath(s.stateDir, "project-overview-questions", true)
	if err != nil {
		return "", err
	}
	if create {
		if err = os.Mkdir(root, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return "", err
		}
	}
	if _, err = guardedProjectPath(s.stateDir, "project-overview-questions", false); err != nil {
		return "", err
	}
	if create && runtime.GOOS == "windows" {
		if err = restrictPrivateFile(root); err != nil {
			return "", err
		}
	}
	return guardedProjectPath(s.stateDir, "project-overview-questions/"+id+".json", create)
}
func projectOverviewQuestionProjection(saved orchestratorLedger) overviewQuestionProjection {
	result := overviewQuestionProjection{Version: 1, AccountScope: saved.AccountScope, ProjectScope: saved.ProjectScope, ProjectID: saved.ProjectID, Revision: saved.Revision, Items: []ProjectOverviewQuestion{}}
	for _, q := range orchestratorQuestions(saved) {
		result.Count++
		if len(result.Items) < projectOverviewQuestionLimit {
			text, truncated := overviewText(q.Question, projectOverviewTextLimit)
			result.Items = append(result.Items, ProjectOverviewQuestion{ID: q.TurnID, Source: "orchestrator", Text: text, Truncated: truncated})
		}
	}
	return result
}

// Best-effort secondary index after a successful canonical write. Failure cannot
// retroactively fail an admitted turn: an absent/stale projection is reported
// as partial by overview reads. The execution gate always reads the real ledger.
func (s *Server) writeOverviewQuestions(saved orchestratorLedger, ledgerPath string) error {
	file, err := openOverviewRegular(ledgerPath, orchestratorFileLimit)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return errOverviewQuestions
	}
	identity, err := projectOverviewFileIdentity(file)
	if err != nil {
		return err
	}
	row := projectOverviewQuestionProjection(saved)
	row.LedgerIdentity = identity
	row.LedgerSize = info.Size()
	row.LedgerModified = info.ModTime().UTC().Format(time.RFC3339Nano)
	path, err := s.overviewQuestionsPath(saved.ProjectID, true)
	if err != nil {
		return err
	}
	if err = hostpath.OneName(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	data, err := json.Marshal(row)
	if err != nil || len(data) > overviewQuestionProjectionLimit {
		return errOverviewQuestions
	}
	if err = atomicJSON(path, row); err != nil {
		return err
	}
	return syncProjectRunDirectory(filepath.Dir(path))
}
func (s *Server) readOverviewQuestions(ctx context.Context, p project.Project) (overviewQuestionProjection, error) {
	if err := ctx.Err(); err != nil {
		return overviewQuestionProjection{}, err
	}
	scope, err := s.approvalScopes(p.Path, p.ID)
	if err != nil {
		return overviewQuestionProjection{}, errOverviewQuestions
	}
	if err := ctx.Err(); err != nil {
		return overviewQuestionProjection{}, err
	}
	ledgerPath, err := s.orchestratorPath(p.ID, false)
	if errors.Is(err, os.ErrNotExist) {
		return overviewQuestionProjection{Items: []ProjectOverviewQuestion{}}, nil
	}
	if err != nil || hostpath.OneName(ledgerPath) != nil {
		return overviewQuestionProjection{}, errOverviewQuestions
	}
	ledger, err := openOverviewRegular(ledgerPath, orchestratorFileLimit)
	if errors.Is(err, os.ErrNotExist) {
		return overviewQuestionProjection{Items: []ProjectOverviewQuestion{}}, nil
	}
	if err != nil {
		return overviewQuestionProjection{}, errOverviewQuestions
	}
	defer ledger.Close()
	info, err := ledger.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > orchestratorFileLimit {
		return overviewQuestionProjection{}, errOverviewQuestions
	}
	if err := ctx.Err(); err != nil {
		return overviewQuestionProjection{}, err
	}
	identity, identityErr := projectOverviewFileIdentity(ledger)
	path, pathErr := s.overviewQuestionsPath(p.ID, false)
	if identityErr == nil && pathErr == nil && hostpath.OneName(path) == nil {
		file, openErr := openOverviewRegular(path, overviewQuestionProjectionLimit)
		if openErr == nil {
			data, readErr := io.ReadAll(io.LimitReader(file, overviewQuestionProjectionLimit+1))
			file.Close()
			if err := ctx.Err(); err != nil {
				return overviewQuestionProjection{}, err
			}
			var row overviewQuestionProjection
			decoder := json.NewDecoder(bytes.NewReader(data))
			decoder.DisallowUnknownFields()
			if readErr == nil && len(data) <= overviewQuestionProjectionLimit && decoder.Decode(&row) == nil && decoder.Decode(new(any)) == io.EOF && validOverviewQuestionProjection(row, scope, info, identity) {
				return row, nil
			}
		}
	}
	// Existing installs do not need a write-on-read migration. Small ledgers can
	// be read through the same validator as the conversation; large legacy ones
	// are explicitly partial until a normal conversation write creates the index.
	if info.Size() > overviewLegacyConversationLimit {
		return overviewQuestionProjection{}, errOverviewQuestions
	}
	if err := ctx.Err(); err != nil {
		return overviewQuestionProjection{}, err
	}
	data, err := io.ReadAll(io.LimitReader(ledger, overviewLegacyConversationLimit+1))
	if err != nil || len(data) > overviewLegacyConversationLimit {
		return overviewQuestionProjection{}, errOverviewQuestions
	}
	if err := ctx.Err(); err != nil {
		return overviewQuestionProjection{}, err
	}
	saved, err := decodeOrchestratorLedger(data, orchestratorLedger{Version: 1, AccountScope: scope.AccountScope, ProjectScope: scope.ProjectScope, ProjectID: p.ID})
	if err != nil {
		return overviewQuestionProjection{}, errOverviewQuestions
	}
	return projectOverviewQuestionProjection(saved), nil
}
func validOverviewQuestionProjection(row overviewQuestionProjection, scope journeyApprovalStore, info os.FileInfo, identity string) bool {
	if row.Version != 1 || row.AccountScope != scope.AccountScope || row.ProjectScope != scope.ProjectScope || row.ProjectID != scope.ProjectID || row.Revision < 0 || row.LedgerSize != info.Size() || row.LedgerModified != info.ModTime().UTC().Format(time.RFC3339Nano) || row.LedgerIdentity != identity || row.Count < 0 || row.Count > orchestratorTurnLimit || len(row.Items) > projectOverviewQuestionLimit || len(row.Items) > row.Count {
		return false
	}
	expected := row.Count
	if expected > projectOverviewQuestionLimit {
		expected = projectOverviewQuestionLimit
	}
	if len(row.Items) != expected {
		return false
	}
	seen := map[string]bool{}
	for _, q := range row.Items {
		if !prototypeReleaseID.MatchString(q.ID) || seen[q.ID] || q.Source != "orchestrator" || len(q.Text) == 0 || len(q.Text) > projectOverviewTextLimit {
			return false
		}
		seen[q.ID] = true
	}
	return true
}

// Refuse devices, directories and links before opening a private projection.
// Recheck the opened identity so replacement cannot validate a different file.
func openOverviewRegular(path string, limit int64) (*os.File, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() || before.Size() > limit || hostpath.OneName(path) != nil {
		return nil, errOverviewQuestions
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	after, err := file.Stat()
	if err != nil || !after.Mode().IsRegular() || after.Size() > limit || !os.SameFile(before, after) {
		file.Close()
		return nil, errOverviewQuestions
	}
	return file, nil
}
