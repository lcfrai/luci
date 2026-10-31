package product

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path"
	"reflect"
	"strings"
)

// Nonsecret, immutable generated inputs for an explicitly approved task. These
// are planning data, not proof of implementation, check success or permission.
// The preparation record remains the authority for subsequent image builds.
type TaskRuntimeArtifact struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	SHA256  string `json:"sha256"`
}
type TaskRuntimePrerequisite struct {
	PreparationID     string                `json:"preparationId"`
	PreparationDigest string                `json:"preparationDigest"`
	ServiceID         string                `json:"serviceId"`
	Kind              string                `json:"kind"`
	Artifacts         []TaskRuntimeArtifact `json:"artifacts"`
	Dependencies      map[string]string     `json:"dependencies,omitempty"`
}

func runtimeArtifactPath(v string) bool {
	if v == "" || len(v) > 240 || !validProjectTaskWritePath(v) || path.Clean(v) != v || path.IsAbs(v) || strings.ContainsAny(v, "\\:\x00\r\n") {
		return false
	}
	for _, part := range strings.Split(v, "/") {
		if part == ".." || part == ".git" || strings.HasPrefix(part, ".env") || part == ".luci" {
			return false
		}
	}
	return true
}
func validateTaskRuntimePrerequisites(rows *[]TaskRuntimePrerequisite) error {
	if rows == nil {
		return nil
	}
	if len(*rows) > 2 {
		return errors.New("Use at most two runtime prerequisites per task")
	}
	seen := map[string]bool{}
	total := 0
	for _, r := range *rows {
		key := r.PreparationID + ":" + r.ServiceID + ":" + r.Kind
		if !runtimePreparationID(r.PreparationID) || !hexDigest(r.PreparationDigest) || !designIDPattern.MatchString(r.ServiceID) || seen[key] || (r.Kind != "instrumentation" && r.Kind != "inference-client") || len(r.Artifacts) < 1 || len(r.Artifacts) > 4 || len(r.Dependencies) > 16 {
			return errors.New("Invalid exact runtime prerequisite reference")
		}
		seen[key] = true
		paths := map[string]bool{}
		for _, a := range r.Artifacts {
			h := sha256.Sum256([]byte(a.Content))
			total += len(a.Content)
			if !runtimeArtifactPath(a.Path) || paths[a.Path] || a.Content == "" || len(a.Content) > 16<<10 || !validArtifactStoredText(a.Content) || hex.EncodeToString(h[:]) != a.SHA256 || total > 64<<10 {
				return errors.New("Generated runtime task files must be bounded exact nonsecret content with unique relative paths and matching hashes")
			}
			paths[a.Path] = true
		}
		for name, version := range r.Dependencies {
			if name == "" || len(name) > 120 || strings.ContainsAny(name, "\x00\r\n\\:") || len(version) < 5 || len(version) > 80 || strings.ContainsAny(version, " \t\r\n^~*<>=/\\:") {
				return errors.New("Runtime dependencies need explicit package names and exact declared versions")
			}
		}
	}
	return nil
}
func snapshotTaskRuntimePrerequisites(rows *[]TaskRuntimePrerequisite) *[]TaskRuntimePrerequisite {
	if rows == nil {
		return nil
	}
	out := append([]TaskRuntimePrerequisite{}, (*rows)...)
	for i := range out {
		out[i].Artifacts = append([]TaskRuntimeArtifact{}, out[i].Artifacts...)
		if out[i].Dependencies != nil {
			m := map[string]string{}
			for k, v := range out[i].Dependencies {
				m[k] = v
			}
			out[i].Dependencies = m
		}
	}
	return &out
}
func preserveTaskRuntimePrerequisites(old, next []ProjectTask) []ProjectTask {
	out := snapshotProjectTasks(next)
	byID := map[string]*[]TaskRuntimePrerequisite{}
	for _, t := range old {
		byID[t.ID] = t.RuntimePrerequisites
	}
	for i := range out {
		if out[i].RuntimePrerequisites == nil {
			out[i].RuntimePrerequisites = snapshotTaskRuntimePrerequisites(byID[out[i].ID])
		}
	}
	return out
}

// A model cannot invent or move a preparation assignment to another task.
// Explicit [] can clear; independent direct human/server Plan saves are separate.
func validateAssistedRuntimePrerequisites(old, next []ProjectTask) error {
	byID := map[string]*[]TaskRuntimePrerequisite{}
	for _, t := range old {
		byID[t.ID] = t.RuntimePrerequisites
	}
	for _, t := range next {
		if t.RuntimePrerequisites == nil || len(*t.RuntimePrerequisites) == 0 {
			continue
		}
		prior := byID[t.ID]
		if prior == nil {
			return errors.New("Save the reviewed runtime prerequisite assignment before asking Luci to use it")
		}
		for _, p := range *t.RuntimePrerequisites {
			found := false
			for _, q := range *prior {
				if reflect.DeepEqual(p, q) {
					found = true
					break
				}
			}
			if !found {
				return errors.New("Runtime prerequisite identity/content cannot be invented or reassigned by a plan proposal")
			}
		}
	}
	return nil
}
