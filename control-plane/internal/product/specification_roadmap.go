package product

import (
	"fmt"
	"strings"
)

// Roadmap phases are planned outcomes, not execution or verified completion.
// The optional field preserves the encoded content of legacy specifications.
type SpecificationRoadmapPhase struct {
	ID                 string   `json:"id"`
	Title              string   `json:"title"`
	Outcome            string   `json:"outcome"`
	RequirementIDs     []string `json:"requirementIds"`
	DependsOn          []string `json:"dependsOn"`
	CompletionCriteria []string `json:"completionCriteria"`
}

func validateSpecificationRoadmap(spec ProjectSpecification, recordIDs map[string]bool, approval bool) error {
	if len(spec.Roadmap) > 100 {
		return fmt.Errorf("Keep the roadmap to at most 100 phases.")
	}
	requirements := map[string]bool{}
	for _, requirement := range spec.Requirements {
		requirements[requirement.ID] = true
	}
	phases := map[string]SpecificationRoadmapPhase{}
	for _, phase := range spec.Roadmap {
		if !designIDPattern.MatchString(phase.ID) || recordIDs[phase.ID] || phases[phase.ID].ID != "" || strings.TrimSpace(phase.Title) == "" || len(phase.Title) > 200 {
			return fmt.Errorf("Roadmap phases need unique stable IDs and a title of at most 200 bytes.")
		}
		if phase.RequirementIDs == nil || phase.DependsOn == nil || phase.CompletionCriteria == nil {
			return fmt.Errorf("Roadmap phase %s needs requirement, dependency and completion-criteria arrays; use empty arrays for an unfinished draft.", phase.ID)
		}
		if len(phase.Outcome) > 12000 || len(phase.RequirementIDs) > 100 || len(phase.DependsOn) > 100 || len(phase.CompletionCriteria) > 30 {
			return fmt.Errorf("Roadmap phase %s exceeds its outcome, link or completion-criteria limits.", phase.ID)
		}
		for _, criterion := range phase.CompletionCriteria {
			if strings.TrimSpace(criterion) == "" || len(criterion) > 2000 {
				return fmt.Errorf("Roadmap phase %s has an empty or oversized completion criterion.", phase.ID)
			}
		}
		if approval && (strings.TrimSpace(phase.Outcome) == "" || len(phase.RequirementIDs) == 0 || len(phase.CompletionCriteria) == 0) {
			return fmt.Errorf("Before approval, phase %s needs its outcome, linked requirements and completion criteria.", phase.ID)
		}
		phases[phase.ID] = phase
	}
	for _, phase := range spec.Roadmap {
		links := map[string]bool{}
		for _, id := range phase.RequirementIDs {
			if !requirements[id] || links[id] {
				return fmt.Errorf("Roadmap phase %s has an unknown or duplicate requirement %s.", phase.ID, id)
			}
			links[id] = true
		}
		dependencies := map[string]bool{}
		for _, id := range phase.DependsOn {
			if phases[id].ID == "" || id == phase.ID || dependencies[id] {
				return fmt.Errorf("Roadmap phase %s has an unknown, duplicate or self dependency %s.", phase.ID, id)
			}
			dependencies[id] = true
		}
	}
	visited := map[string]bool{}
	visiting := map[string]bool{}
	var visit func(string) bool
	visit = func(id string) bool {
		if visiting[id] {
			return false
		}
		if visited[id] {
			return true
		}
		visiting[id] = true
		for _, dependency := range phases[id].DependsOn {
			if !visit(dependency) {
				return false
			}
		}
		visiting[id] = false
		visited[id] = true
		return true
	}
	for _, phase := range spec.Roadmap {
		if !visit(phase.ID) {
			return fmt.Errorf("Roadmap dependencies contain a cycle. Each phase must be reachable without depending on itself.")
		}
	}
	return nil
}

func renderSpecificationRoadmap(phases []SpecificationRoadmapPhase) string {
	if len(phases) == 0 {
		return "" // Keep legacy Markdown exports byte-for-byte unchanged.
	}
	var out strings.Builder
	out.WriteString("\n## Roadmap\n\nPlanned outcomes only. Phase order and completion criteria are not evidence of completed work or authority to execute.\n")
	for _, phase := range phases {
		fmt.Fprintf(&out, "\n### %s: %s\n\nOutcome: %s\nRequirements: %s\nDepends on: %s\n\nCompletion criteria:\n", phase.ID, phase.Title, phase.Outcome, strings.Join(phase.RequirementIDs, ", "), strings.Join(phase.DependsOn, ", "))
		for _, criterion := range phase.CompletionCriteria {
			fmt.Fprintf(&out, "- %s\n", criterion)
		}
	}
	return out.String()
}
