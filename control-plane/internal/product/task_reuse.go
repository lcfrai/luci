package product

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"net/url"
	"reflect"
	"strings"
	"time"
)

// A retained discovery reference and an authoring decision, never permission to
// download/install code or proof of licensing, compatibility or source quality.
type TaskReuseCitation struct {
	URL        string `json:"url"`
	Digest     string `json:"digest"`
	ObservedAt string `json:"observedAt"`
}
type TaskReuseSource struct {
	ID                   string              `json:"id"`
	Name                 string              `json:"name"`
	URL                  string              `json:"url"`
	Description          string              `json:"description"`
	Framework            string              `json:"framework"`
	Licence              string              `json:"licence"`
	LicenceStatus        string              `json:"licenceStatus"`
	LicenceScope         string              `json:"licenceScope"`
	LicenceBasis         string              `json:"licenceBasis"`
	ObservedAt           string              `json:"observedAt"`
	CatalogueHash        string              `json:"catalogueHash"`
	CatalogueGeneratedAt string              `json:"catalogueGeneratedAt"`
	ResponseDigest       string              `json:"responseDigest"`
	CapturedAt           string              `json:"capturedAt"`
	Citations            []TaskReuseCitation `json:"citations"`
	CitationCount        int                 `json:"citationCount"`
	Digest               string              `json:"digest"`
}
type TaskReuseDecision struct {
	Source              TaskReuseSource `json:"source"`
	Decision            string          `json:"decision"`
	Rationale           string          `json:"rationale"`
	LicenceReview       string          `json:"licenceReview"`
	CompatibilityReview string          `json:"compatibilityReview"`
}

func TaskReuseSourceDigest(source TaskReuseSource) string {
	source.Digest = ""
	return reviewDigest(source)
}
func TaskReusePublicURL(value string) bool {
	u, err := url.Parse(value)
	if err != nil || len(value) > 4000 || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Port() != "" || u.RawQuery != "" || u.Fragment != "" || u.String() != value {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if net.ParseIP(host) != nil || !strings.Contains(host, ".") || strings.HasSuffix(host, ".") {
		return false
	}
	for _, tail := range []string{"localhost", "local", "internal", "test", "invalid"} {
		if host == tail || strings.HasSuffix(host, "."+tail) {
			return false
		}
	}
	return true
}
func reuseText(value string, n int) bool { return len(value) <= n && validArtifactStoredText(value) }
func reuseTime(value string) bool        { _, err := time.Parse(time.RFC3339Nano, value); return err == nil }
func ValidateTaskReuseSource(s TaskReuseSource) error {
	h := sha256.Sum256([]byte(s.URL))
	id := hex.EncodeToString(h[:])[:20]
	if s.ID != id || !TaskReusePublicURL(s.URL) || strings.TrimSpace(s.Name) == "" || !reuseText(s.Name, 200) || !reuseText(s.Description, 1200) || !reuseText(s.Framework, 100) || !reuseText(s.Licence, 100) || !reuseTime(s.ObservedAt) || !reuseTime(s.CatalogueGeneratedAt) || !reuseTime(s.CapturedAt) || !hexDigest(s.CatalogueHash) || !hexDigest(s.ResponseDigest) || s.Digest != TaskReuseSourceDigest(s) || len(s.Citations) < 1 || len(s.Citations) > 3 || s.CitationCount < len(s.Citations) || s.CitationCount > 21 {
		return errors.New("Invalid saved Glasses source snapshot")
	}
	if s.LicenceStatus != "known" && s.LicenceStatus != "restricted" && s.LicenceStatus != "unknown" || (s.Licence == "") != (s.LicenceStatus == "unknown") {
		return errors.New("Licence metadata is incomplete")
	}
	if s.LicenceScope != "repository" && s.LicenceScope != "registry-provider" && s.LicenceScope != "registry-item" && s.LicenceScope != "source-artifact" && s.LicenceScope != "public-page" {
		return errors.New("Unknown licence source scope")
	}
	if s.LicenceBasis != "upstream-metadata" && s.LicenceBasis != "source-declaration" {
		return errors.New("Unknown licence evidence basis")
	}
	seen := map[string]bool{}
	for _, c := range s.Citations {
		key := c.URL + ":" + c.Digest
		if !TaskReusePublicURL(c.URL) || !hexDigest(c.Digest) || !reuseTime(c.ObservedAt) || seen[key] {
			return errors.New("Invalid saved source citation")
		}
		seen[key] = true
	}
	return nil
}
func validateTaskReuse(rows *[]TaskReuseDecision) error {
	if rows == nil {
		return nil
	}
	if *rows == nil || len(*rows) > 4 {
		return errors.New("Use at most four reuse decisions per task")
	}
	seen := map[string]bool{}
	for _, r := range *rows {
		if err := ValidateTaskReuseSource(r.Source); err != nil {
			return err
		}
		if seen[r.Source.ID] {
			return errors.New("Choose one version of each candidate per task")
		}
		seen[r.Source.ID] = true
		if r.Decision != "investigate" && r.Decision != "use" && r.Decision != "reject" || !reuseText(r.Rationale, 2000) || !reuseText(r.LicenceReview, 2000) || !reuseText(r.CompatibilityReview, 2000) {
			return errors.New("Invalid reuse decision")
		}
		if r.Decision == "use" && (strings.TrimSpace(r.Rationale) == "" || strings.TrimSpace(r.LicenceReview) == "" || strings.TrimSpace(r.CompatibilityReview) == "") {
			return errors.New("Record why this source fits and review its licence and compatibility before planning reuse")
		}
	}
	return nil
}
func snapshotTaskReuse(rows *[]TaskReuseDecision) *[]TaskReuseDecision {
	if rows == nil {
		return nil
	}
	copy := append([]TaskReuseDecision{}, (*rows)...)
	for i := range copy {
		copy[i].Source.Citations = append([]TaskReuseCitation{}, copy[i].Source.Citations...)
	}
	return &copy
}

// Older clients/model proposals omit the optional field. Preserve same-task
// decisions; an explicit [] removes them. Deleted tasks remain deliberate edits.
func preserveTaskReuse(old []ProjectTask, next []ProjectTask) []ProjectTask {
	result := snapshotProjectTasks(next)
	byID := map[string]*[]TaskReuseDecision{}
	for _, t := range old {
		byID[t.ID] = t.Reuse
	}
	for i := range result {
		if result[i].Reuse == nil {
			result[i].Reuse = snapshotTaskReuse(byID[result[i].ID])
		}
	}
	return result
}

// Assistant proposals may reorganize or review captured choices, but cannot mint
// source evidence or resurrect a choice removed before this turn was captured.
// Human Plan saves retain the separate explicit authoring path.
func validateAssistedTaskReuse(captured, next []ProjectTask) error {
	sources := map[string][]TaskReuseSource{}
	for _, task := range captured {
		if task.Reuse != nil {
			for _, r := range *task.Reuse {
				sources[r.Source.Digest] = append(sources[r.Source.Digest], r.Source)
			}
		}
	}
	for _, task := range next {
		if task.Reuse == nil {
			continue
		}
		for _, r := range *task.Reuse {
			found := false
			for _, source := range sources[r.Source.Digest] {
				if reflect.DeepEqual(source, r.Source) {
					found = true
					break
				}
			}
			if !found {
				return errors.New("Reuse sources must come from this turn's captured project plan; inspect and save new candidates before asking Luci to use them")
			}
		}
	}
	return nil
}
