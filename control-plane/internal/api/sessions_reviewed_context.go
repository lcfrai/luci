package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/harness"
	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/httpx"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

const reviewedContextContainerDir = harness.ReviewedContextDirectory
const reviewedContextDirectory = ".reviewed-context"
const reviewedManifestLimit = 1 << 20

var reviewedSessionID = regexp.MustCompile(`^s-[0-9a-f]{8}$`)
var reviewedDigest = regexp.MustCompile(`^[0-9a-f]{64}$`)

type reviewedContextDelivery struct {
	Status  string `json:"status"`
	Version int    `json:"version,omitempty"`
	Digest  string `json:"digest,omitempty"`
}

// This describes captured delivery, not a fresh filesystem integrity check.
// Session polling remains a read of its recorded receipt, with no host paths.
func sessionReviewedContextDelivery(s registry.Session) *reviewedContextDelivery {
	if s.ImplementationContext == nil {
		return nil
	}
	if s.ReviewedContext == nil {
		return &reviewedContextDelivery{Status: "legacy-live"}
	}
	return &reviewedContextDelivery{Status: "captured", Version: s.ReviewedContext.Version, Digest: s.ReviewedContext.Digest}
}

func (a *API) reviewedContextProblem(w http.ResponseWriter, sessionID string, err error) {
	a.sessionLog().Warn("reviewed context cannot be verified", "sessionId", sessionID, "err", err)
	httpx.WriteProblem(w, http.StatusConflict, httpx.CodeConflict, "This reviewed session's frozen context cannot be verified. Start a new reviewed run from the current plan. Its history, stop and reconciliation controls remain available.", nil)
}

var reviewedFiles = []string{
	".luci/brief.json", ".luci/design-handoff.json", ".luci/design-handoff.md",
	".luci/design.json", ".luci/plan.json", ".luci/specification.json",
	".luci/specification.md", "LUCI.md",
}

// No session ID appears in the content identity: a resumed session receives an
// independent byte-identical copy, whose deletion cannot affect its source.
// Source* remains on ImplementationContext in the registry: fresh code may be
// captured on resume while the reviewed scope delivered to it stays identical.
type reviewedContextManifest struct {
	Version int                             `json:"version"`
	Binding *registry.ImplementationContext `json:"binding"`
	Files   []reviewedContextManifestFile   `json:"files"`
}

type reviewedContextManifestFile struct {
	Path   string `json:"path"`
	Bytes  int    `json:"bytes"`
	Digest string `json:"digest"`
}

func reviewedContextBinding(c *registry.ImplementationContext) *registry.ImplementationContext {
	c = c.Clone()
	if c != nil {
		c.SourceCommit, c.SourceDigest, c.SourceDirty = "", "", false
	}
	return c
}

func reviewedHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (a *API) reviewedContextPath(sessionID string) (string, error) {
	if !reviewedSessionID.MatchString(sessionID) {
		return "", errors.New("invalid reviewed context owner")
	}
	return filepath.Join(a.deps.ConversationsDir, sessionConversationOwner, reviewedContextDirectory, sessionID), nil
}

func reviewedManifest(capture *product.ReviewedContextCapture) ([]byte, *registry.ReviewedContext, error) {
	if capture == nil || capture.Binding == nil || len(capture.Files) != len(reviewedFiles) {
		return nil, nil, errors.New("incomplete reviewed context")
	}
	manifest := reviewedContextManifest{Version: 1, Binding: reviewedContextBinding(capture.Binding), Files: []reviewedContextManifestFile{}}
	files := append([]product.ReviewedContextFile{}, capture.Files...)
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	total := 0
	for i, file := range files {
		total += len(file.Data)
		if file.Path != reviewedFiles[i] || total > product.ReviewedContextBytesLimit {
			return nil, nil, errors.New("invalid or oversized reviewed context")
		}
		manifest.Files = append(manifest.Files, reviewedContextManifestFile{Path: file.Path, Bytes: len(file.Data), Digest: reviewedHash(file.Data)})
	}
	data, err := json.Marshal(manifest)
	if err != nil || len(data) > reviewedManifestLimit {
		return nil, nil, errors.New("invalid reviewed context manifest")
	}
	data = append(data, '\n')
	return data, &registry.ReviewedContext{Version: 1, Digest: reviewedHash(data)}, nil
}

// The exclusive per-session directory is not visible in any sandbox until all
// files and the final manifest are durable, verified, and named by the registry.
// Publishing the manifest last is the commit marker. A process interruption
// before it leaves an incomplete directory which can never be used as context.
func (a *API) prepareReviewedContext(sessionID string, capture *product.ReviewedContextCapture) (*registry.ReviewedContext, backend.Mount, error) {
	data, descriptor, err := reviewedManifest(capture)
	if err != nil {
		return nil, backend.Mount{}, err
	}
	dir, err := a.reviewedContextPath(sessionID)
	if err != nil {
		return nil, backend.Mount{}, err
	}
	if _, err = a.assistDirectory(filepath.Join(sessionConversationOwner, reviewedContextDirectory)); err != nil {
		return nil, backend.Mount{}, err
	}
	// A collision belongs to someone else. Never overwrite it or adopt it as
	// cleanup authority, even when its bytes happen to match this request.
	if err = os.Mkdir(dir, 0700); err != nil {
		return nil, backend.Mount{}, err
	}
	complete := false
	defer func() {
		if !complete {
			_ = a.removeReviewedContext(sessionID)
		}
	}()
	if err = codexPrivateDir(dir); err != nil {
		return nil, backend.Mount{}, err
	}
	if err = os.Mkdir(filepath.Join(dir, ".luci"), 0700); err != nil {
		return nil, backend.Mount{}, err
	}
	if err = codexPrivateDir(filepath.Join(dir, ".luci")); err != nil {
		return nil, backend.Mount{}, err
	}
	for _, file := range capture.Files {
		if err = reviewedWriteFile(filepath.Join(dir, filepath.FromSlash(file.Path)), file.Data); err != nil {
			return nil, backend.Mount{}, err
		}
	}
	if err = reviewedWriteFile(filepath.Join(dir, "manifest.json"), data); err != nil {
		return nil, backend.Mount{}, err
	}
	if _, err = a.readReviewedContext(sessionID, capture.Binding, descriptor); err != nil {
		return nil, backend.Mount{}, err
	}
	complete = true
	return descriptor, backend.Mount{HostPath: dir, ContainerPath: reviewedContextContainerDir, ReadOnly: true}, nil
}

func reviewedWriteFile(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	if err = codexPrivateFile(path); err == nil {
		_, err = file.Write(data)
	}
	if err == nil {
		err = file.Sync()
	}
	return errors.Join(err, file.Close())
}

func reviewedReadFile(root *os.Root, dir, name string, limit int64) ([]byte, error) {
	before, err := root.Lstat(name)
	if err != nil || !before.Mode().IsRegular() || before.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 || before.Size() > limit {
		return nil, errors.New("reviewed context contains an invalid file")
	}
	if err = hostpath.OneName(filepath.Join(dir, name)); err != nil {
		return nil, err
	}
	f, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return nil, errors.New("reviewed context changed while opening")
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || int64(len(data)) > limit {
		return nil, errors.New("reviewed context exceeds its size limit")
	}
	return data, nil
}

func (a *API) readReviewedContext(sessionID string, binding *registry.ImplementationContext, descriptor *registry.ReviewedContext) (*product.ReviewedContextCapture, error) {
	if binding == nil || descriptor == nil || descriptor.Version != 1 || !reviewedDigest.MatchString(descriptor.Digest) {
		return nil, errors.New("This reviewed session has no verified frozen context. Start a new reviewed run from the current plan; its history and stop controls remain available.")
	}
	dir, err := a.reviewedContextPath(sessionID)
	if err != nil {
		return nil, err
	}
	if _, err = hostpath.ResolveOutside(a.projectsDir(), dir); err != nil {
		return nil, err
	}
	if err = codexRegularPath(a.deps.ConversationsDir, filepath.Join(dir, ".luci")); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	data, err := reviewedReadFile(root, dir, "manifest.json", reviewedManifestLimit)
	if err != nil || reviewedHash(data) != descriptor.Digest {
		return nil, errors.New("reviewed context manifest is missing or changed")
	}
	var manifest reviewedContextManifest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&manifest) != nil || manifest.Version != 1 || !reflect.DeepEqual(manifest.Binding, reviewedContextBinding(binding)) || len(manifest.Files) != len(reviewedFiles) {
		return nil, errors.New("reviewed context does not match this session's selected inputs")
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return nil, errors.New("reviewed context manifest contains trailing data")
	}
	allowed := map[string]bool{".": true, ".luci": true, "manifest.json": true}
	capture := &product.ReviewedContextCapture{Binding: binding.Clone(), Files: []product.ReviewedContextFile{}}
	total := 0
	for i, file := range manifest.Files {
		if file.Path != reviewedFiles[i] || file.Bytes < 0 || file.Bytes > product.ReviewedContextBytesLimit-total || !reviewedDigest.MatchString(file.Digest) {
			return nil, errors.New("invalid reviewed context file inventory")
		}
		total += file.Bytes
		body, err := reviewedReadFile(root, dir, file.Path, int64(file.Bytes))
		if err != nil || len(body) != file.Bytes || reviewedHash(body) != file.Digest {
			return nil, errors.New("reviewed context file is missing or changed")
		}
		capture.Files = append(capture.Files, product.ReviewedContextFile{Path: file.Path, Data: body})
		allowed[file.Path] = true
	}
	entries := 0
	if err = fs.WalkDir(root.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		entries++
		if walkErr != nil || entries > len(allowed) || !allowed[name] || entry.Type()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return errors.New("reviewed context contains unexpected entries")
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return capture, nil
}

// Only server-selected fixed filenames inside the exclusively owned session
// directory may be removed. Unknown content or links stop cleanup; there is no
// recursive removal through a path supplied by a record or project.
func (a *API) removeReviewedContext(sessionID string) error {
	dir, err := a.reviewedContextPath(sessionID)
	if err != nil {
		return err
	}
	if err = codexRegularPath(a.deps.ConversationsDir, dir); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	if _, err = hostpath.ResolveOutside(a.projectsDir(), dir); err != nil {
		return err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	if info, err := root.Lstat(".luci"); err == nil {
		if !info.IsDir() || info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return errors.New("unsafe reviewed context cleanup directory")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	names := append(append([]string{}, reviewedFiles...), "manifest.json")
	for _, name := range names {
		info, err := root.Lstat(name)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil || !info.Mode().IsRegular() || info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return errors.New("unsafe reviewed context cleanup target")
		}
		if err = root.Remove(name); err != nil {
			return err
		}
	}
	if err = root.Remove(".luci"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err = root.Close(); err != nil {
		return err
	}
	return os.Remove(dir)
}
