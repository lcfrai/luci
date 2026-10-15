package product

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

const discoveryNativeReceiptBytes = 2 << 20

type discoveryExtractionJobs struct {
	mu     sync.Mutex
	active map[string]*discoveryExtractionJob
}
type discoveryExtractionJob struct {
	cancel context.CancelFunc
	done   chan struct{}
}

func prepareDiscoveryExtractionRuntime(value DiscoveryExtractionRuntime) DiscoveryExtractionRuntime {
	value.jobs = &discoveryExtractionJobs{active: map[string]*discoveryExtractionJob{}}
	return value
}
func discoveryNativeJobKey(projectID, documentID, attemptID string) string {
	return projectID + ":" + documentID + ":" + attemptID
}

type discoveryNativeReceipt struct {
	Version                              int `json:"version"`
	AccountScope, ProjectScope           string
	Request                              DiscoveryExtractionRequest
	Outcome                              DiscoveryExtractionOutcome
	CancelKeyDigest, CancelRequestDigest string
}

func (s *Server) discoveryNativePath(projectID, documentID, attemptID, suffix string, create bool) (string, error) {
	if !prototypeReleaseID.MatchString(projectID) || !discoveryDocumentID.MatchString(documentID) || !prototypeReleaseID.MatchString(attemptID) || suffix != ".json" && suffix != ".original" {
		return "", errDiscoveryDocument
	}
	relative := "discovery-extractions/" + projectID
	if create {
		for _, part := range []string{"discovery-extractions", relative} {
			path, err := guardedProjectPath(s.stateDir, part, true)
			if err != nil {
				return "", err
			}
			if err = os.Mkdir(path, 0700); err != nil && !errors.Is(err, os.ErrExist) {
				return "", err
			}
			if _, err = guardedProjectPath(s.stateDir, part, false); err != nil {
				return "", err
			}
			if runtime.GOOS == "windows" {
				if err = restrictPrivateFile(path); err != nil {
					return "", err
				}
			}
		}
	}
	return guardedProjectPath(s.stateDir, relative+"/"+documentID+"-"+attemptID+suffix, create)
}

func (s *Server) readDiscoveryNative(p project.Project, documentID, attemptID string) (discoveryNativeReceipt, error) {
	var result discoveryNativeReceipt
	path, err := s.discoveryNativePath(p.ID, documentID, attemptID, ".json", false)
	if err != nil {
		return result, err
	}
	data, err := discoveryReadPrivate(path, discoveryNativeReceiptBytes)
	if err != nil {
		return result, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&result) != nil || decoder.Decode(new(any)) != io.EOF || result.Version != 1 || !validDiscoveryExtractionRequest(result.Request) || result.Request.ProjectID != p.ID || result.Request.DocumentID != documentID || result.Request.AttemptID != attemptID {
		return result, errDiscoveryDocument
	}
	scope, err := s.approvalScopes(p.Path, p.ID)
	if err != nil {
		return result, err
	}
	if result.AccountScope != scope.AccountScope || result.ProjectScope != scope.ProjectScope || !validDiscoveryNativeOutcome(result.Request, result.Outcome) {
		return result, errDiscoveryDocument
	}
	expected := filepath.Join(filepath.Dir(path), documentID+"-"+attemptID+".original")
	// A settled copy can be absent, but its retained pathname must still be
	// confined and derived from the exact receipt, never arbitrary host input.
	if result.Request.OriginalPath != expected || result.Request.OriginalUserID < 1 || result.Request.OriginalGroupID < 1 || result.Request.ExpectedDocumentRevision < 1 {
		return result, errDiscoveryDocument
	}
	if result.CancelKeyDigest != "" && (!hexDigest(result.CancelKeyDigest) || !hexDigest(result.CancelRequestDigest)) {
		return result, errDiscoveryDocument
	}
	return result, nil
}
func validDiscoveryNativeOutcome(req DiscoveryExtractionRequest, out DiscoveryExtractionOutcome) bool {
	if len(out.Stage) > 80 || len(out.ReasonCode) > 100 || len(out.Message) > 1000 || len(out.Result.Data) > backend.DocumentResultMaxBytes {
		return false
	}
	if (out.Result.Digest != "" || len(out.Result.Data) > 0) && (len(out.Result.Data) == 0 || discoveryTextDigest(string(out.Result.Data)) != out.Result.Digest) {
		return false
	}
	if out.Identity != (backend.DocumentExtractionIdentity{}) && (!backend.ValidateDocumentExtractionIdentity(out.Identity, true) || out.Identity.ProjectID != req.ProjectID || out.Identity.DocumentID != req.DocumentID || out.Identity.AttemptID != req.AttemptID || out.Identity.AllocationID != req.AllocationID || out.Identity.OriginalDigest != req.Original.Digest || out.Identity.ActorDigest != backend.DocumentExtractionActorDigest(req.Actor.UserID, req.Actor.TeamID, req.Actor.ProjectID, req.Actor.Local)) {
		return false
	}
	if out.Ref != "" && !hexDigest(out.Ref) {
		return false
	}
	if !out.ResourcesHeld && out.Identity != (backend.DocumentExtractionIdentity{}) && (!out.CleanupConfirmed || !out.ReaderSettled) {
		return false
	}
	switch out.State {
	case "claimed", "working", "blocked", "unknown", "unavailable", "cancelled", "complete":
		return true
	default:
		return false
	}
}
func (s *Server) saveDiscoveryNative(p project.Project, receipt discoveryNativeReceipt) error {
	if receipt.Version != 1 || !validDiscoveryExtractionRequest(receipt.Request) || !validDiscoveryNativeOutcome(receipt.Request, receipt.Outcome) {
		return errDiscoveryDocument
	}
	scope, err := s.approvalScopes(p.Path, p.ID)
	if err != nil {
		return err
	}
	if receipt.AccountScope != scope.AccountScope || receipt.ProjectScope != scope.ProjectScope {
		return errDiscoveryDocument
	}
	data, err := json.Marshal(receipt)
	if err != nil || len(data)+1 > discoveryNativeReceiptBytes {
		return errDiscoveryCapacity
	}
	path, err := s.discoveryNativePath(p.ID, receipt.Request.DocumentID, receipt.Request.AttemptID, ".json", true)
	if err != nil {
		return err
	}
	// Bounded directory scan counts bytes without loading other histories.
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	entries, readErr := dir.ReadDir(4097)
	dir.Close()
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return readErr
	}
	if len(entries) > 4096 {
		return errDiscoveryCapacity
	}
	var total int64
	found := false
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			return errDiscoveryDocument
		}
		if filepath.Join(filepath.Dir(path), entry.Name()) == path {
			found = true
			continue
		}
		total += info.Size()
		if total > discoveryProjectBytes {
			return errDiscoveryCapacity
		}
	}
	if !found && len(entries) >= 4096 || total+int64(len(data)+1) > discoveryProjectBytes {
		return errDiscoveryCapacity
	}
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() || hostpath.OneName(path) != nil {
			return errDiscoveryDocument
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err = durableRunJSON(path, receipt); err != nil {
		return err
	}
	return syncProjectRunDirectory(filepath.Dir(path))
}

// An extractor sees only a per-attempt private copy. Root-owned originals stay
// private and unchanged; the fixed nonroot uid can read its own exact copy.
func (s *Server) stageDiscoveryNativeOriginal(p project.Project, request *DiscoveryExtractionRequest) error {
	original, err := s.discoveryDocumentPath(p.ID, request.DocumentID+".original", false)
	if err != nil {
		return err
	}
	data, err := discoveryReadPrivate(original, discoveryOriginalBytes)
	if err != nil || len(data) != request.Original.Bytes || discoveryTextDigest(string(data)) != request.Original.Digest {
		return errDiscoveryDocument
	}
	path, err := s.discoveryNativePath(p.ID, request.DocumentID, request.AttemptID, ".original", true)
	if err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	entries, readErr := dir.ReadDir(4097)
	dir.Close()
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return readErr
	}
	if len(entries) > 4094 {
		return errDiscoveryCapacity
	}
	var retained int64
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			return errDiscoveryDocument
		}
		retained += info.Size()
		if retained > discoveryProjectBytes {
			return errDiscoveryCapacity
		}
	}
	if retained+int64(request.Original.Bytes)+discoveryNativeReceiptBytes > discoveryProjectBytes {
		return errDiscoveryCapacity
	}
	uid, gid := os.Getuid(), os.Getgid()
	if uid < 1 {
		uid = 1001
	}
	if gid < 1 {
		gid = 1001
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
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
	if runtime.GOOS == "windows" {
		err = restrictPrivateFile(path)
	} else {
		err = file.Chown(uid, gid)
	}
	if err != nil {
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
	request.OriginalPath, request.OriginalUserID, request.OriginalGroupID = path, uid, gid
	complete = true
	return nil
}

func (s *Server) retireDiscoveryNativeOriginal(p project.Project, receipt discoveryNativeReceipt) error {
	if receipt.Outcome.ResourcesHeld || !receipt.Outcome.CleanupConfirmed || !receipt.Outcome.ReaderSettled {
		return errDiscoveryDocument
	}
	path, err := s.discoveryNativePath(p.ID, receipt.Request.DocumentID, receipt.Request.AttemptID, ".original", false)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil || path != receipt.Request.OriginalPath {
		return errDiscoveryDocument
	}
	data, err := discoveryReadPrivate(path, discoveryOriginalBytes)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil || discoveryTextDigest(string(data)) != receipt.Request.Original.Digest {
		return errDiscoveryDocument
	}
	if err = os.Remove(path); err != nil {
		return err
	}
	return syncProjectRunDirectory(filepath.Dir(path))
}
