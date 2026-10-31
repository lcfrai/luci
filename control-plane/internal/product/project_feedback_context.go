package product

import (
	"encoding/json"
	"strings"
	"unicode/utf8"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

// Imported issue text is untrusted historical evidence. This projection never
// grants access to GitHub, tools, execution or approval of a proposed amendment.
type OrchestratorFeedbackItem struct {
	ID                    string `json:"id"`
	Digest                string `json:"digest"`
	Repository            string `json:"repository"`
	Number                int    `json:"number"`
	TitleExcerpt          string `json:"titleExcerpt"`
	TitleBytesOmitted     int    `json:"titleBytesOmitted"`
	BodyExcerpt           string `json:"bodyExcerpt"`
	BodyBytesOmitted      int    `json:"bodyBytesOmitted"`
	ProviderBodyTruncated bool   `json:"providerBodyTruncated"`
	State                 string `json:"state"`
	UpdatedAt             string `json:"updatedAt"`
	ObservedAt            string `json:"observedAt"`
	SourceContextDigest   string `json:"sourceContextDigest"`
}
type OrchestratorFeedbackContext struct {
	ProjectID        string                     `json:"projectId"`
	Items            []OrchestratorFeedbackItem `json:"items"`
	OmittedBodyBytes int                        `json:"omittedBodyBytes"`
	EvidenceOnly     bool                       `json:"evidenceOnly"`
}

func feedbackExcerpt(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	value = value[:limit]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return strings.Clone(value)
}

// Caller already holds s.mu. Atomic private-file reads need no write mutex;
// the empty legacy path adds neither a field nor a new context digest layer.
func (s *Server) orchestratorFeedbackLocked(p project.Project) (*OrchestratorFeedbackContext, error) {
	_, ledger, err := s.readFeedbackLocked(p)
	if err != nil {
		return nil, err
	}
	return feedbackContextForLedger(p.ID, ledger)
}
func feedbackContextForLedger(projectID string, ledger feedbackLedger) (*OrchestratorFeedbackContext, error) {
	if len(ledger.SelectedIDs) == 0 {
		return nil, nil
	}
	selected := map[string]bool{}
	for _, id := range ledger.SelectedIDs {
		selected[id] = true
	}
	value := OrchestratorFeedbackContext{ProjectID: projectID, Items: []OrchestratorFeedbackItem{}, EvidenceOnly: true}
	for _, row := range ledger.Observations {
		if !selected[row.ID] {
			continue
		}
		body := feedbackExcerpt(row.Issue.Body, 256)
		title := feedbackExcerpt(row.Issue.Title, 128)
		omitted := len(row.Issue.Body) - len(body)
		value.Items = append(value.Items, OrchestratorFeedbackItem{ID: row.ID, Digest: row.Digest, Repository: row.Repository, Number: row.Issue.Number, TitleExcerpt: title, TitleBytesOmitted: len(row.Issue.Title) - len(title), BodyExcerpt: body, BodyBytesOmitted: omitted, ProviderBodyTruncated: row.Issue.BodyTruncated, State: row.Issue.State, UpdatedAt: row.Issue.UpdatedAt, ObservedAt: row.ObservedAt, SourceContextDigest: row.SourceContextDigest})
		value.OmittedBodyBytes += omitted
	}
	if !validOrchestratorFeedback(&value, projectID) {
		return nil, ErrProjectFeedback
	}
	return &value, nil
}

func validOrchestratorFeedback(value *OrchestratorFeedbackContext, projectID string) bool {
	if value == nil {
		return true
	}
	if value.ProjectID != projectID || !value.EvidenceOnly || value.Items == nil || len(value.Items) == 0 || len(value.Items) > feedbackSelectionLimit || value.OmittedBodyBytes < 0 {
		return false
	}
	seen := map[string]bool{}
	omitted := 0
	for _, row := range value.Items {
		if row.ID != "feedback-"+row.Digest || !hexDigest(row.Digest) || seen[row.ID] || !validFeedbackRepository(row.Repository) || row.Number <= 0 || row.Number > 1_000_000_000 || row.TitleExcerpt == "" || len(row.TitleExcerpt) > 128 || row.TitleBytesOmitted < 0 || len(row.BodyExcerpt) > 256 || row.BodyBytesOmitted < 0 || !validArtifactStoredText(row.TitleExcerpt) || !validArtifactStoredText(row.BodyExcerpt) || row.State != "open" && row.State != "closed" || !feedbackTimestamp(row.UpdatedAt) || !feedbackTimestamp(row.ObservedAt) || !hexDigest(row.SourceContextDigest) {
			return false
		}
		seen[row.ID] = true
		omitted += row.BodyBytesOmitted
	}
	raw, err := json.Marshal(value)
	return err == nil && len(raw) <= 16<<10 && omitted == value.OmittedBodyBytes
}
