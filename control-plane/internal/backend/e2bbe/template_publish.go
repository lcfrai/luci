package e2bbe

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// TemplatePublication is an explicit operator effect, separate from task
// allocation. The image must already be in a registry reachable by Embed.
// The journal directory is private, durable controller-host storage.
type TemplatePublication struct {
	Endpoint    string
	APIKey      string
	RootCAs     *x509.CertPool
	TemplateID  string
	BuildID     string
	FromImage   string
	JournalPath string
}

type TemplateBuildIntent struct {
	RuntimeRevision string `json:"runtimeRevision"`
	TemplateID      string `json:"templateId"`
	BuildID         string `json:"buildId"`
	FromImage       string `json:"fromImage"`
	ImageDigest     string `json:"imageDigest"`
}

var pinnedOCIImage = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{0,255}@sha256:([0-9a-f]{64})$`)

func (p TemplatePublication) intent() (TemplateBuildIntent, error) {
	match := pinnedOCIImage.FindStringSubmatch(p.FromImage)
	if !validID(p.TemplateID) || !validID(p.BuildID) || len(match) != 2 || !filepath.IsAbs(p.JournalPath) {
		return TemplateBuildIntent{}, ErrUnsupported
	}
	return TemplateBuildIntent{RuntimeRevision: RuntimeRevision, TemplateID: p.TemplateID, BuildID: p.BuildID, FromImage: p.FromImage, ImageDigest: match[1]}, nil
}

func (p TemplatePublication) client() (*providerClient, error) {
	return newClient(Config{Endpoint: p.Endpoint, APIKey: p.APIKey, RootCAs: p.RootCAs})
}

func privatePublicationDirectory(path string) error {
	dir := filepath.Dir(path)
	for current := dir; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return ErrUnsupported
		}
		if current == dir && runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
			return ErrUnsupported
		}
		if parent := filepath.Dir(current); parent == current {
			break
		}
	}
	return nil
}

// StartTemplateBuild checkpoints an immutable intent before the sole POST.
// Any existing journal, including one left by a lost response, prevents a
// replay. Operators may inspect that exact build ID; they must not delete the
// intent to guess whether a provider mutation occurred.
func (p TemplatePublication) StartTemplateBuild(ctx context.Context) (TemplateBuildIntent, error) {
	intent, err := p.intent()
	if err != nil {
		return intent, err
	}
	client, err := p.client()
	if err != nil || privatePublicationDirectory(p.JournalPath) != nil {
		return intent, ErrUnsupported
	}
	data, err := json.Marshal(intent)
	if err != nil {
		return intent, err
	}
	f, err := os.OpenFile(p.JournalPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if errors.Is(err, os.ErrExist) {
		return intent, ErrConflict
	}
	if err != nil {
		return intent, err
	}
	_, writeErr := f.Write(append(data, '\n'))
	if writeErr == nil {
		writeErr = f.Sync()
	}
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil {
		return intent, ErrUnknown
	}
	dir, err := os.Open(filepath.Dir(p.JournalPath))
	if err != nil {
		return intent, ErrUnknown
	}
	syncErr := dir.Sync()
	_ = dir.Close()
	if syncErr != nil {
		return intent, ErrUnknown
	}
	path := "/v2/templates/" + url.PathEscape(p.TemplateID) + "/builds/" + url.PathEscape(p.BuildID)
	status, _, err := client.call(ctx, http.MethodPost, path, nil, map[string]any{
		"fromImage": p.FromImage, "force": false, "steps": []any{},
	}, nil)
	if err != nil || status != http.StatusAccepted {
		return intent, ErrUnknown // The provider may have accepted the build.
	}
	return intent, nil
}

// InspectTemplateBuild is read-only and can run after an uncertain POST. It
// reports only the pinned provider status; guest bytes and KVM behavior still
// need independent verification before configuring this build for dispatch.
func (p TemplatePublication) InspectTemplateBuild(ctx context.Context) (string, error) {
	want, err := p.intent()
	if err != nil || privatePublicationDirectory(p.JournalPath) != nil {
		return "", ErrUnsupported
	}
	info, err := os.Lstat(p.JournalPath)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 ||
		runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 || info.Size() > 4096 {
		return "", ErrUnknown
	}
	f, err := os.Open(p.JournalPath)
	if err != nil {
		return "", ErrUnknown
	}
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		_ = f.Close()
		return "", ErrUnknown
	}
	data, err := io.ReadAll(io.LimitReader(f, 4097))
	_ = f.Close()
	if err != nil || len(data) > 4096 {
		return "", ErrUnknown
	}
	var saved TemplateBuildIntent
	if json.Unmarshal(data, &saved) != nil || saved != want {
		return "", ErrConflict
	}
	client, err := p.client()
	if err != nil {
		return "", err
	}
	var out struct {
		TemplateID string `json:"templateID"`
		BuildID    string `json:"buildID"`
		Status     string `json:"status"`
	}
	path := "/templates/" + p.TemplateID + "/builds/" + p.BuildID + "/status"
	status, _, err := client.call(ctx, http.MethodGet, path, url.Values{"limit": {"0"}}, nil, &out)
	if err != nil || status != http.StatusOK || out.TemplateID != p.TemplateID || out.BuildID != p.BuildID {
		return "", ErrUnknown
	}
	switch out.Status {
	case "waiting", "building", "ready", "error":
		return out.Status, nil
	default:
		return "", ErrUnknown
	}
}

// TemplateImageBinding is suitable for a signed operator inventory. It is not
// the provider build digest, nor evidence that the guest contains these bytes.
func TemplateImageBinding(intent TemplateBuildIntent) string {
	data, _ := json.Marshal(intent)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func (i TemplateBuildIntent) String() string {
	return fmt.Sprintf("template %s build %s image sha256:%s", i.TemplateID, i.BuildID, i.ImageDigest)
}

func (i TemplateBuildIntent) GoString() string { return i.String() }

func (i TemplateBuildIntent) Valid() bool {
	return i.RuntimeRevision == RuntimeRevision && i.TemplateID != "" && i.BuildID != "" &&
		strings.HasSuffix(i.FromImage, "@sha256:"+i.ImageDigest)
}
