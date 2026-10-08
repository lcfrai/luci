package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"

	"github.com/lcfrai/luci/control-plane/internal/httpx"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// Plans, including task dependencies, intended paths and acceptance, are bounded
// before persistence/import to 5 MiB of actual indented JSON including escaping.
// The former maximum task shape still fits that bound. Specification and design
// are each bounded to 500KB compact JSON. Brief text is bounded to 500KB before
// escaping, or derived from that bounded specification. The binding fits the
// 1MiB manifest limit. A compact document plus binding therefore fits this 7MiB
// envelope with room for response fields, under the existing 8MiB team proxy
// limit. No supported document is truncated or doubled as an encoded JSON string.
const reviewedInspectionLimit = 7 << 20

type reviewedContextInspection struct {
	SessionID             string                          `json:"sessionId"`
	ProjectID             string                          `json:"projectId"`
	WorktreeID            string                          `json:"worktreeId"`
	Version               int                             `json:"version"`
	Digest                string                          `json:"digest"`
	ImplementationContext *registry.ImplementationContext `json:"implementationContext"`
	Document              reviewedContextDocument         `json:"document"`
}

type reviewedContextDocument struct {
	Kind   string          `json:"kind"`
	SHA256 string          `json:"sha256"`
	Bytes  int             `json:"bytes"`
	Value  json.RawMessage `json:"value"`
}

func reviewedDocumentPath(selector string) string {
	switch selector {
	case "brief":
		return ".luci/brief.json"
	case "definition":
		return ".luci/specification.json"
	case "design":
		return ".luci/design-handoff.json"
	case "plan":
		return ".luci/plan.json"
	default:
		return ""
	}
}

// Explicit inspection verifies the entire retained bundle without asking an
// agent to read it. In particular, do not call ValidateImplementationContext:
// newer collaborative amendments must not make historical captured A unreadable.
// There are no source/provider lookups, repair, execution, or persistence here.
func (a *API) handleSessionReviewedContext(w http.ResponseWriter, r *http.Request) {
	sess, ok := a.sessionByID(w, r)
	if !ok {
		return
	}
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil || len(query) > 1 || len(query["document"]) > 1 {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.CodeBadRequest, "Choose one captured document: brief, definition, design or plan.", nil)
		return
	}
	selector := "definition"
	if values, present := query["document"]; present && len(values) == 1 {
		selector = values[0]
	} else if len(query) != 0 {
		selector = ""
	}
	path := reviewedDocumentPath(selector)
	if path == "" {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.CodeBadRequest, "Choose one captured document: brief, definition, design or plan.", nil)
		return
	}
	if sess.ImplementationContext == nil || sess.ReviewedContext == nil {
		httpx.WriteProblem(w, http.StatusConflict, httpx.CodeConflict, "This session has no verified captured documents. Its recorded revisions and tasks remain available, but current project files cannot establish what it received.", nil)
		return
	}
	if sess.ProjectID != sess.ImplementationContext.ProjectID || sess.WorktreeID != sess.ImplementationContext.WorktreeID {
		a.reviewedInspectionProblem(w, sess.ID, errors.New("captured context owner differs from the session"))
		return
	}
	capture, err := a.readReviewedContext(sess.ID, sess.ImplementationContext, sess.ReviewedContext)
	if err != nil {
		a.reviewedInspectionProblem(w, sess.ID, err)
		return
	}
	for _, file := range capture.Files {
		if file.Path != path {
			continue
		}
		// Captures are generated JSON objects, not arbitrary content, HTML or
		// private file selectors. Validate again before embedding exact JSON.
		var object map[string]json.RawMessage
		if json.Unmarshal(file.Data, &object) != nil || object == nil {
			a.reviewedInspectionProblem(w, sess.ID, errors.New("captured document is not a JSON object"))
			return
		}
		out := reviewedContextInspection{
			SessionID: sess.ID, ProjectID: sess.ProjectID, WorktreeID: sess.WorktreeID,
			Version: sess.ReviewedContext.Version, Digest: sess.ReviewedContext.Digest,
			ImplementationContext: capture.Binding,
			Document:              reviewedContextDocument{Kind: selector, SHA256: reviewedHash(file.Data), Bytes: len(file.Data), Value: file.Data},
		}
		data, err := json.Marshal(out)
		if err != nil || len(data)+1 > reviewedInspectionLimit {
			a.reviewedInspectionProblem(w, sess.ID, errors.New("captured document exceeds the supported inspection envelope"))
			return
		}
		if r.Context().Err() != nil {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(append(data, '\n'))
		return
	}
	a.reviewedInspectionProblem(w, sess.ID, errors.New("captured document is missing"))
}

func (a *API) reviewedInspectionProblem(w http.ResponseWriter, sessionID string, err error) {
	a.sessionLog().Warn("captured document inspection failed", "sessionId", sessionID, "err", err)
	httpx.WriteProblem(w, http.StatusConflict, httpx.CodeConflict, "The captured inputs for this session could not be verified. Its recorded revisions and tasks remain available. No current project files were substituted and nothing was changed.", nil)
}
