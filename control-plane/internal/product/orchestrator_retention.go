package product

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

type orchestratorArchive struct {
	Digest string             `json:"digest"`
	Ledger orchestratorLedger `json:"ledger"`
}

var orchestratorStagingName = regexp.MustCompile(`^\.luci-[0-9]+$`)

type orchestratorStaging struct {
	Name   string `json:"name"`
	Bytes  int    `json:"bytes"`
	Digest string `json:"digest"`
}

type orchestratorExport struct {
	Version       int                   `json:"version"`
	ProjectID     string                `json:"projectId"`
	StorageDigest string                `json:"storageDigest"`
	Ledger        orchestratorLedger    `json:"ledger"`
	Archive       []orchestratorArchive `json:"archive"`
	Staging       []orchestratorStaging `json:"uncommittedStaging"`
}
type orchestratorRotateRequest struct {
	ExpectedRevision           int    `json:"expectedRevision"`
	ExpectedStorageDigest      string `json:"expectedStorageDigest"`
	AcknowledgeExportedHistory bool   `json:"acknowledgeExportedHistory"`
}

func (s *Server) orchestratorArchiveDir(id string, create bool) (string, error) {
	if !prototypeReleaseID.MatchString(id) {
		return "", ErrOrchestratorInvalid
	}
	for _, relative := range []string{"orchestrator-archives", "orchestrator-archives/" + id} {
		path, err := guardedProjectPath(s.stateDir, relative, create)
		if err != nil {
			return "", err
		}
		if create {
			if err = os.Mkdir(path, 0700); err != nil && !errors.Is(err, os.ErrExist) {
				return "", ErrOrchestratorInvalid
			}
			if runtime.GOOS == "windows" {
				err = restrictPrivateFile(path)
			} else {
				err = os.Chmod(path, 0700)
			}
			if err != nil {
				return "", ErrOrchestratorInvalid
			}
		}
	}
	return guardedProjectPath(s.stateDir, "orchestrator-archives/"+id, false)
}

// The token covers every verified physical archive, including a pre-commit
// archive left by a crashed rotation. A download cannot authorize later data.
func (s *Server) orchestratorStorageLocked(p project.Project, saved orchestratorLedger) (orchestratorExport, error) {
	out := orchestratorExport{Version: 1, ProjectID: p.ID, Ledger: saved, Archive: []orchestratorArchive{}, Staging: []orchestratorStaging{}}
	dir, err := s.orchestratorArchiveDir(p.ID, false)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return out, ErrOrchestratorInvalid
	}
	if err == nil {
		folder, err := os.Open(dir)
		if err != nil {
			return out, ErrOrchestratorInvalid
		}
		entries, readErr := folder.ReadDir(4)
		folder.Close()
		if readErr != nil && !errors.Is(readErr, io.EOF) || len(entries) > 3 {
			return out, ErrOrchestratorCapacity
		}
		for _, entry := range entries {
			digest := strings.TrimSuffix(entry.Name(), ".json")
			staging := orchestratorStagingName.MatchString(entry.Name())
			if (!staging && (!hexDigest(digest) || entry.Name() != digest+".json")) || !entry.Type().IsRegular() {
				return out, ErrOrchestratorInvalid
			}
			path, err := guardedProjectPath(dir, entry.Name(), false)
			if err != nil || hostpath.OneName(path) != nil {
				return out, ErrOrchestratorInvalid
			}
			f, err := os.Open(path)
			if err != nil {
				return out, ErrOrchestratorInvalid
			}
			info, statErr := f.Stat()
			data, readErr := io.ReadAll(io.LimitReader(f, orchestratorFileLimit+1))
			f.Close()
			if statErr != nil || !info.Mode().IsRegular() || info.Size() > orchestratorFileLimit || readErr != nil || len(data) > orchestratorFileLimit {
				return out, ErrOrchestratorInvalid
			}
			if staging {
				if len(out.Staging) > 0 {
					return out, ErrOrchestratorCapacity
				}
				sum := sha256.Sum256(data)
				out.Staging = append(out.Staging, orchestratorStaging{entry.Name(), len(data), hex.EncodeToString(sum[:])})
				continue
			}
			if len(out.Archive) >= 2 {
				return out, ErrOrchestratorCapacity
			}
			archive, err := decodeOrchestratorLedger(data, saved)
			if err != nil || reviewDigest(archive) != digest {
				return out, ErrOrchestratorInvalid
			}
			out.Archive = append(out.Archive, orchestratorArchive{digest, archive})
		}
	}
	// Directory order is not a storage change.
	if len(out.Archive) == 2 && out.Archive[0].Digest > out.Archive[1].Digest {
		out.Archive[0], out.Archive[1] = out.Archive[1], out.Archive[0]
	}
	if saved.ArchiveDigest != "" {
		found := false
		for _, archive := range out.Archive {
			if archive.Digest == saved.ArchiveDigest {
				found = true
			}
		}
		if !found {
			return out, ErrOrchestratorInvalid
		}
	}
	out.StorageDigest = reviewDigest(struct {
		Ledger  orchestratorLedger
		Archive []orchestratorArchive
		Staging []orchestratorStaging
	}{out.Ledger, out.Archive, out.Staging})
	return out, nil
}
func (s *Server) exportOrchestrator(w http.ResponseWriter, r *http.Request) {
	p, err := s.resolveProject(r.PathValue("id"))
	if err != nil {
		orchestratorProblem(w, err)
		return
	}
	s.mu.RLock()
	saved, err := s.readOrchestrator(p)
	var value orchestratorExport
	if err == nil {
		value, err = s.orchestratorStorageLocked(p, saved)
	}
	s.mu.RUnlock()
	if err != nil {
		orchestratorProblem(w, err)
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="luci-project-discussion.json"`)
	w.Header().Set("Cache-Control", "no-store")
	journeyJSON(w, 200, value)
}
func (s *Server) rotateOrchestrator(w http.ResponseWriter, r *http.Request) {
	var req orchestratorRotateRequest
	if !journeyDecode(w, r, &req) {
		return
	}
	if err := s.rotateOrchestratorHistory(r.Context(), r.PathValue("id"), req); err != nil {
		orchestratorProblem(w, err)
		return
	}
	view, err := s.OrchestratorView(r.Context(), r.PathValue("id"), 0)
	if err != nil {
		orchestratorProblem(w, err)
		return
	}
	journeyJSON(w, 200, view)
}
func (s *Server) rotateOrchestratorHistory(ctx context.Context, id string, req orchestratorRotateRequest) error {
	if _, ok := authenticatedContextActor(ctx); !ok {
		return ErrOrchestratorInvalid
	}
	if !req.AcknowledgeExportedHistory || req.ExpectedRevision < 0 || !hexDigest(req.ExpectedStorageDigest) {
		return ErrOrchestratorConflict
	}
	release, err := s.AcquireGitSessionAdmission(ctx, id)
	if err != nil {
		return err
	}
	defer release()
	p, err := s.resolveProject(id)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	saved, err := s.readOrchestrator(p)
	if err != nil {
		return err
	}
	storage, err := s.orchestratorStorageLocked(p, saved)
	if err != nil {
		return err
	}
	if saved.Revision != req.ExpectedRevision || storage.StorageDigest != req.ExpectedStorageDigest {
		return ErrOrchestratorConflict
	}
	keep := []storedOrchestratorTurn{}
	for i, turn := range saved.Turns {
		if !terminalOrchestrator(turn.Status) {
			return ErrOrchestratorBusy
		}
		unresolved := turn.Status == "succeeded" && turn.Answer != nil && turn.Answer.Blocking && turn.QuestionResolvedBy == ""
		if i >= len(saved.Turns)-8 || unresolved || turn.ApplyIntent != "" {
			keep = append(keep, turn)
		}
	}
	if len(keep) == len(saved.Turns) {
		return ErrOrchestratorCapacity
	}
	dir, err := s.orchestratorArchiveDir(id, true)
	if err != nil {
		return ErrOrchestratorInvalid
	}
	// Only acknowledged, verified unreferenced archives may be removed before
	// the new ledger is durable; the currently referenced archive stays intact.
	for _, archive := range storage.Archive {
		if archive.Digest != saved.ArchiveDigest {
			if err = os.Remove(filepath.Join(dir, archive.Digest+".json")); err != nil {
				return ErrOrchestratorInvalid
			}
		}
	}
	// Temporary writes never became accepted history; their source is still the
	// unchanged active ledger. Their inventory is nevertheless part of the CAS.
	for _, staging := range storage.Staging {
		if err = os.Remove(filepath.Join(dir, staging.Name)); err != nil {
			return ErrOrchestratorInvalid
		}
	}
	digest := reviewDigest(saved)
	archivePath := filepath.Join(dir, digest+".json")
	if err = atomicJSON(archivePath, saved); err != nil {
		return ErrOrchestratorInvalid
	}
	if err = syncProjectRunDirectory(dir); err != nil {
		return ErrOrchestratorInvalid
	}
	old := saved.ArchiveDigest
	saved.ArchiveDigest = digest
	saved.Turns = keep
	saved.Revision++
	if err = s.saveOrchestrator(saved); err != nil {
		return err
	}
	if old != "" && old != digest {
		if err = os.Remove(filepath.Join(dir, old+".json")); err != nil {
			return ErrOrchestratorInvalid
		}
		if err = syncProjectRunDirectory(dir); err != nil {
			return ErrOrchestratorInvalid
		}
	}
	return nil
}
