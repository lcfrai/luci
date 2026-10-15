package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/assistance"
	"github.com/lcfrai/luci/control-plane/internal/httpx"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/project"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

const homePageSize = 64
const homeProjectLimit = 4096

// The gateway additionally filters team collections at response time. This
// worker check happens before deriving any project's private activity.
func (a *API) authorizedHomeProjects(ctx context.Context) ([]project.Project, string, error) {
	actor, _, ok := management.ActorFromContext(ctx)
	if !ok || actor.Disabled || actor.ID == "" {
		return nil, "", management.ErrExecutionAuthority
	}
	projects, err := a.deps.Projects.Projects(ctx)
	if err != nil || len(projects) > homeProjectLimit {
		return nil, "", errors.New("project catalogue unavailable or over limit")
	}
	teamID := management.TeamRuntimeIDFromContext(ctx)
	allowed := make([]project.Project, 0, len(projects))
	for _, p := range projects {
		if err := ctx.Err(); err != nil {
			return nil, "", err
		}
		if a.deps.AuthorizeProjectRead != nil {
			_, err := a.deps.AuthorizeProjectRead(ctx, management.ExecutionActorRef{UserID: actor.ID, TeamID: teamID, ProjectID: p.ID})
			if err != nil {
				continue
			}
		} else if teamID != "" {
			return nil, "", management.ErrExecutionAuthority
		}
		allowed = append(allowed, p)
	}
	sort.Slice(allowed, func(i, j int) bool {
		if allowed[i].Name == allowed[j].Name {
			return allowed[i].ID < allowed[j].ID
		}
		return allowed[i].Name < allowed[j].Name
	})
	return allowed, actor.ID, nil
}

func (a *API) reauthorizeHomeProject(ctx context.Context, actorID, projectID string) error {
	teamID := management.TeamRuntimeIDFromContext(ctx)
	if a.deps.AuthorizeProjectRead == nil {
		if teamID != "" {
			return management.ErrExecutionAuthority
		}
		return nil
	}
	_, err := a.deps.AuthorizeProjectRead(ctx, management.ExecutionActorRef{UserID: actorID, TeamID: teamID, ProjectID: projectID})
	return err
}

// This scan is performed once per Home page, never once per project. It reads
// bounded private metadata and strips results; no prompt or model text enters
// the response. A failed scan makes activity unknown rather than empty.
func (a *API) homeAssistance(ctx context.Context, selected map[string]bool) (map[string][]assistance.Job, error) {
	out := map[string][]assistance.Job{}
	dir := filepath.Join(a.deps.ConversationsDir, "assistance", "jobs")
	if a.deps.ConversationsDir == "" {
		return out, errors.New("assistant history unavailable")
	}
	if _, err := os.Lstat(dir); errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err := codexRegularPath(a.deps.ConversationsDir, dir); err != nil {
		return out, err
	}
	file, err := os.Open(dir)
	if err != nil {
		return out, err
	}
	defer file.Close()
	entries, err := file.ReadDir(4097)
	if err != nil && !errors.Is(err, io.EOF) {
		return out, err
	}
	if len(entries) > 4096 {
		return out, errors.New("assistant history over limit")
	}
	store := a.assistanceStore()
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "assist-") || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		job, readErr := store.Read(strings.TrimSuffix(entry.Name(), ".json"))
		if readErr != nil {
			return out, readErr
		}
		if selected[job.ProjectID] {
			job.Result = nil
			job.OrchestratorResult = nil
			job.OrchestratorContext = nil
			job.UserMessage = ""
			out[job.ProjectID] = append(out[job.ProjectID], job)
		}
	}
	return out, nil
}

type homeProjectJSON struct {
	ID                 string             `json:"id"`
	Name               string             `json:"name"`
	Working            bool               `json:"working"`
	NeedsInput         bool               `json:"needsInput"`
	Queued             bool               `json:"queued"`
	Unknown            bool               `json:"unknown"`
	Stops              []product.HomeStop `json:"stops"`
	NewestObservedStop string             `json:"newestObservedStop,omitempty"`
}
type homeResponse struct {
	Version          int               `json:"version"`
	ObservedAt       string            `json:"observedAt"`
	Total            int               `json:"total"`
	NextOffset       *int              `json:"nextOffset"`
	Revision         int               `json:"revision"`
	PinnedProjectIDs []string          `json:"pinnedProjectIds"`
	Projects         []homeProjectJSON `json:"projects"`
}

func appendHomeStop(row *homeProjectJSON, event product.HomeStop) {
	for _, old := range row.Stops {
		if old.ID == event.ID {
			return
		}
	}
	row.Stops = append(row.Stops, event)
}

func (a *API) homePage(ctx context.Context, projects []project.Project, actorID string, prefs product.HomePreferences, now time.Time, filterAcknowledged bool) ([]homeProjectJSON, error) {
	signals, err := a.deps.Product.HomeProjectSignals(ctx, projects, actorID, now)
	if err != nil {
		return nil, err
	}
	selected := map[string]bool{}
	rows := make([]homeProjectJSON, 0, len(projects))
	for _, p := range projects {
		selected[p.ID] = true
		signal := signals[p.ID]
		rows = append(rows, homeProjectJSON{ID: p.ID, Name: p.Name, Working: signal.Working, NeedsInput: signal.NeedsInput, Queued: signal.Queued, Unknown: signal.Unknown, Stops: append([]product.HomeStop{}, signal.Stops...)})
	}
	sessions, err := a.deps.Store.List()
	if err != nil {
		for i := range rows {
			rows[i].Unknown = true
		}
	} else {
		positions := map[string]int{}
		for i, row := range rows {
			positions[row.ID] = i
		}
		for _, session := range sessions {
			pos, ok := positions[session.ProjectID]
			if !ok || session.AuthOnly {
				continue
			}
			row := &rows[pos]
			switch session.Status {
			case registry.StatusStarting, registry.StatusRunning, registry.StatusIdle:
				row.Working = true
			case registry.StatusExited:
				at := session.StoppedAt
				if at.IsZero() {
					continue // older rows have no immutable observed stop timestamp
				}
				if stop, ok := product.HomeStopEvent("session", session.ProjectID, session.ID, session.SandboxRef+":"+session.CreatedAt.Format(time.RFC3339Nano), at.Format(time.RFC3339Nano), "stopped", now); ok {
					appendHomeStop(row, stop)
				}
			default:
				row.Unknown = true
			}
		}
	}
	jobs, err := a.homeAssistance(ctx, selected)
	if err != nil {
		for i := range rows {
			rows[i].Unknown = true
		}
	} else {
		for i := range rows {
			for _, job := range jobs[rows[i].ID] {
				switch job.Status {
				case "queued", "reserved":
					rows[i].Queued = true
				case "running", "starting":
					rows[i].Working = true
				case "unknown":
					rows[i].Unknown = true
				case "succeeded", "failed", "cancelled":
					at := job.TerminalAt
					if at == "" {
						continue // historical terminal metadata may have been revised later
					}
					if stop, ok := product.HomeStopEvent("assistant", job.ProjectID, job.ID, job.SessionID+":"+job.Status, at, job.Status, now); ok {
						appendHomeStop(&rows[i], stop)
					}
				}
			}
		}
	}
	for i := range rows {
		row := &rows[i]
		sort.Slice(row.Stops, func(a, b int) bool {
			if row.Stops[a].At == row.Stops[b].At {
				return row.Stops[a].ID < row.Stops[b].ID
			}
			return row.Stops[a].At < row.Stops[b].At
		})
		if len(row.Stops) > 32 {
			row.Stops = row.Stops[len(row.Stops)-32:]
		}
		if !filterAcknowledged {
			continue
		}
		acknowledged := map[string]bool{}
		for _, id := range prefs.AcknowledgedStopIDs[row.ID] {
			acknowledged[id] = true
		}
		filtered := make([]product.HomeStop, 0, len(row.Stops))
		for _, stop := range row.Stops {
			if !acknowledged[stop.ID] {
				filtered = append(filtered, stop)
			}
		}
		row.Stops = filtered
		if len(filtered) > 0 {
			row.NewestObservedStop = filtered[len(filtered)-1].ID
		}
	}
	return rows, nil
}

func (a *API) handleProjectHome(w http.ResponseWriter, r *http.Request) {
	if a.deps.Product == nil {
		httpx.WriteProblem(w, 503, "home_unavailable", "Project activity is unavailable.", nil)
		return
	}
	projects, actor, err := a.authorizedHomeProjects(r.Context())
	if err != nil {
		httpx.WriteProblem(w, 503, "home_unavailable", "Current project access could not be checked.", nil)
		return
	}
	prefs, err := a.deps.Product.HomePreferences(r.Context())
	if err != nil {
		httpx.WriteProblem(w, 503, "home_unavailable", "Your project preferences could not be read.", nil)
		return
	}
	offset := 0
	if raw := r.URL.Query().Get("offset"); raw != "" {
		offset, err = strconv.Atoi(raw)
	}
	if err != nil || offset < 0 || offset > len(projects) {
		httpx.WriteProblem(w, 400, "bad_request", "Choose a current Home page.", nil)
		return
	}
	end := min(len(projects), offset+homePageSize)
	page, err := a.homePage(r.Context(), projects[offset:end], actor, prefs, time.Now().UTC(), true)
	if err != nil {
		httpx.WriteProblem(w, 503, "home_unavailable", "Project activity could not be read. Refresh when the workspace is available.", nil)
		return
	}
	accessible := map[string]bool{}
	for _, p := range projects {
		accessible[p.ID] = true
	}
	pins := []string{}
	for _, id := range prefs.PinnedProjectIDs {
		if accessible[id] {
			pins = append(pins, id)
		}
	}
	var next *int
	if end < len(projects) {
		next = &end
	}
	httpx.WriteJSON(w, 200, homeResponse{Version: 1, ObservedAt: time.Now().UTC().Format(time.RFC3339Nano), Total: len(projects), NextOffset: next, Revision: prefs.Revision, PinnedProjectIDs: pins, Projects: page})
}

func (a *API) handleHomePin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProjectID        string `json:"projectId"`
		ExpectedRevision int    `json:"expectedRevision"`
		Pinned           bool   `json:"pinned"`
	}
	if !readHomeJSON(w, r, &req) {
		return
	}
	projects, actor, err := a.authorizedHomeProjects(r.Context())
	if err != nil {
		httpx.WriteProblem(w, 503, "home_unavailable", "Current project access could not be checked.", nil)
		return
	}
	found := false
	for _, p := range projects {
		if p.ID == req.ProjectID {
			found = true
			break
		}
	}
	if !found {
		httpx.WriteProblem(w, 403, "project_unavailable", "This project is unavailable in your workspace.", nil)
		return
	}
	if err := a.reauthorizeHomeProject(r.Context(), actor, req.ProjectID); err != nil {
		httpx.WriteProblem(w, 403, "project_unavailable", "This project is unavailable in your workspace.", nil)
		return
	}
	prefs, err := a.deps.Product.HomePin(r.Context(), req.ProjectID, req.Pinned, req.ExpectedRevision)
	if err != nil {
		homeWriteError(w, err)
		return
	}
	httpx.WriteJSON(w, 200, map[string]any{"revision": prefs.Revision, "pinnedProjectIds": prefs.PinnedProjectIDs})
}

func (a *API) handleHomeAcknowledge(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProjectID        string   `json:"projectId"`
		ExpectedRevision int      `json:"expectedRevision"`
		ObservedStopIDs  []string `json:"observedStopIds"`
	}
	if !readHomeJSON(w, r, &req) {
		return
	}
	projects, actor, err := a.authorizedHomeProjects(r.Context())
	if err != nil {
		httpx.WriteProblem(w, 503, "home_unavailable", "Current project access could not be checked.", nil)
		return
	}
	var selected []project.Project
	for _, p := range projects {
		if p.ID == req.ProjectID {
			selected = []project.Project{p}
			break
		}
	}
	if len(selected) == 0 {
		httpx.WriteProblem(w, 403, "project_unavailable", "This project is unavailable in your workspace.", nil)
		return
	}
	prefs, err := a.deps.Product.HomePreferences(r.Context())
	if err != nil {
		homeWriteError(w, err)
		return
	}
	page, err := a.homePage(r.Context(), selected, actor, prefs, time.Now().UTC(), false)
	if err != nil {
		httpx.WriteProblem(w, 503, "home_unavailable", "Stop activity could not be checked; nothing was acknowledged.", nil)
		return
	}
	current := []string{}
	for _, stop := range page[0].Stops {
		current = append(current, stop.ID)
	}
	if err := a.reauthorizeHomeProject(r.Context(), actor, req.ProjectID); err != nil {
		httpx.WriteProblem(w, 403, "project_unavailable", "This project is unavailable in your workspace.", nil)
		return
	}
	updated, err := a.deps.Product.HomeAcknowledge(r.Context(), req.ProjectID, req.ExpectedRevision, req.ObservedStopIDs, current)
	if err != nil {
		homeWriteError(w, err)
		return
	}
	httpx.WriteJSON(w, 200, map[string]any{"revision": updated.Revision, "acknowledgedStopIds": req.ObservedStopIDs})
}

func homeWriteError(w http.ResponseWriter, err error) {
	if errors.Is(err, product.ErrProjectRunConflict) {
		httpx.WriteProblem(w, 409, "home_changed", "Project activity or your pins changed. Refresh Home before trying again.", nil)
		return
	}
	if errors.Is(err, product.ErrProjectRunInvalid) || errors.Is(err, product.ErrProjectRunLimit) {
		httpx.WriteProblem(w, 400, "bad_request", "Choose a bounded current project preference.", nil)
		return
	}
	httpx.WriteProblem(w, 503, "home_unavailable", "Your project preference could not be safely saved.", nil)
}

func readHomeJSON(w http.ResponseWriter, r *http.Request, out any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if decoder.Decode(out) != nil || decoder.Decode(new(any)) != io.EOF {
		httpx.WriteProblem(w, 400, "bad_request", "Send one bounded Home request.", nil)
		return false
	}
	return true
}
