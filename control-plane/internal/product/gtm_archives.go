package product

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

const gtmArchiveLimit = 128
const gtmArchiveBytes = 64 << 20
const gtmArchiveSlots = 4

// Correlation metadata only. Immutable snapshots retain the complete original
// rows, including request and orchestrator provenance; the index is not a new
// strategy revision and cannot authorize a replay with different content.
type GTMArchiveKey struct {
	Revision         int    `json:"revision"`
	Digest           string `json:"digest"`
	At               string `json:"at"`
	RequestKeyDigest string `json:"requestKeyDigest"`
	RequestDigest    string `json:"requestDigest"`
	StoredRowDigest  string `json:"storedRowDigest"`
	OriginDigest     string `json:"originDigest"`
}
type GTMArchive struct {
	ID            string          `json:"id"`
	FirstRevision int             `json:"firstRevision"`
	LastRevision  int             `json:"lastRevision"`
	Bytes         int             `json:"bytes"`
	SHA256        string          `json:"sha256"`
	Slot          int             `json:"slot,omitempty"`
	StorageBytes  int             `json:"storageBytes,omitempty"` // reserves all preceding possible attempts
	Keys          []GTMArchiveKey `json:"keys"`
}
type GTMArchiveChoice struct {
	ID            string `json:"id"`
	FirstRevision int    `json:"firstRevision"`
	LastRevision  int    `json:"lastRevision"`
	Revisions     int    `json:"revisions"`
	Bytes         int    `json:"bytes"`
	StorageBytes  int    `json:"storageBytes,omitempty"`
}
type gtmArchiveSnapshot struct {
	Version       int            `json:"version"`
	ProjectID     string         `json:"projectId"`
	AccountScope  string         `json:"accountScope"`
	ProjectScope  string         `json:"projectScope"`
	FirstRevision int            `json:"firstRevision"`
	Revisions     []GTMWorkspace `json:"revisions"`
}
type gtmArchiveSlot struct {
	Path     string
	Original os.FileInfo
}
type preparedGTMArchive struct {
	Path       string
	Descriptor GTMArchive
	Snapshot   gtmArchiveSnapshot
	Slots      []gtmArchiveSlot
}
type GTMHistoryView struct {
	ProjectID  string         `json:"projectId"`
	ArchiveID  string         `json:"archiveId,omitempty"`
	Revisions  []GTMWorkspace `json:"revisions"`
	Historical bool           `json:"historical"`
}

func gtmRawHash(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
func gtmSerialized(value any) ([]byte, error) {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}
func gtmRowValid(row GTMWorkspace, projectID string, revision int) bool {
	if !validGTMOrchestratorOrigin(row.OrchestratorOrigin) || row.ProjectID != projectID || row.Revision != revision || validateGTM(row.Content) != nil || !validArtifactSource(row.Source) || row.SourceDigest != reviewDigest(row.Source) || row.Digest != gtmDigest(row) || !hexDigest(row.RequestKeyDigest) || !hexDigest(row.RequestDigest) || row.Actor.ID == "" || row.Actor.DisplayName == "" {
		return false
	}
	_, err := time.Parse(time.RFC3339Nano, row.At)
	return err == nil
}
func gtmArchiveKey(row GTMWorkspace) GTMArchiveKey {
	return GTMArchiveKey{row.Revision, row.Digest, row.At, row.RequestKeyDigest, row.RequestDigest, reviewDigest(row), reviewDigest(row.OrchestratorOrigin)}
}
func gtmArchiveStorageBytes(row GTMArchive) int {
	if row.StorageBytes == 0 && row.Slot == 0 {
		return row.Bytes
	}
	return row.StorageBytes
}
func validGTMArchiveIndex(ledger gtmLedger) (int, map[string]bool, bool) {
	keys := map[string]bool{}
	if ledger.Version == 1 {
		return 1, keys, ledger.FirstRevision == 0 && len(ledger.Archives) == 0
	}
	if ledger.Version != 2 || len(ledger.Archives) < 1 || len(ledger.Archives) > gtmArchiveLimit || ledger.FirstRevision < 2 {
		return 0, keys, false
	}
	next, total := 1, 0
	ids := map[string]bool{}
	for _, archive := range ledger.Archives {
		if !hexDigest(archive.ID) || ids[archive.ID] || !hexDigest(archive.SHA256) || archive.FirstRevision != next || archive.LastRevision < next || archive.LastRevision-next >= gtmHistoryLimit || archive.Bytes < 1 || archive.Bytes > artifactLedgerLimit || len(archive.Keys) != archive.LastRevision-next+1 || archive.Slot < 0 || archive.Slot >= gtmArchiveSlots || gtmArchiveStorageBytes(archive) < archive.Bytes*(archive.Slot+1) || gtmArchiveStorageBytes(archive) > archive.Bytes*gtmArchiveSlots || gtmArchiveStorageBytes(archive)%archive.Bytes != 0 {
			return 0, keys, false
		}
		ids[archive.ID] = true
		total += gtmArchiveStorageBytes(archive)
		if total > gtmArchiveBytes {
			return 0, keys, false
		}
		for i, key := range archive.Keys {
			if key.Revision != next+i || !hexDigest(key.Digest) || !hexDigest(key.RequestKeyDigest) || !hexDigest(key.RequestDigest) || !hexDigest(key.StoredRowDigest) || !hexDigest(key.OriginDigest) || keys[key.RequestKeyDigest] {
				return 0, keys, false
			}
			if _, err := time.Parse(time.RFC3339Nano, key.At); err != nil {
				return 0, keys, false
			}
			keys[key.RequestKeyDigest] = true
		}
		next = archive.LastRevision + 1
	}
	return next, keys, next == ledger.FirstRevision
}
func gtmArchiveChoices(ledger gtmLedger) []GTMArchiveChoice {
	result := []GTMArchiveChoice{}
	for _, archive := range ledger.Archives {
		result = append(result, GTMArchiveChoice{archive.ID, archive.FirstRevision, archive.LastRevision, len(archive.Keys), archive.Bytes, gtmArchiveStorageBytes(archive)})
	}
	return result
}
func (s *Server) gtmArchivePath(p project.Project, id string, create bool, slots ...int) (string, error) {
	slot := 0
	if len(slots) > 1 {
		return "", errArtifact
	}
	if len(slots) == 1 {
		slot = slots[0]
	}
	if !hexDigest(id) || slot < 0 || slot >= gtmArchiveSlots {
		return "", errArtifact
	}
	suffix := ""
	if slot > 0 {
		suffix = "-" + strconv.Itoa(slot)
	}
	return guardedProjectPath(s.stateDir, "gtm-archive-"+p.ID+"-"+id+suffix+".json", create)
}

// The original opened file is checked against its path and bounded before a
// strict JSON decode. No parser accepts another project/account's snapshot.
func readGTMArchiveRaw(path string, limit int) ([]byte, os.FileInfo, error) {
	if limit < 1 || limit > artifactLedgerLimit || hostpath.OneName(path) != nil {
		return nil, nil, errArtifact
	}
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() || before.Size() < 0 || before.Size() > int64(limit) {
		return nil, nil, errArtifact
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, errArtifact
	}
	opened, statErr := file.Stat()
	if statErr != nil || !os.SameFile(before, opened) {
		file.Close()
		return nil, nil, errArtifact
	}
	raw, readErr := io.ReadAll(io.LimitReader(file, int64(limit)+1))
	after, endErr := file.Stat()
	closeErr := file.Close()
	current, pathErr := os.Lstat(path)
	if readErr != nil || endErr != nil || closeErr != nil || pathErr != nil || hostpath.OneName(path) != nil || !os.SameFile(opened, after) || !os.SameFile(after, current) || after.Size() != opened.Size() || current.Size() != after.Size() || !after.ModTime().Equal(opened.ModTime()) || !current.ModTime().Equal(after.ModTime()) || len(raw) > limit || len(raw) != int(after.Size()) {
		return nil, nil, errArtifact
	}
	return raw, current, nil
}
func readGTMArchiveFile(path string, descriptor GTMArchive, owner gtmLedger) (gtmArchiveSnapshot, error) {
	var snapshot gtmArchiveSnapshot
	raw, _, err := readGTMArchiveRaw(path, descriptor.Bytes)
	if err != nil || len(raw) != descriptor.Bytes || gtmRawHash(raw) != descriptor.SHA256 {
		return snapshot, errArtifact
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&snapshot) != nil || decoder.Decode(new(any)) != io.EOF || snapshot.Version != 1 || snapshot.ProjectID != owner.ProjectID || snapshot.AccountScope != owner.AccountScope || snapshot.ProjectScope != owner.ProjectScope || snapshot.FirstRevision != descriptor.FirstRevision || snapshot.Revisions == nil || len(snapshot.Revisions) != len(descriptor.Keys) {
		return snapshot, errArtifact
	}
	for i, row := range snapshot.Revisions {
		if !gtmRowValid(row, owner.ProjectID, descriptor.FirstRevision+i) || gtmArchiveKey(row) != descriptor.Keys[i] {
			return snapshot, errArtifact
		}
	}
	return snapshot, nil
}
func (s *Server) readGTMArchive(p project.Project, ledger gtmLedger, archive GTMArchive) (gtmArchiveSnapshot, error) {
	path, err := s.gtmArchivePath(p, archive.ID, false, archive.Slot)
	if err != nil {
		return gtmArchiveSnapshot{}, err
	}
	return readGTMArchiveFile(path, archive, ledger)
}
func (s *Server) gtmRevisionByRequest(p project.Project, ledger gtmLedger, key string) (*GTMWorkspace, error) {
	for _, row := range ledger.Revisions {
		if row.RequestKeyDigest == key {
			return &row, nil
		}
	}
	for _, archive := range ledger.Archives {
		for _, index := range archive.Keys {
			if index.RequestKeyDigest != key {
				continue
			}
			snapshot, err := s.readGTMArchive(p, ledger, archive)
			if err != nil {
				return nil, err
			}
			for _, row := range snapshot.Revisions {
				if row.RequestKeyDigest == key {
					return &row, nil
				}
			}
			return nil, errArtifact
		}
	}
	return nil, nil
}
func (s *Server) gtmRevisionByNumber(p project.Project, ledger gtmLedger, revision int) (*GTMWorkspace, error) {
	for _, row := range ledger.Revisions {
		if row.Revision == revision {
			return &row, nil
		}
	}
	for _, archive := range ledger.Archives {
		if revision < archive.FirstRevision || revision > archive.LastRevision {
			continue
		}
		snapshot, err := s.readGTMArchive(p, ledger, archive)
		if err != nil {
			return nil, err
		}
		return &snapshot.Revisions[revision-archive.FirstRevision], nil
	}
	return nil, errArtifact
}
func (s *Server) chooseGTMArchiveSlot(p project.Project, owner gtmLedger, descriptor *GTMArchive) ([]gtmArchiveSlot, error) {
	slots := make([]gtmArchiveSlot, gtmArchiveSlots)
	complete, empty, highest := -1, -1, -1
	for i := 0; i < gtmArchiveSlots; i++ {
		path, err := s.gtmArchivePath(p, descriptor.ID, true, i)
		if err != nil {
			return nil, err
		}
		slots[i].Path = path
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			if empty < 0 {
				empty = i
			}
			continue
		}
		if err != nil || !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > int64(descriptor.Bytes) {
			return nil, errArtifact
		}
		raw, original, err := readGTMArchiveRaw(path, descriptor.Bytes)
		if err != nil {
			return nil, err
		}
		slots[i].Original = original
		highest = i
		if len(raw) == descriptor.Bytes && gtmRawHash(raw) == descriptor.SHA256 {
			if _, err = readGTMArchiveFile(path, *descriptor, owner); err != nil {
				return nil, err
			}
			if complete < 0 {
				complete = i
			}
		}
	}
	selected := complete
	if selected < 0 {
		selected = empty
	}
	if selected < 0 {
		return nil, &gtmSaveFailure{413, "gtm_archive_attempts_full", "Four archive attempts are retained for this chunk. Export the active history and ask for storage recovery; no original file was overwritten or deleted."}
	}
	if selected > highest {
		highest = selected
	}
	descriptor.Slot = selected
	descriptor.StorageBytes = descriptor.Bytes * (highest + 1)
	return slots, nil
}

// Recheck the exact bounded slot namespace before one write and again before
// publication. A linked/changed/oversized original is not a partial to bypass.
func checkGTMArchiveSlots(archive *preparedGTMArchive, created os.FileInfo) error {
	if len(archive.Slots) != gtmArchiveSlots {
		return errArtifact
	}
	for i, slot := range archive.Slots {
		expected := slot.Original
		if i == archive.Descriptor.Slot && created != nil {
			expected = created
		}
		current, err := os.Lstat(slot.Path)
		if expected == nil {
			if !errors.Is(err, os.ErrNotExist) {
				return errArtifact
			}
			continue
		}
		if err != nil || hostpath.OneName(slot.Path) != nil || !current.Mode().IsRegular() || !os.SameFile(expected, current) || current.Size() != expected.Size() || current.Size() > int64(archive.Descriptor.Bytes) || !current.ModTime().Equal(expected.ModTime()) {
			return errArtifact
		}
	}
	return nil
}

// Called only after the new row passed current source, link and global CAS
// validation. It is a preparation step: no archive or ledger is written here.
func (s *Server) prepareGTMArchive(p project.Project, prepared *preparedGTMRevision) error {
	raw, err := gtmSerialized(prepared.Ledger)
	if err != nil {
		return err
	}
	if len(prepared.Ledger.Revisions) <= gtmHistoryLimit && len(raw) <= artifactLedgerLimit {
		return nil
	}
	old := prepared.Ledger.Revisions[:len(prepared.Ledger.Revisions)-1]
	if len(old) == 0 || len(prepared.Ledger.Archives) >= gtmArchiveLimit {
		return &gtmSaveFailure{413, "gtm_storage_full", "GTM storage capacity is full. All saved revisions remain available for history and export; no revision was deleted."}
	}
	first := old[0].Revision
	snapshot := gtmArchiveSnapshot{1, p.ID, prepared.Ledger.AccountScope, prepared.Ledger.ProjectScope, first, append([]GTMWorkspace{}, old...)}
	archiveRaw, err := gtmSerialized(snapshot)
	if err != nil || len(archiveRaw) > artifactLedgerLimit {
		return &gtmSaveFailure{413, "gtm_storage_full", "The current GTM chunk cannot fit in its immutable archive. Saved revisions remain unchanged and exportable."}
	}
	descriptor := GTMArchive{ID: reviewDigest([]string{"gtm-archive", snapshot.AccountScope, snapshot.ProjectScope, strconv.Itoa(first), strconv.Itoa(old[len(old)-1].Revision), old[len(old)-1].Digest}), FirstRevision: first, LastRevision: old[len(old)-1].Revision, Bytes: len(archiveRaw), SHA256: gtmRawHash(archiveRaw), Keys: []GTMArchiveKey{}}
	for _, row := range old {
		descriptor.Keys = append(descriptor.Keys, gtmArchiveKey(row))
	}
	slots, err := s.chooseGTMArchiveSlot(p, prepared.Ledger, &descriptor)
	if err != nil {
		return err
	}
	total := gtmArchiveStorageBytes(descriptor)
	for _, archive := range prepared.Ledger.Archives {
		total += gtmArchiveStorageBytes(archive)
	}
	if total > gtmArchiveBytes {
		return &gtmSaveFailure{413, "gtm_storage_full", "The 64 MiB GTM archive capacity, including reserved interrupted attempts, is full. History remains readable and exportable; no automatic deletion is allowed."}
	}
	path := slots[descriptor.Slot].Path
	prepared.Ledger.Version = 2
	prepared.Ledger.FirstRevision = prepared.Row.Revision
	prepared.Ledger.Archives = append(append([]GTMArchive{}, prepared.Ledger.Archives...), descriptor)
	prepared.Ledger.Revisions = []GTMWorkspace{prepared.Row}
	raw, err = gtmSerialized(prepared.Ledger)
	if err != nil || len(raw) > artifactLedgerLimit {
		return &gtmSaveFailure{413, "gtm_storage_full", "The GTM index and current revision exceed private storage capacity. Saved history remains unchanged."}
	}
	prepared.Archive = &preparedGTMArchive{path, descriptor, snapshot, slots}
	return nil
}
func persistGTMArchive(ctx context.Context, prepared preparedGTMRevision) error {
	if prepared.Archive == nil {
		return nil
	}
	archive := prepared.Archive
	raw, err := gtmSerialized(archive.Snapshot)
	if err != nil || len(raw) != archive.Descriptor.Bytes || gtmRawHash(raw) != archive.Descriptor.SHA256 {
		return errArtifact
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if checkGTMArchiveSlots(archive, nil) != nil {
		return errArtifact
	}
	if archive.Slots[archive.Descriptor.Slot].Original != nil {
		_, err = readGTMArchiveFile(archive.Path, archive.Descriptor, prepared.Ledger)
		if err == nil {
			err = syncProjectRunDirectory(filepath.Dir(archive.Path))
		}
		if err == nil {
			err = checkGTMArchiveSlots(archive, nil)
		}
		return err
	}
	file, err := os.OpenFile(archive.Path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	// A partial exclusive file is never overwritten, deleted or replayed as an
	// archive. The original active ledger still contains every preceding row.
	err = restrictPrivateFile(archive.Path)
	if err == nil {
		var count int
		count, err = file.Write(raw)
		if err == nil && count != len(raw) {
			err = io.ErrShortWrite
		}
	}
	if err == nil {
		err = file.Sync()
	}
	created, statErr := file.Stat()
	if err == nil {
		err = statErr
	}
	closed := file.Close()
	if err == nil {
		err = closed
	}
	if err == nil {
		err = checkGTMArchiveSlots(archive, created)
	}
	if err == nil {
		err = syncProjectRunDirectory(filepath.Dir(archive.Path))
	}
	return err
}

// Existing GET only: old content is explicitly requested and bounded. There is
// no archive mutation, implicit restore, draft replacement or provider action.
func (s *Server) gtmHistoryResponse(w http.ResponseWriter, r *http.Request, p project.Project, ledger gtmLedger) bool {
	query := r.URL.Query()
	archiveID, revisionText := query.Get("history"), query.Get("revision")
	_, hasArchive := query["history"]
	_, hasRevision := query["revision"]
	if !hasArchive && !hasRevision {
		return false
	}
	if len(query) != 1 || len(query["history"])+len(query["revision"]) != 1 || archiveID != "" && revisionText != "" {
		journeyError(w, 400, "gtm_history_invalid", "Choose exactly one retained history chunk or revision.")
		return true
	}
	view := GTMHistoryView{ProjectID: p.ID, Revisions: []GTMWorkspace{}, Historical: true}
	var err error
	if archiveID != "" {
		found := false
		for _, archive := range ledger.Archives {
			if archive.ID != archiveID {
				continue
			}
			found = true
			var snapshot gtmArchiveSnapshot
			snapshot, err = s.readGTMArchive(p, ledger, archive)
			if err == nil {
				view.ArchiveID = archiveID
				view.Revisions = snapshot.Revisions
			}
			break
		}
		if !found {
			err = errArtifact
		}
	} else {
		revision, parseErr := strconv.Atoi(revisionText)
		if parseErr != nil || revision < 1 || strconv.Itoa(revision) != revisionText {
			err = errArtifact
		} else {
			var row *GTMWorkspace
			row, err = s.gtmRevisionByNumber(p, ledger, revision)
			if err == nil {
				view.Revisions = append(view.Revisions, *row)
			}
		}
	}
	if err != nil {
		artifactProblem(w, err)
		return true
	}
	if r.Context().Err() != nil || !artifactAccess(w, r, false) {
		return true
	}
	journeyJSON(w, 200, view)
	return true
}
