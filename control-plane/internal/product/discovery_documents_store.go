package product

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

// Immutable source/extraction artifacts use the existing private file and
// account/project scope boundary. No project Git file carries original bytes.
type discoveryRequestReceipt struct {
	KeyDigest          string `json:"keyDigest"`
	RequestDigest      string `json:"requestDigest"`
	Action             string `json:"action"`
	AttemptID          string `json:"attemptId,omitempty"`
	ExtractionRevision int    `json:"extractionRevision,omitempty"`
}
type storedDiscoveryDocument struct {
	Version      int                       `json:"version"`
	AccountScope string                    `json:"accountScope"`
	ProjectScope string                    `json:"projectScope"`
	Document     DiscoveryDocument         `json:"document"`
	Requests     []discoveryRequestReceipt `json:"requests"`
}

func (s *Server) discoveryDocumentPath(projectID, name string, create bool) (string, error) {
	if !prototypeReleaseID.MatchString(projectID) || (name != "" && !discoveryDocumentID.MatchString(strings.TrimSuffix(strings.TrimSuffix(name, ".json"), ".original"))) {
		return "", errDiscoveryDocument
	}
	relative := "discovery-documents/" + projectID
	if create {
		for _, part := range []string{"discovery-documents", relative} {
			root, err := guardedProjectPath(s.stateDir, part, true)
			if err != nil {
				return "", err
			}
			if err = os.Mkdir(root, 0700); err != nil && !errors.Is(err, os.ErrExist) {
				return "", err
			}
			if _, err = guardedProjectPath(s.stateDir, part, false); err != nil {
				return "", err
			}
			info, err := os.Lstat(root)
			if err != nil || !info.IsDir() {
				return "", errDiscoveryDocument
			}
			if runtime.GOOS == "windows" {
				err = restrictPrivateFile(root)
			} else {
				err = os.Chmod(root, 0700)
			}
			if err != nil {
				return "", err
			}
		}
	}
	if name != "" {
		relative += "/" + name
	}
	return guardedProjectPath(s.stateDir, relative, create)
}
func discoveryReadPrivate(path string, limit int) ([]byte, error) {
	if err := hostpath.OneName(path); err != nil {
		return nil, err
	}
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() || before.Size() > int64(limit) {
		return nil, errDiscoveryDocument
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(before, opened) || !opened.Mode().IsRegular() {
		return nil, errDiscoveryDocument
	}
	data, err := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if err != nil || len(data) > limit {
		return nil, errDiscoveryDocument
	}
	return data, nil
}

// Caller holds s.mu. GET callers never create or repair artifact directories.
func (s *Server) readDiscoveryDocument(p project.Project, id string) (storedDiscoveryDocument, error) {
	scope, err := s.approvalScopes(p.Path, p.ID)
	if err != nil {
		return storedDiscoveryDocument{}, err
	}
	path, err := s.discoveryDocumentPath(p.ID, id+".json", false)
	if err != nil {
		return storedDiscoveryDocument{}, err
	}
	data, err := discoveryReadPrivate(path, discoveryDocumentRecordBytes)
	if err != nil {
		return storedDiscoveryDocument{}, err
	}
	var saved storedDiscoveryDocument
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&saved) != nil || decoder.Decode(new(any)) != io.EOF || saved.Version != 1 || saved.AccountScope != scope.AccountScope || saved.ProjectScope != scope.ProjectScope || saved.Document.ProjectID != p.ID || saved.Document.ID != id || validateDiscoveryDocument(saved.Document) != nil || len(saved.Requests) > 256 {
		return storedDiscoveryDocument{}, errDiscoveryDocument
	}
	keys := map[string]bool{}
	for _, receipt := range saved.Requests {
		if !hexDigest(receipt.KeyDigest) || !hexDigest(receipt.RequestDigest) || keys[receipt.KeyDigest] || receipt.Action == "" {
			return storedDiscoveryDocument{}, errDiscoveryDocument
		}
		keys[receipt.KeyDigest] = true
	}
	return saved, nil
}
func validateDiscoveryDocument(document DiscoveryDocument) error {
	if !discoveryDocumentID.MatchString(document.ID) || !prototypeReleaseID.MatchString(document.ProjectID) || document.Revision < 1 || !hexDigest(document.Original.Digest) || document.Original.Bytes < 1 || document.Original.Bytes > discoveryOriginalBytes || !discoveryPlainText(document.Original.Name, 255) || strings.TrimSpace(document.Original.Name) == "" || document.Attempts == nil || document.Extractions == nil || len(document.Attempts) > 128 || len(document.Extractions) > 128 {
		return errDiscoveryDocument
	}
	if _, err := time.Parse(time.RFC3339Nano, document.CreatedAt); err != nil {
		return errDiscoveryDocument
	}
	if !validConnectionToken(document.CreatedBy.ID, 128, false) || document.CreatedBy.DisplayName == "" {
		return errDiscoveryDocument
	}
	ids := map[string]bool{}
	for _, attempt := range document.Attempts {
		if !prototypeReleaseID.MatchString(attempt.ID) || ids[attempt.ID] || !validConnectionToken(attempt.Actor.ID, 128, false) || attempt.Actor.DisplayName == "" {
			return errDiscoveryDocument
		}
		ids[attempt.ID] = true
		started, e1 := time.Parse(time.RFC3339Nano, attempt.StartedAt)
		expires, e2 := time.Parse(time.RFC3339Nano, attempt.ExpiresAt)
		if e1 != nil || e2 != nil || !expires.After(started) || expires.Sub(started) > discoveryAttemptLifetime || attempt.Status != "extracting" && !discoveryTerminal(attempt.Status) {
			return errDiscoveryDocument
		}
		if discoveryTerminal(attempt.Status) {
			if _, err := time.Parse(time.RFC3339Nano, attempt.FinishedAt); err != nil {
				return errDiscoveryDocument
			}
		} else if attempt.FinishedAt != "" {
			return errDiscoveryDocument
		}
	}
	for i, extraction := range document.Extractions {
		if extraction.Revision != i+1 || !ids[extraction.AttemptID] || extraction.Digest == "" || validateDiscoveryExtraction(extraction, document.Original) != nil {
			return errDiscoveryDocument
		}
	}
	return nil
}
func (s *Server) saveDiscoveryDocument(saved storedDiscoveryDocument) error {
	if validateDiscoveryDocument(saved.Document) != nil || len(saved.Requests) > 256 {
		return errDiscoveryDocument
	}
	data, err := json.MarshalIndent(saved, "", "  ")
	if err != nil || len(data)+1 > discoveryDocumentRecordBytes {
		return errDiscoveryCapacity
	}
	path, err := s.discoveryDocumentPath(saved.Document.ProjectID, saved.Document.ID+".json", true)
	if err != nil {
		return err
	}
	if _, err = os.Lstat(path); err == nil {
		if hostpath.OneName(path) != nil {
			return errDiscoveryDocument
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err = durableRunJSON(path, saved); err != nil {
		return err
	}
	return syncProjectRunDirectory(filepath.Dir(path))
}
func (s *Server) writeDiscoveryOriginal(p project.Project, id string, original DiscoveryDocumentOriginal, data []byte) error {
	if len(data) != original.Bytes || discoveryTextDigest(string(data)) != original.Digest {
		return errDiscoveryDocument
	}
	path, err := s.discoveryDocumentPath(p.ID, id+".original", true)
	if err != nil {
		return err
	}
	// A retry can recognize its own complete original but cannot adopt changed bytes.
	if existing, err := discoveryReadPrivate(path, discoveryOriginalBytes); err == nil {
		if len(existing) != len(data) || discoveryTextDigest(string(existing)) != original.Digest {
			return errDiscoveryDocument
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	complete := false
	defer func() {
		file.Close()
		if !complete {
			os.Remove(path)
		}
	}()
	if err = restrictPrivateFile(path); err != nil {
		return err
	}
	if _, err = file.Write(data); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	if err = syncProjectRunDirectory(filepath.Dir(path)); err != nil {
		return err
	}
	complete = true
	return nil
}
func discoveryDocumentView(document DiscoveryDocument, now time.Time) DiscoveryDocument {
	document.Attempts = append([]DiscoveryDocumentAttempt{}, document.Attempts...)
	for i := range document.Attempts {
		attempt := &document.Attempts[i]
		expires, err := time.Parse(time.RFC3339Nano, attempt.ExpiresAt)
		if attempt.Status == "extracting" && (err != nil || !now.Before(expires)) {
			attempt.Status = "interrupted"
		}
	}
	return document // Projection only; expired ownership is never an implicit restart.
}
