package product

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

const projectTaskConflictLimit = 100

// The inspected plan shares a 7 MiB envelope with its at-most-1 MiB binding.
// Bound the actual indented export, including JSON escaping, before saving or
// importing new task metadata. Former maximum plans fit within this budget.
const projectPlanExportBytesLimit = 5 << 20

var errProjectPlanTooLarge = errors.New("The implementation plan exceeds 5 MiB of encoded context. Reduce task detail or split the work into smaller reviewed plans before saving; captured documents must remain fully readable.")

func validateProjectPlanBytes(plan ProjectPlan) error {
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return err
	}
	if len(data)+1 > projectPlanExportBytesLimit {
		return errProjectPlanTooLarge
	}
	return nil
}

// ProjectTaskGraph is planning advice, never execution admission or evidence.
// In particular, recorded Done statuses cannot establish that a dependency has
// been verified or integrated into a coding checkout. Runtime task dispatch
// must independently establish its prerequisites and execution isolation.
type ProjectTaskGraph struct {
	Waves               [][]string                `json:"waves"`
	ReadyTaskIDs        []string                  `json:"readyTaskIds"`
	Blocked             []ProjectTaskBlocked      `json:"blocked"`
	Conflicts           []ProjectTaskPathConflict `json:"conflicts"`
	ConflictsOmitted    int                       `json:"conflictsOmitted"`
	UnscopedTaskIDs     []string                  `json:"unscopedTaskIds"`
	RecordedDoneTaskIDs []string                  `json:"recordedDoneTaskIds"`
}

type ProjectTaskBlocked struct {
	TaskID    string   `json:"taskId"`
	DependsOn []string `json:"dependsOn"`
}

type ProjectTaskPathConflict struct {
	TaskIDs [2]string `json:"taskIds"`
	// Paths contains the first overlapping pair, not every overlapping path.
	Paths []string `json:"paths"`
}

// Keep derived advice outside canonical plan bytes and approval/run digests.
// Reads do not persist the projection or change earlier context snapshots.
func projectPlanResponse(plan ProjectPlan) any {
	return struct {
		ProjectPlan
		Graph ProjectTaskGraph `json:"graph"`
	}{plan, BuildProjectTaskGraph(plan.Tasks)}
}

func validateProjectTaskGraph(tasks []ProjectTask) error {
	ids := make(map[string]bool, len(tasks))
	roles := make(map[string]string, len(tasks))
	for _, task := range tasks {
		ids[task.ID] = true
		roles[task.ID] = task.Role
	}
	for _, task := range tasks {
		if task.Role != "" && task.Role != "coding" && task.Role != "reviewer" {
			return fmt.Errorf("Task %s has an unsupported role. Choose coding or independent reviewer.", task.ID)
		}
		if task.Role == "reviewer" {
			if task.ReviewOfTaskID == task.ID || !ids[task.ReviewOfTaskID] || roles[task.ReviewOfTaskID] == "reviewer" || len(task.WritePaths) != 0 || len(task.DependsOn) != 0 || strings.TrimSpace(task.Acceptance) == "" {
				return fmt.Errorf("Reviewer task %s needs a different saved coding task, no write paths or dependencies, and explicit review acceptance.", task.ID)
			}
		} else if task.ReviewOfTaskID != "" {
			return fmt.Errorf("Coding task %s cannot claim an independent review assignment.", task.ID)
		}
		for _, other := range tasks {
			for _, dependency := range other.DependsOn {
				if dependency == task.ID && task.Role == "reviewer" {
					return fmt.Errorf("Reviewer task %s cannot be a source dependency for another task.", task.ID)
				}
			}
		}
		if len(task.DependsOn) > 199 {
			return fmt.Errorf("Task %s can depend on at most 199 other tasks.", task.ID)
		}
		seen := map[string]bool{}
		for _, dependency := range task.DependsOn {
			if !designIDPattern.MatchString(dependency) || seen[dependency] {
				return fmt.Errorf("Task %s needs unique stable dependency IDs.", task.ID)
			}
			if dependency == task.ID {
				return fmt.Errorf("Task %s cannot depend on itself.", task.ID)
			}
			if !ids[dependency] {
				return fmt.Errorf("Task %s depends on unavailable task %s. Restore that task or remove the dependency before saving.", task.ID, dependency)
			}
			seen[dependency] = true
		}
		if len(task.Acceptance) > 4000 || !utf8.ValidString(task.Acceptance) {
			return fmt.Errorf("Task %s acceptance must use valid text of at most 4,000 bytes.", task.ID)
		}
		if len(task.WritePaths) > 32 {
			return fmt.Errorf("Task %s can name at most 32 intended write paths.", task.ID)
		}
		for i, value := range task.WritePaths {
			if !validProjectTaskWritePath(value) {
				return fmt.Errorf("Task %s needs canonical repository-relative write paths of at most 512 bytes. Use file or directory names without globs, trailing slashes, traversal, reserved Windows names, or .git/.luci control paths.", task.ID)
			}
			for _, earlier := range task.WritePaths[:i] {
				if strings.EqualFold(earlier, value) {
					return fmt.Errorf("Task %s has duplicate write paths, including a case-only duplicate. Keep each path once.", task.ID)
				}
			}
		}
	}
	if _, ok := projectTaskWaves(tasks); !ok {
		return errors.New("Task dependencies form a cycle. Remove a dependency so every task can follow a clear order.")
	}
	return nil
}

func validProjectTaskWritePath(value string) bool {
	if len(value) > 512 || !utf8.ValidString(value) || strings.TrimSpace(value) != value || !backupSafePath(value) {
		return false
	}
	for _, part := range strings.Split(value, "/") {
		if strings.EqualFold(part, ".git") || strings.EqualFold(part, ".luci") {
			return false
		}
		// A portable path must not depend on Windows silently aliasing device
		// names or case. Device digits include the three legacy superscripts.
		base := strings.ToUpper(strings.TrimSpace(strings.SplitN(part, ".", 2)[0]))
		device := []rune(base)
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || (len(device) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && strings.ContainsRune("0123456789¹²³", device[3])) {
			return false
		}
		units := 0
		for _, c := range part {
			if c == 127 {
				return false
			}
			units++
			if c > 0xffff {
				units++
			}
		}
		if units > 255 {
			return false
		}
	}
	return true
}

// Wave ordering follows saved task order and describes dependencies only. File
// conflicts and unscoped work are exposed separately; membership in one wave
// never guarantees that those tasks can execute safely in parallel.
func projectTaskWaves(tasks []ProjectTask) ([][]string, bool) {
	waves := [][]string{}
	visited := map[string]bool{}
	for len(visited) < len(tasks) {
		wave := []string{}
		for _, task := range tasks {
			if visited[task.ID] {
				continue
			}
			ready := true
			for _, dependency := range task.DependsOn {
				if !visited[dependency] {
					ready = false
					break
				}
			}
			if ready {
				wave = append(wave, task.ID)
			}
		}
		if len(wave) == 0 {
			return waves, false
		}
		for _, id := range wave {
			visited[id] = true
		}
		waves = append(waves, wave)
	}
	return waves, true
}

// BuildProjectTaskGraph projects a validated saved plan without observing a
// runtime, launching work, changing status, or treating agent claims as proof.
func BuildProjectTaskGraph(tasks []ProjectTask) ProjectTaskGraph {
	waves, _ := projectTaskWaves(tasks)
	graph := ProjectTaskGraph{Waves: waves, ReadyTaskIDs: []string{}, Blocked: []ProjectTaskBlocked{}, Conflicts: []ProjectTaskPathConflict{}, UnscopedTaskIDs: []string{}, RecordedDoneTaskIDs: []string{}}
	byID := make(map[string]ProjectTask, len(tasks))
	for _, task := range tasks {
		byID[task.ID] = task
	}
	for _, task := range tasks {
		if len(task.WritePaths) == 0 {
			graph.UnscopedTaskIDs = append(graph.UnscopedTaskIDs, task.ID)
		}
		if task.Status == "done" {
			graph.RecordedDoneTaskIDs = append(graph.RecordedDoneTaskIDs, task.ID)
			continue
		}
		waiting := []string{}
		for _, dependency := range task.DependsOn {
			if byID[dependency].Status != "done" {
				waiting = append(waiting, dependency)
			}
		}
		if len(waiting) > 0 {
			graph.Blocked = append(graph.Blocked, ProjectTaskBlocked{TaskID: task.ID, DependsOn: waiting})
		} else if task.Status == "todo" {
			graph.ReadyTaskIDs = append(graph.ReadyTaskIDs, task.ID)
		}
	}
	ancestors := make(map[string]map[string]bool, len(tasks))
	for _, wave := range waves {
		for _, id := range wave {
			ancestors[id] = map[string]bool{}
			for _, dependency := range byID[id].DependsOn {
				ancestors[id][dependency] = true
				for ancestor := range ancestors[dependency] {
					ancestors[id][ancestor] = true
				}
			}
		}
	}
	for i, a := range tasks {
		for _, b := range tasks[i+1:] {
			if ancestors[a.ID][b.ID] || ancestors[b.ID][a.ID] {
				continue
			}
			paths := projectTaskOverlappingPaths(a.WritePaths, b.WritePaths)
			if len(paths) == 0 {
				continue
			}
			if len(graph.Conflicts) == projectTaskConflictLimit {
				graph.ConflictsOmitted++
				continue
			}
			graph.Conflicts = append(graph.Conflicts, ProjectTaskPathConflict{TaskIDs: [2]string{a.ID, b.ID}, Paths: paths})
		}
	}
	return graph
}

func projectTaskOverlappingPaths(a, b []string) []string {
	for _, x := range a {
		for _, y := range b {
			left, right := strings.Split(x, "/"), strings.Split(y, "/")
			overlaps := true
			for i := 0; i < len(left) && i < len(right); i++ {
				if !strings.EqualFold(left[i], right[i]) {
					overlaps = false
					break
				}
			}
			if overlaps {
				if x == y {
					return []string{x}
				}
				return []string{x, y}
			}
		}
	}
	return nil
}
