package product

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func graphTask(id string, dependencies ...string) ProjectTask {
	return ProjectTask{ID: id, Title: "Implement " + id, Status: "todo", DependsOn: dependencies}
}

func TestProjectTaskGraphRejectsUnusableDependenciesAndWritePaths(t *testing.T) {
	cases := []struct {
		name string
		edit func([]ProjectTask) []ProjectTask
		want string
	}{
		{"missing", func(tasks []ProjectTask) []ProjectTask { tasks[1].DependsOn = []string{"missing"}; return tasks }, "unavailable task missing"},
		{"self", func(tasks []ProjectTask) []ProjectTask { tasks[0].DependsOn = []string{"a"}; return tasks }, "cannot depend on itself"},
		{"duplicate", func(tasks []ProjectTask) []ProjectTask { tasks[1].DependsOn = []string{"a", "a"}; return tasks }, "unique stable dependency IDs"},
		{"invalid id", func(tasks []ProjectTask) []ProjectTask { tasks[1].DependsOn = []string{"../a"}; return tasks }, "unique stable dependency IDs"},
		{"cycle", func(tasks []ProjectTask) []ProjectTask { tasks[0].DependsOn = []string{"c"}; return tasks }, "form a cycle"},
		{"too many dependencies", func(tasks []ProjectTask) []ProjectTask { tasks[1].DependsOn = make([]string, 200); return tasks }, "at most 199"},
		{"too many paths", func(tasks []ProjectTask) []ProjectTask { tasks[0].WritePaths = make([]string, 33); return tasks }, "at most 32"},
		{"duplicate path", func(tasks []ProjectTask) []ProjectTask {
			tasks[0].WritePaths = []string{"src/App.tsx", "SRC/app.tsx"}
			return tasks
		}, "duplicate write paths"},
		{"oversized acceptance", func(tasks []ProjectTask) []ProjectTask { tasks[0].Acceptance = strings.Repeat("a", 4001); return tasks }, "4,000 bytes"},
		{"invalid acceptance text", func(tasks []ProjectTask) []ProjectTask { tasks[0].Acceptance = string([]byte{0xff}); return tasks }, "valid text"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tasks := tc.edit([]ProjectTask{graphTask("a"), graphTask("b", "a"), graphTask("c", "b")})
			err := validatePlan(ProjectPlan{Tasks: tasks})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("wanted %q, got %v", tc.want, err)
			}
		})
	}
	invalidPaths := []string{"", ".", "..", "../outside", "src/../outside", "/absolute", "C:/project", "src\\file", "src//file", "src/", "src/*", "src/?", "src/[a]*", "src/file\n", "src/file\x00", "src/file.", "src/file ", " src/file", ".git/config", "vendor/.Git/config", ".luci/plan.json", "vendor/.LUCI/plan.json", "CON", "src/NUL.txt", "src/COM1.js", "src/LPT9", "src/COM¹.txt", "src/LPT².log", "src/CON .txt", "src/file\x7f", "src/file:stream", strings.Repeat("x", 256), strings.Repeat("x", 513), string([]byte{0xff})}
	for _, value := range invalidPaths {
		t.Run("path_"+fmt.Sprintf("%q", value), func(t *testing.T) {
			task := graphTask("a")
			task.WritePaths = []string{value}
			if err := validatePlan(ProjectPlan{Tasks: []ProjectTask{task}}); err == nil {
				t.Fatalf("unsafe or noncanonical path accepted: %q", value)
			}
		})
	}
	for _, value := range []string{"src", "src/App.tsx", "tests/unit", "README.md", ".github/workflows/check.yml", "assets/écran.svg", "src/some file.ts", "COM10", strings.Repeat("x", 255) + "/" + strings.Repeat("y", 255)} {
		task := graphTask("a")
		task.WritePaths = []string{value}
		if err := validatePlan(ProjectPlan{Tasks: []ProjectTask{task}}); err != nil {
			t.Fatalf("valid literal path %q rejected: %v", value, err)
		}
	}
}

func TestProjectTaskGraphSeparatesStructuralWavesRecordedProgressAndFileConflicts(t *testing.T) {
	// Saved order is deliberately not topological. The independent documentation
	// task can run with either the schema or API work, though it appears in wave 1.
	tasks := []ProjectTask{graphTask("verify", "api", "ui"), graphTask("schema"), graphTask("api", "schema"), graphTask("ui", "schema"), graphTask("docs"), graphTask("unscoped")}
	tasks[0].WritePaths = []string{"tests"}
	tasks[1].WritePaths = []string{"schema"}
	tasks[2].WritePaths = []string{"src/api"}
	tasks[3].WritePaths = []string{"src/ui"}
	tasks[4].WritePaths = []string{"SRC/api/readme.md"}
	if err := validatePlan(ProjectPlan{Tasks: tasks}); err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(tasks)
	graph := BuildProjectTaskGraph(tasks)
	wantWaves := [][]string{{"schema", "docs", "unscoped"}, {"api", "ui"}, {"verify"}}
	if !reflect.DeepEqual(graph.Waves, wantWaves) || !reflect.DeepEqual(graph.ReadyTaskIDs, []string{"schema", "docs", "unscoped"}) || !reflect.DeepEqual(graph.UnscopedTaskIDs, []string{"unscoped"}) {
		t.Fatalf("unexpected graph: %#v", graph)
	}
	wantBlocked := []ProjectTaskBlocked{{"verify", []string{"api", "ui"}}, {"api", []string{"schema"}}, {"ui", []string{"schema"}}}
	if !reflect.DeepEqual(graph.Blocked, wantBlocked) || !reflect.DeepEqual(graph.Conflicts, []ProjectTaskPathConflict{{[2]string{"api", "docs"}, []string{"src/api", "SRC/api/readme.md"}}}) {
		t.Fatalf("dependency or path conflicts lost: %#v", graph)
	}
	after, _ := json.Marshal(tasks)
	if string(before) != string(after) {
		t.Fatal("read-only graph changed task inputs")
	}
	tasks[1].Status = "done"
	tasks[2].Status = "doing"
	graph = BuildProjectTaskGraph(tasks)
	if !reflect.DeepEqual(graph.Waves, wantWaves) || !reflect.DeepEqual(graph.ReadyTaskIDs, []string{"ui", "docs", "unscoped"}) || !reflect.DeepEqual(graph.RecordedDoneTaskIDs, []string{"schema"}) {
		t.Fatalf("recorded progress changed structural ordering or offered active work: %#v", graph)
	}
	if !reflect.DeepEqual(graph.Blocked, []ProjectTaskBlocked{{"verify", []string{"api", "ui"}}}) {
		t.Fatalf("active coding was treated as dependency completion: %#v", graph.Blocked)
	}
	data, err := json.Marshal(graph)
	if err != nil || strings.Contains(string(data), "verified") || strings.Contains(string(data), "succeeded") {
		t.Fatalf("planning projection manufactured execution evidence: %s (%v)", data, err)
	}
}

func TestProjectTaskGraphSerialOwnershipAndWindowsPathBoundaries(t *testing.T) {
	tasks := []ProjectTask{graphTask("a"), graphTask("b", "a"), graphTask("c", "b"), graphTask("d"), graphTask("e"), graphTask("f")}
	for i, value := range []string{"src/api", "src", "src/api/handler.go", "src/api2", "SRC/API", "src/api/handler.go"} {
		tasks[i].WritePaths = []string{value}
	}
	graph := BuildProjectTaskGraph(tasks)
	pairs := map[[2]string]bool{}
	for _, conflict := range graph.Conflicts {
		pairs[conflict.TaskIDs] = true
	}
	for _, pair := range [][2]string{{"a", "b"}, {"a", "c"}, {"b", "c"}, {"a", "d"}, {"c", "d"}, {"d", "e"}, {"d", "f"}} {
		if pairs[pair] {
			t.Fatalf("serial dependency or distinct path reported conflicting: %v", pair)
		}
	}
	for _, pair := range [][2]string{{"a", "e"}, {"a", "f"}, {"b", "d"}, {"b", "e"}, {"b", "f"}, {"c", "e"}, {"c", "f"}, {"e", "f"}} {
		if !pairs[pair] {
			t.Fatalf("case-insensitive/ancestor overlap was missed: %v", pair)
		}
	}
}

func TestProjectTaskGraphBoundsConflictsWithoutHidingOmissions(t *testing.T) {
	tasks := make([]ProjectTask, 200)
	for i := range tasks {
		tasks[i] = graphTask(fmt.Sprintf("task-%03d", i))
		tasks[i].WritePaths = []string{"src"}
	}
	if err := validatePlan(ProjectPlan{Tasks: tasks}); err != nil {
		t.Fatal(err)
	}
	graph := BuildProjectTaskGraph(tasks)
	if len(graph.Conflicts) != 100 || graph.ConflictsOmitted != 19800 || len(graph.Waves) != 1 || len(graph.Waves[0]) != 200 || len(graph.ReadyTaskIDs) != 200 {
		t.Fatalf("projection lost scope or omitted conflicts silently: %#v", graph)
	}
	data, err := json.Marshal(graph)
	if err != nil || len(data) > 32<<10 {
		t.Fatalf("unexpected graph size %d: %v", len(data), err)
	}
}

func TestProjectTaskGraphCopiesCoordinationSlices(t *testing.T) {
	input := []ProjectTask{{ID: "b", Title: "Build", Status: "todo", RequirementIDs: []string{"r1"}, DependsOn: []string{"a"}, WritePaths: []string{"src"}, Acceptance: "The saved result reloads"}}
	copy := snapshotProjectTasks(input)
	input[0].RequirementIDs[0], input[0].DependsOn[0], input[0].WritePaths[0] = "r2", "c", "elsewhere"
	if copy[0].RequirementIDs[0] != "r1" || copy[0].DependsOn[0] != "a" || copy[0].WritePaths[0] != "src" {
		t.Fatalf("caller mutation rewrote captured task: %#v", copy)
	}
}

func TestProjectTaskGraphSaveExportReloadAndRejectionPreserveCanonicalContext(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerProjectWork(mux)
	p := createJourneyProject(t, mux, "coordinated-plan")
	base := "/v1/product/projects/" + p.ID + "/plan"
	tasks := []ProjectTask{graphTask("schema"), graphTask("api", "schema"), graphTask("ui", "schema")}
	tasks[0].WritePaths, tasks[1].WritePaths, tasks[2].WritePaths = []string{"db"}, []string{"src/api"}, []string{"src/ui"}
	tasks[1].Acceptance = "An authorized person can save and reload an item."
	response := journeyRequest(t, mux, "POST", base, map[string]any{"expectedRevision": 0, "tasks": tasks}, 200)
	var saved struct {
		ProjectPlan
		Graph ProjectTaskGraph `json:"graph"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &saved); err != nil || saved.Revision != 1 || !reflect.DeepEqual(saved.Tasks, tasks) || len(saved.Graph.Waves) != 2 {
		t.Fatalf("save dropped coordination contract: %s (%v)", response.Body, err)
	}
	export := filepath.Join(p.Path, ".luci", "plan.json")
	before, err := os.ReadFile(export)
	if err != nil {
		t.Fatal(err)
	}
	var canonical ProjectPlan
	if err := json.Unmarshal(before, &canonical); err != nil || !reflect.DeepEqual(canonical.Tasks, tasks) || strings.Contains(string(before), `"graph"`) {
		t.Fatalf("canonical captured plan lost tasks or persisted derived graph: %s (%v)", before, err)
	}
	changed := snapshotProjectTasks(tasks)
	changed[0].DependsOn = []string{"api"}
	journeyRequest(t, mux, "POST", base, map[string]any{"expectedRevision": 1, "tasks": changed}, 400)
	// A deletion must be deliberate about dependencies, not silently erase them.
	journeyRequest(t, mux, "POST", base, map[string]any{"expectedRevision": 1, "tasks": tasks[1:]}, 400)
	journeyRequest(t, mux, "POST", base, map[string]any{"expectedRevision": 0, "tasks": tasks}, 409)
	after, _ := os.ReadFile(export)
	if string(before) != string(after) {
		t.Fatal("rejected amendment changed exported context")
	}
	reloaded, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	defer reloaded.Close()
	other := http.NewServeMux()
	reloaded.registerProjects(other)
	reloaded.registerProjectWork(other)
	read := journeyRequest(t, other, "GET", base, nil, 200)
	if err := json.Unmarshal(read.Body.Bytes(), &saved); err != nil || !reflect.DeepEqual(saved.Tasks, tasks) || !reflect.DeepEqual(saved.Graph.Waves, [][]string{{"schema"}, {"api", "ui"}}) {
		t.Fatalf("restart lost task coordination: %s (%v)", read.Body, err)
	}
	after, _ = os.ReadFile(export)
	if string(before) != string(after) {
		t.Fatal("read projection rewrote canonical context")
	}
}

func TestProjectTaskGraphLegacyPlansRemainValidAndHonest(t *testing.T) {
	for _, tasks := range [][]ProjectTask{nil, {}, {graphTask("legacy")}} {
		if err := validatePlan(ProjectPlan{Tasks: tasks}); err != nil {
			t.Fatal(err)
		}
		graph := BuildProjectTaskGraph(tasks)
		if graph.Waves == nil || graph.ReadyTaskIDs == nil || graph.Blocked == nil || graph.Conflicts == nil || graph.RecordedDoneTaskIDs == nil || graph.UnscopedTaskIDs == nil || len(graph.UnscopedTaskIDs) != len(tasks) {
			t.Fatalf("legacy graph is ambiguous or assumes write scope: %#v", graph)
		}
	}
}

func TestProjectTaskGraphExportBudgetPreservesLegacyMaximumAndRejectsUnreadableNewContext(t *testing.T) {
	plan := ProjectPlan{ProjectID: "p-12345678", Revision: 1, Tasks: make([]ProjectTask, 200)}
	for i := range plan.Tasks {
		plan.Tasks[i] = graphTask(fmt.Sprintf("task-%03d", i))
		// A NUL byte takes six bytes after JSON escaping, the largest expansion.
		plan.Tasks[i].Title = strings.Repeat("\x00", 2000)
		for j := 0; j < 100; j++ {
			plan.Tasks[i].RequirementIDs = append(plan.Tasks[i].RequirementIDs, "r"+strings.Repeat("x", 75)+fmt.Sprintf("%04d", j))
		}
	}
	if err := validatePlan(plan); err != nil {
		t.Fatalf("previously supported maximum plan rejected: %v", err)
	}
	legacy, err := json.MarshalIndent(plan, "", "  ")
	if err != nil || len(legacy)+1 > projectPlanExportBytesLimit {
		t.Fatalf("legacy maximum exceeds export budget: %d (%v)", len(legacy), err)
	}
	t.Logf("Maximum legacy task shape: %d exported bytes, budget %d", len(legacy)+1, projectPlanExportBytesLimit)
	for i := range plan.Tasks {
		plan.Tasks[i].Acceptance = strings.Repeat("\x00", 4000)
	}
	if err := validatePlan(plan); err != errProjectPlanTooLarge {
		t.Fatalf("oversized new task contract was accepted: %v", err)
	}
	// Export is also the common gate used before persisted state/history changes,
	// including reviewed assistant amendments that update revision metadata.
	if _, err := journeyContextExports(journeyState{Plan: &plan}); err != errProjectPlanTooLarge {
		t.Fatalf("oversized imported or amended plan bypassed export admission: %v", err)
	}
}
