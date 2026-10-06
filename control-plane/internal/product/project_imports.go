package product

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

var projectImportIDPattern = regexp.MustCompile(`^imp-[a-f0-9]{24}$`)

type projectImportView struct {
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	Status       string           `json:"status"`
	Message      string           `json:"message"`
	RetainedPath string           `json:"retainedPath,omitempty"`
	Project      *project.Project `json:"project,omitempty"`
}
type projectImportReceipt struct {
	Version int `json:"version"`
	projectImportView
	ActorID           string    `json:"actorId"`
	Root              string    `json:"root"`
	RootIdentity      string    `json:"rootIdentity"`
	RemoteURL         string    `json:"remoteUrl"`
	ContainerIdentity string    `json:"containerIdentity"`
	ParentIdentity    string    `json:"parentIdentity"`
	DirectoryIdentity string    `json:"directoryIdentity"`
	GitIdentity       string    `json:"gitIdentity"`
	Phase             string    `json:"phase"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

func (s *Server) reserveProjectName(path string) bool {
	s.importMu.Lock()
	defer s.importMu.Unlock()
	if s.importNames[path] {
		return false
	}
	if s.importNames == nil {
		s.importNames = map[string]bool{}
	}
	s.importNames[path] = true
	return true
}
func (s *Server) releaseProjectName(path string) {
	s.importMu.Lock()
	delete(s.importNames, path)
	s.importMu.Unlock()
}

// Receipts live outside every project and contain no Git stderr, tokens or
// credential-helper output. The retained directory is never recursively removed.
func (s *Server) importReceiptPath(id string, create bool) (string, error) {
	if !projectImportIDPattern.MatchString(id) {
		return "", os.ErrInvalid
	}
	dir := filepath.Join(s.stateDir, "project-imports")
	if create {
		if err := os.Mkdir(dir, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return "", err
		}
	}
	if _, err := hostpath.ConfineToMounts([]string{s.stateDir}, dir); err != nil {
		return "", err
	}
	if _, err := projectImportDirectoryIdentity(dir); err != nil {
		return "", err
	}
	path := filepath.Join(dir, id+".json")
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() || info.Size() > 16384 {
			return "", os.ErrInvalid
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	return path, nil
}
func (s *Server) readImportReceipt(id string) (projectImportReceipt, error) {
	var receipt projectImportReceipt
	path, err := s.importReceiptPath(id, false)
	if err != nil {
		return receipt, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return receipt, err
	}
	if err = json.Unmarshal(data, &receipt); err != nil {
		return receipt, err
	}
	if receipt.Version != 1 || receipt.ID != id || !validProjectName(receipt.Name) || !filepath.IsAbs(receipt.Root) {
		return receipt, os.ErrInvalid
	}
	return receipt, nil
}
func (s *Server) writeImportReceipt(receipt *projectImportReceipt) error {
	path, err := s.importReceiptPath(receipt.ID, true)
	if err != nil {
		return err
	}
	receipt.UpdatedAt = time.Now().UTC()
	if err = writePrivateAgentJSON(path, receipt); err != nil {
		return err
	}
	return syncImportDirectory(filepath.Dir(path))
}
func importDirectoryMatches(root, path, identity string) bool {
	if identity == "" {
		return false
	}
	if _, err := hostpath.ConfineToMounts([]string{root}, path); err != nil {
		return false
	}
	current, err := projectImportDirectoryIdentity(path)
	return err == nil && current == identity
}
func (s *Server) importView(receipt projectImportReceipt) projectImportView {
	view := receipt.projectImportView
	if filepath.Clean(s.projectRoot()) != receipt.Root || !importDirectoryMatches(receipt.Root, receipt.Root, receipt.RootIdentity) {
		view.Status = "unconfirmed"
		view.Project = nil
		view.Message = "The projects folder changed. Restore the original workspace configuration before checking this import."
		return view
	}
	final := filepath.Join(receipt.Root, receipt.Name)
	if receipt.Phase == "publishing" || receipt.Status == "complete" {
		if importDirectoryMatches(receipt.Root, final, receipt.DirectoryIdentity) && importDirectoryMatches(receipt.Root, filepath.Join(final, ".git"), receipt.GitIdentity) {
			view.Status = "complete"
			view.RetainedPath = ""
			view.Message = "The repository was imported. Open the project to review its files before running code."
			view.Project = &project.Project{ID: project.ProjectID(final), Name: receipt.Name, Path: final}
			return view
		}
		view.Status = "unconfirmed"
		view.Project = nil
		view.Message = "The saved import result no longer matches its project folder. Inspect the retained attempt and projects folder; Luci will not adopt or overwrite another folder."
		return view
	}
	if view.Status == "running" && !s.importActive[receipt.ID] {
		view.Status = "interrupted"
		view.Message = "The previous import was interrupted before publication. Its partial files remain separate from projects. You can retry the same name with a new attempt."
	}
	return view
}
func (s *Server) getProjectImport(w http.ResponseWriter, r *http.Request) {
	actor, ok := authenticatedContextActor(r.Context())
	if !ok {
		journeyError(w, 403, "authentication_required", "Sign in to check an import attempt.")
		return
	}
	s.importMu.Lock()
	defer s.importMu.Unlock()
	receipt, err := s.readImportReceipt(r.PathValue("attemptId"))
	if err != nil || receipt.ActorID != actor.ID {
		journeyError(w, 404, "import_unavailable", "No import receipt is available for this account and attempt. This does not establish whether a delayed request has arrived.")
		return
	}
	journeyJSON(w, 200, s.importView(receipt))
}
func importResponse(w http.ResponseWriter, view projectImportView, status int, code string) {
	if view.Status == "complete" && view.Project != nil {
		journeyJSON(w, status, view.Project)
		return
	}
	journeyJSON(w, status, map[string]any{"error": code, "message": view.Message, "attempt": view})
}

// The short admission lock protects request identity and name reservations. It
// is never held during Git; setup/status and read-only recovery remain usable.
func (s *Server) importProductProject(w http.ResponseWriter, r *http.Request, root, name, remote, id string) {
	actor, _ := authenticatedContextActor(r.Context())
	if id == "" {
		var bytes [12]byte
		if _, err := rand.Read(bytes[:]); err != nil {
			journeyError(w, 500, "import_unavailable", "Could not reserve an import attempt.")
			return
		}
		id = "imp-" + hex.EncodeToString(bytes[:])
	}
	if !projectImportIDPattern.MatchString(id) {
		journeyError(w, 400, "invalid_request", "Use a valid import attempt identifier.")
		return
	}
	root = filepath.Clean(root)
	final := filepath.Join(root, name)
	s.importMu.Lock()
	previous, err := s.readImportReceipt(id)
	if err == nil {
		if previous.ActorID != actor.ID || previous.Root != root || previous.Name != name || previous.RemoteURL != remote {
			s.importMu.Unlock()
			journeyError(w, 409, "import_conflict", "This attempt belongs to a different import. Check its original result before starting another.")
			return
		}
		view := s.importView(previous)
		s.importMu.Unlock()
		status, code := 409, "import_"+view.Status
		if view.Status == "complete" {
			status = 200
		}
		if view.Status == "failed" {
			status = 422
			code = "git_failed"
		}
		importResponse(w, view, status, code)
		return
	}
	if !errors.Is(err, os.ErrNotExist) {
		s.importMu.Unlock()
		journeyError(w, 500, "import_unavailable", "The private import receipt could not be read. No repository was cloned.")
		return
	}
	if s.importNames[final] {
		s.importMu.Unlock()
		journeyError(w, 409, "project_exists", "An import is already preparing that project name. Check its import status before retrying.")
		return
	}
	if _, err = os.Lstat(final); err == nil || !errors.Is(err, os.ErrNotExist) {
		s.importMu.Unlock()
		journeyError(w, 409, "project_exists", "A file or project with that name already exists. Nothing was overwritten.")
		return
	}
	if err = os.MkdirAll(root, 0700); err != nil {
		s.importMu.Unlock()
		journeyError(w, 500, "projects_unavailable", "The projects folder is unavailable.")
		return
	}
	rootID, err := projectImportDirectoryIdentity(root)
	if err != nil {
		s.importMu.Unlock()
		journeyError(w, 409, "projects_unavailable", "The projects folder must be an ordinary directory.")
		return
	}
	receipt := projectImportReceipt{Version: 1, projectImportView: projectImportView{ID: id, Name: name, Status: "running", Message: "Importing the repository into a separate staging folder."}, ActorID: actor.ID, Root: root, RootIdentity: rootID, RemoteURL: remote, Phase: "reserved", CreatedAt: time.Now().UTC()}
	if err = s.writeImportReceipt(&receipt); err != nil {
		s.importMu.Unlock()
		journeyError(w, 500, "import_unavailable", "Could not retain the private import receipt. No repository was cloned.")
		return
	}
	if s.importNames == nil {
		s.importNames = map[string]bool{}
	}
	if s.importActive == nil {
		s.importActive = map[string]bool{}
	}
	s.importNames[final] = true
	s.importActive[id] = true
	s.importMu.Unlock()
	defer func() {
		s.importMu.Lock()
		delete(s.importNames, final)
		delete(s.importActive, id)
		s.importMu.Unlock()
	}()
	fail := func(message string) {
		s.importMu.Lock()
		receipt.Status = "failed"
		receipt.Message = message
		saveErr := s.writeImportReceipt(&receipt)
		s.importMu.Unlock()
		if saveErr != nil {
			receipt.Status = "unconfirmed"
			receipt.Message = "The import did not confirm completion, and its final receipt could not be saved. Check this attempt before retrying."
		}
		importResponse(w, receipt.projectImportView, 422, "git_failed")
	}
	stage, err := s.prepareImportStage(&receipt)
	if err != nil {
		fail("Luci could not safely prepare the import staging folder. Existing files were retained; inspect the projects folder and its permissions before retrying.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	args := []string{"-c", "protocol.allow=never", "-c", "protocol.https.allow=always", "-c", "core.hooksPath=/dev/null", "clone"}
	if runtime.GOOS == "windows" {
		args = append(args, "-c", "core.longpaths=true")
	}
	args = append(args, "--", remote, ".")
	if _, err = s.runner(ctx, "git", args, stage); err != nil {
		message := "Git could not import this repository. Check the HTTPS URL, network and workspace Git credentials, then retry the same project name. Partial files were retained outside the project list."
		if ctx.Err() != nil {
			message = "The import was cancelled or exceeded its time limit before publication. Partial files were retained outside the project list. Check the network or repository size, then retry the same name."
		}
		fail(message)
		return
	}
	if ctx.Err() != nil {
		fail("The import was cancelled before publication. Partial files were retained; retry the same name when ready.")
		return
	}
	if !s.validImportStage(receipt, stage) {
		fail("The projects or staging folder changed during import. Nothing was published or removed. Inspect the retained attempt before retrying.")
		return
	}
	receipt.GitIdentity, err = projectImportDirectoryIdentity(filepath.Join(stage, ".git"))
	if err != nil {
		fail("Git did not produce an ordinary repository. Its files were retained outside the project list for inspection.")
		return
	}
	s.importMu.Lock()
	receipt.Phase = "publishing"
	err = s.writeImportReceipt(&receipt)
	s.importMu.Unlock()
	if err != nil {
		receipt.Phase = "reserved"
		fail("The publication receipt could not be saved. The staged repository was retained and no project was published.")
		return
	}
	s.mu.RLock()
	if ctx.Err() != nil || filepath.Clean(s.config.ProjectsDir) != receipt.Root || !validImportStageFiles(receipt, stage) {
		s.mu.RUnlock()
		receipt.Phase = "reserved"
		fail("The import was cancelled or its folder changed before publication. Nothing was published or removed.")
		return
	}
	err = projectImportRenameNoReplace(stage, final)
	s.mu.RUnlock()
	if err != nil {
		receipt.Phase = "reserved"
		if errors.Is(err, os.ErrExist) {
			fail("A file or project appeared with that name while Git was working. It was not overwritten. Choose another name or inspect the existing project.")
		} else {
			fail("This filesystem could not safely publish the import without overwriting another folder. The staged repository was retained; inspect its folder or choose a supported projects location.")
		}
		return
	}
	// A directory rename is the only publication step. A crash after this point
	// can be resolved read-only from the saved intent and both native identities.
	view, err := s.finishImportPublication(&receipt)
	if err != nil {
		importResponse(w, view, 503, "import_unconfirmed")
		return
	}
	journeyJSON(w, 201, view.Project)
}

// Validation after the native move catches a replaced source/destination in
// the pathname-validation window. Never claim or clean up a foreign folder.
func (s *Server) finishImportPublication(receipt *projectImportReceipt) (projectImportView, error) {
	s.importMu.Lock()
	defer s.importMu.Unlock()
	view := s.importView(*receipt)
	if view.Status != "complete" {
		return view, os.ErrInvalid
	}
	if err := syncImportDirectory(receipt.Root); err != nil {
		return projectImportView{ID: receipt.ID, Name: receipt.Name, Status: "unconfirmed", Message: "The repository was published but its durable result could not be confirmed. Check this import's status before retrying."}, err
	}
	view = s.importView(*receipt)
	if view.Status != "complete" {
		return view, os.ErrInvalid
	}
	receipt.Status = "complete"
	receipt.Project = view.Project
	receipt.Message = view.Message
	if err := s.writeImportReceipt(receipt); err != nil {
		return projectImportView{ID: receipt.ID, Name: receipt.Name, Status: "unconfirmed", Message: "The repository was published but the final receipt could not be confirmed. Check this import's status before retrying."}, err
	}
	return view, nil
}

func (s *Server) prepareImportStage(receipt *projectImportReceipt) (string, error) {
	// The immediate staging container has no .git. Both project catalogues scan
	// immediate children, including legitimate dot-prefixed existing projects.
	container := filepath.Join(receipt.Root, ".li")
	sum := sha256.Sum256([]byte(receipt.Root))
	binding := filepath.Join(s.stateDir, "project-imports", "container-"+hex.EncodeToString(sum[:12])+".json")
	s.importMu.Lock()
	defer s.importMu.Unlock()
	if info, statErr := os.Lstat(binding); statErr == nil {
		if !info.Mode().IsRegular() || info.Size() > 1024 {
			return "", os.ErrInvalid
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return "", statErr
	}
	data, err := os.ReadFile(binding)
	if errors.Is(err, os.ErrNotExist) {
		if err = os.Mkdir(container, 0700); err != nil {
			return "", err
		}
		identity, identityErr := projectImportDirectoryIdentity(container)
		if identityErr != nil {
			return "", identityErr
		}
		if err = writePrivateAgentJSON(binding, struct {
			Identity string `json:"identity"`
		}{identity}); err != nil {
			return "", err
		}
		if err = syncImportDirectory(filepath.Dir(binding)); err != nil {
			return "", err
		}
		if err = syncImportDirectory(receipt.Root); err != nil {
			return "", err
		}
		receipt.ContainerIdentity = identity
	} else if err != nil {
		return "", err
	} else {
		var saved struct {
			Identity string `json:"identity"`
		}
		if json.Unmarshal(data, &saved) != nil || !importDirectoryMatches(receipt.Root, container, saved.Identity) {
			return "", os.ErrInvalid
		}
		receipt.ContainerIdentity = saved.Identity
	}
	if _, err = os.Lstat(filepath.Join(container, ".git")); !errors.Is(err, os.ErrNotExist) {
		return "", os.ErrInvalid
	}
	parent := filepath.Join(container, strings.TrimPrefix(receipt.ID, "imp-"))
	if err = os.Mkdir(parent, 0700); err != nil {
		return "", err
	}
	receipt.ParentIdentity, err = projectImportDirectoryIdentity(parent)
	if err != nil {
		return "", err
	}
	stage := filepath.Join(parent, "r")
	if err = os.Mkdir(stage, 0700); err != nil {
		return "", err
	}
	receipt.DirectoryIdentity, err = projectImportDirectoryIdentity(stage)
	if err != nil {
		return "", err
	}
	receipt.RetainedPath = filepath.Join(".li", strings.TrimPrefix(receipt.ID, "imp-"), "r")
	if err = s.writeImportReceipt(receipt); err != nil {
		return "", err
	}
	return stage, nil
}
func (s *Server) validImportStage(receipt projectImportReceipt, stage string) bool {
	return filepath.Clean(s.projectRoot()) == receipt.Root && validImportStageFiles(receipt, stage)
}
func validImportStageFiles(receipt projectImportReceipt, stage string) bool {
	if receipt.GitIdentity != "" && !importDirectoryMatches(receipt.Root, filepath.Join(stage, ".git"), receipt.GitIdentity) {
		return false
	}
	return importDirectoryMatches(receipt.Root, receipt.Root, receipt.RootIdentity) && importDirectoryMatches(receipt.Root, filepath.Join(receipt.Root, ".li"), receipt.ContainerIdentity) && importDirectoryMatches(receipt.Root, filepath.Dir(stage), receipt.ParentIdentity) && importDirectoryMatches(receipt.Root, stage, receipt.DirectoryIdentity)
}
func syncImportDirectory(path string) error {
	// MoveFileEx WRITE_THROUGH provides the corresponding Windows persistence
	// boundary; directory handles cannot be fsynced through os.File there.
	if runtime.GOOS == "windows" {
		return nil
	}
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
