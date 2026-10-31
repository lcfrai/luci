package product

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Strategy belongs to the versioned specification, not a second source of
// project intent. Evidence is attributed input; Luci does not verify a claim
// merely because someone records it here.
type SpecificationStrategy struct {
	Customer         string                          `json:"customer"`
	Problem          string                          `json:"problem"`
	ValueProposition string                          `json:"valueProposition"`
	Alternatives     []SpecificationAlternative      `json:"alternatives"`
	Assumptions      []SpecificationAssumption       `json:"assumptions"`
	Evidence         []SpecificationStrategyEvidence `json:"evidence"`
	Metrics          []SpecificationSuccessMetric    `json:"metrics"`
}

type SpecificationAlternative struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Difference  string   `json:"difference"`
	EvidenceIDs []string `json:"evidenceIds"`
}

type SpecificationAssumption struct {
	ID             string   `json:"id"`
	Claim          string   `json:"claim"`
	Status         string   `json:"status" enum:"untested,supported,contradicted"`
	Experiment     string   `json:"experiment"`
	EvidenceIDs    []string `json:"evidenceIds"`
	RequirementIDs []string `json:"requirementIds"`
}

type SpecificationStrategyEvidence struct {
	ID          string `json:"id"`
	Source      string `json:"source"`
	URL         string `json:"url"`
	Observation string `json:"observation"`
	ObservedAt  string `json:"observedAt"`
}

type SpecificationSuccessMetric struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Baseline       string   `json:"baseline"`
	Target         string   `json:"target"`
	Window         string   `json:"window"`
	Measurement    string   `json:"measurement"`
	RequirementIDs []string `json:"requirementIds"`
}

func validateSpecificationStrategy(spec ProjectSpecification) error {
	s := spec.Strategy
	if s == nil {
		return nil
	}
	if len(s.Customer) > 12000 || len(s.Problem) > 12000 || len(s.ValueProposition) > 12000 {
		return fmt.Errorf("Keep each strategy summary to at most 12000 bytes.")
	}
	if s.Alternatives == nil || s.Assumptions == nil || s.Evidence == nil || s.Metrics == nil || len(s.Alternatives) > 40 || len(s.Assumptions) > 60 || len(s.Evidence) > 80 || len(s.Metrics) > 40 {
		return fmt.Errorf("Strategy needs explicit arrays: at most 40 alternatives, 60 assumptions, 80 sources and 40 success measures.")
	}
	ids, evidence, requirements := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, r := range spec.Requirements {
		requirements[r.ID] = true
	}
	id := func(value string) bool {
		if !designIDPattern.MatchString(value) || ids[value] {
			return false
		}
		ids[value] = true
		return true
	}
	text := func(value string, max int) bool { return strings.TrimSpace(value) != "" && len(value) <= max }
	links := func(values []string, allowed map[string]bool) bool {
		if values == nil || len(values) > 100 {
			return false
		}
		seen := map[string]bool{}
		for _, value := range values {
			if !allowed[value] || seen[value] {
				return false
			}
			seen[value] = true
		}
		return true
	}
	for _, e := range s.Evidence {
		if !id(e.ID) || !text(e.Source, 500) || !text(e.Observation, 12000) || len(e.URL) > 2048 || len(e.ObservedAt) > 40 {
			return fmt.Errorf("Each strategy source needs a unique ID, a source name and an observation within the text limits.")
		}
		if e.URL != "" {
			u, err := url.Parse(e.URL)
			if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || u.User != nil || strings.ContainsAny(e.URL, "\r\n\t ") {
				return fmt.Errorf("Source %s needs an HTTP(S) URL without embedded credentials, or no URL for an interview or offline source.", e.ID)
			}
		}
		if e.ObservedAt != "" {
			if _, err := time.Parse(time.RFC3339, e.ObservedAt); err != nil {
				return fmt.Errorf("Source %s needs an RFC3339 observation date or an empty date when unknown.", e.ID)
			}
		}
		evidence[e.ID] = true
	}
	for _, a := range s.Alternatives {
		if !id(a.ID) || !text(a.Name, 500) || len(a.Difference) > 12000 || !links(a.EvidenceIDs, evidence) {
			return fmt.Errorf("Alternative %s needs a unique ID, name and valid, nonduplicate source links.", a.ID)
		}
	}
	for _, a := range s.Assumptions {
		if !id(a.ID) || !text(a.Claim, 12000) || len(a.Experiment) > 12000 || !links(a.EvidenceIDs, evidence) || !links(a.RequirementIDs, requirements) {
			return fmt.Errorf("Assumption %s needs a unique ID, claim and valid source and requirement links.", a.ID)
		}
		if a.Status != "untested" && a.Status != "supported" && a.Status != "contradicted" {
			return fmt.Errorf("Assumption %s has an unknown status.", a.ID)
		}
		if a.Status != "untested" && len(a.EvidenceIDs) == 0 {
			return fmt.Errorf("Link a recorded observation before marking assumption %s supported or contradicted.", a.ID)
		}
	}
	for _, m := range s.Metrics {
		if !id(m.ID) || !text(m.Name, 500) || len(m.Baseline) > 2000 || len(m.Target) > 2000 || len(m.Window) > 2000 || len(m.Measurement) > 12000 || !links(m.RequirementIDs, requirements) {
			return fmt.Errorf("Success measure %s needs a unique ID, name and valid requirement links within the text limits.", m.ID)
		}
	}
	return nil
}

// Legacy or partial assisted proposals cannot silently erase strategy/roadmap.
// An explicit empty strategy object (with arrays) or empty roadmap can clear it.
func preserveSpecificationPlanning(next *ProjectSpecification, current ProjectSpecification) {
	if next.Strategy == nil {
		next.Strategy = current.Strategy
	}
	if next.Roadmap == nil {
		next.Roadmap = current.Roadmap
	}
}

func renderSpecificationStrategy(s *SpecificationStrategy) string {
	if s == nil {
		return ""
	}
	var out strings.Builder
	fmt.Fprintf(&out, "\n## Product strategy\n\nCustomer: %s\nProblem and current workaround: %s\nValue proposition: %s\n\nThese are attributed inputs and hypotheses, not automatically verified facts. Unknown baselines remain unknown.\n", s.Customer, s.Problem, s.ValueProposition)
	for _, a := range s.Alternatives {
		fmt.Fprintf(&out, "\n### Alternative %s: %s\n%s\nSources: %s\n", a.ID, a.Name, a.Difference, strings.Join(a.EvidenceIDs, ", "))
	}
	for _, a := range s.Assumptions {
		fmt.Fprintf(&out, "\n### Assumption %s (%s)\n%s\nExperiment: %s\nSources: %s\nAffected requirements: %s\n", a.ID, a.Status, a.Claim, a.Experiment, strings.Join(a.EvidenceIDs, ", "), strings.Join(a.RequirementIDs, ", "))
	}
	for _, e := range s.Evidence {
		fmt.Fprintf(&out, "\n### Source %s: %s\nURL: %s\nObserved at: %s\nObservation: %s\n", e.ID, e.Source, e.URL, e.ObservedAt, e.Observation)
	}
	for _, m := range s.Metrics {
		fmt.Fprintf(&out, "\n### Success measure %s: %s\nBaseline: %s\nTarget: %s\nTime window: %s\nMeasurement method: %s\nRequirements: %s\n", m.ID, m.Name, m.Baseline, m.Target, m.Window, m.Measurement, strings.Join(m.RequirementIDs, ", "))
	}
	return out.String()
}
