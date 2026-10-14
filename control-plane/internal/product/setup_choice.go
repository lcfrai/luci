package product

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/management"
)

// SetupChoice is a preference, never an active backing, provisioning grant or
// readiness receipt. It lives in the controller's trusted account state, not a
// project directory, and does not replace the retained workspace connection.
type SetupChoice struct {
	Revision  int    `json:"revision"`
	Mode      string `json:"mode"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

func validSetupMode(mode string) bool {
	return mode == "local" || mode == "bring-your-own-cloud" || mode == "luci-cloud"
}

func (s *Server) loadSetupChoice() error {
	s.setupChoice = SetupChoice{Revision: 1}
	path, err := guardedProjectPath(s.stateDir, "setup-choice.json", false)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := hostpath.OneName(path); err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 4097))
	if err != nil {
		return err
	}
	var saved SetupChoice
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if len(data) > 4096 || dec.Decode(&saved) != nil || dec.Decode(new(any)) != io.EOF || saved.Revision < 2 || !validSetupMode(saved.Mode) {
		return fmt.Errorf("invalid saved workspace setup preference")
	}
	if _, err := time.Parse(time.RFC3339Nano, saved.UpdatedAt); err != nil {
		return fmt.Errorf("invalid saved workspace setup preference timestamp")
	}
	s.setupChoice = saved
	return nil
}

func (s *Server) handleSetupChoice(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ExpectedRevision int    `json:"expectedRevision"`
		Mode             string `json:"mode"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if !validSetupMode(req.Mode) || req.ExpectedRevision < 1 {
		problem(w, 400, "invalid_setup_choice", "Choose Local, Bring your own cloud or Luci Cloud and review the current selection.")
		return
	}
	if _, team := management.TeamAccessFromContext(r.Context()); team || management.TeamRuntimeIDFromContext(r.Context()) != "" {
		problem(w, 403, "setup_choice_forbidden", "Workspace setup choices belong to your personal controller; this team keeps its existing backing.")
		return
	}
	if user, policy, ok := management.ActorFromContext(r.Context()); ok && req.Mode == "bring-your-own-cloud" && !management.PermissionsFor(user, policy).ManageBacking {
		problem(w, 403, "setup_choice_forbidden", "An administrator manages this workspace's cloud setup.")
		return
	}
	s.mu.Lock()
	if s.setupChoice.Revision != req.ExpectedRevision {
		s.mu.Unlock()
		problem(w, 409, "setup_choice_changed", "The saved setup choice changed. Refresh and review it before saving again.")
		return
	}
	if r.Context().Err() != nil {
		s.mu.Unlock()
		return
	}
	next := SetupChoice{Revision: s.setupChoice.Revision + 1, Mode: req.Mode, UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	path, err := guardedProjectPath(s.stateDir, "setup-choice.json", true)
	if err == nil {
		err = durableRunJSON(path, next)
	}
	if err != nil {
		s.mu.Unlock()
		problem(w, 500, "setup_choice_save_failed", "Could not confirm the saved setup choice. Refresh before trying again.")
		return
	}
	s.setupChoice = next
	s.mu.Unlock()
	// No configure callback, connection activation, model call or cloud command.
	writeJSON(w, 200, next)
}
