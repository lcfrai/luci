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
	Endpoint   string
	APIKey     string
	RootCAs    *x509.CertPool
	TemplateID string
	BuildID    string
	FromImage  string
	// The separate, acknowledged private Create journal binds these provider
	// IDs before this command can issue its one build mutation.
	CreateJournalPath string
	JournalPath       string
}

type TemplateBuildIntent struct {
	RuntimeRevision string `json:"runtimeRevision"`
	TemplateID      string `json:"templateId"`
	BuildID         string `json:"buildId"`
	FromImage       string `json:"fromImage"`
	ImageDigest     string `json:"imageDigest"`
}

var pinnedOCIImage = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{0,255}@sha256:([0-9a-f]{64})$`)
var privateTemplateName = regexp.MustCompile(`^luci-[a-z0-9][a-z0-9-]{1,40}-[0-9a-f]{32}$`)

type providerTemplateInventory struct {
	TemplateID string   `json:"templateID"`
	Names      []string `json:"names"`
	Public     *bool    `json:"public"`
	Builds     []struct {
		BuildID   string `json:"buildID"`
		CPUCount  int64  `json:"cpuCount"`
		MemoryMiB int64  `json:"memoryMB"`
	} `json:"builds"`
}

func readTemplateInventory(ctx context.Context, client *providerClient) ([]providerTemplateInventory, error) {
	var templates []providerTemplateInventory
	status, headers, err := client.call(ctx, http.MethodGet, "/v2/templates", url.Values{"limit": {"100"}}, nil, &templates)
	if err != nil || status != http.StatusOK || headers.Get("X-Next-Token") != "" || len(templates) > 100 {
		return nil, ErrUnknown
	}
	return templates, nil
}

// TemplateCreation reserves an exclusively named, private provider template.
// It is a separate operator action from building an image into that template.
// A lost Create response is uncertain and must be reconciled by the operator;
// this method never repeats it.
type TemplateCreation struct {
	Endpoint    string
	APIKey      string
	RootCAs     *x509.CertPool
	Name        string
	CPUCount    int64
	MemoryMiB   int64
	JournalPath string
}

type TemplateCreateIntent struct {
	RuntimeRevision string `json:"runtimeRevision"`
	Name            string `json:"name"`
	CPUCount        int64  `json:"cpuCount"`
	MemoryMiB       int64  `json:"memoryMB"`
}

type TemplateCreateReceipt struct {
	TemplateCreateIntent
	TemplateID string `json:"templateId"`
	BuildID    string `json:"buildId"`
}

func (p TemplateCreation) CreateTemplate(ctx context.Context) (TemplateCreateReceipt, error) {
	var result TemplateCreateReceipt
	if !privateTemplateName.MatchString(p.Name) || p.CPUCount < 1 || p.CPUCount > 16 ||
		p.MemoryMiB < 128 || p.MemoryMiB > 32768 || !filepath.IsAbs(p.JournalPath) ||
		privatePublicationDirectory(p.JournalPath) != nil {
		return result, ErrUnsupported
	}
	client, err := newClient(Config{Endpoint: p.Endpoint, APIKey: p.APIKey, RootCAs: p.RootCAs})
	if err != nil {
		return result, err
	}
	// A prior same-name template could otherwise be mistaken for this request
	// after a lost ACK. The random-suffix name and this complete preflight both
	// precede the durable intent; provider name uniqueness remains authoritative.
	prior, err := readTemplateInventory(ctx, client)
	if err != nil {
		return result, err
	}
	for _, candidate := range prior {
		for _, name := range candidate.Names {
			if name == p.Name {
				return result, ErrConflict
			}
		}
	}
	intent := TemplateCreateIntent{RuntimeRevision: RuntimeRevision, Name: p.Name, CPUCount: p.CPUCount, MemoryMiB: p.MemoryMiB}
	data, _ := json.Marshal(intent)
	if err := checkpointPublicationIntent(p.JournalPath, data); err != nil {
		return result, err
	}
	var response struct {
		TemplateID string `json:"templateID"`
		BuildID    string `json:"buildID"`
		Public     *bool  `json:"public"`
	}
	status, _, err := client.call(ctx, http.MethodPost, "/v3/templates", nil, map[string]any{
		"name": p.Name, "cpuCount": p.CPUCount, "memoryMB": p.MemoryMiB,
	}, &response)
	if err != nil || status != http.StatusAccepted || !validID(response.TemplateID) || !validID(response.BuildID) ||
		response.Public == nil || *response.Public {
		return result, ErrUnknown
	}
	result = TemplateCreateReceipt{TemplateCreateIntent: intent, TemplateID: response.TemplateID, BuildID: response.BuildID}
	data, _ = json.Marshal(result)
	if err = checkpointPublicationIntent(p.JournalPath+".response", data); err != nil {
		return TemplateCreateReceipt{}, ErrUnknown
	}
	return result, nil
}

// ReadTemplateCreation recovers only a durably acknowledged result. A missing
// response file does not prove Create failed; the intent blocks another POST.
func (p TemplateCreation) ReadTemplateCreation() (TemplateCreateReceipt, error) {
	var result TemplateCreateReceipt
	if !privateTemplateName.MatchString(p.Name) || privatePublicationDirectory(p.JournalPath) != nil {
		return result, ErrUnsupported
	}
	intentData, err := readPublicationFile(p.JournalPath)
	var intent TemplateCreateIntent
	if err != nil || json.Unmarshal(intentData, &intent) != nil || intent != (TemplateCreateIntent{
		RuntimeRevision: RuntimeRevision, Name: p.Name, CPUCount: p.CPUCount, MemoryMiB: p.MemoryMiB,
	}) {
		return result, ErrUnknown
	}
	data, err := readPublicationFile(p.JournalPath + ".response")
	if err != nil || json.Unmarshal(data, &result) != nil || result.RuntimeRevision != RuntimeRevision ||
		result.Name != p.Name || result.CPUCount != p.CPUCount || result.MemoryMiB != p.MemoryMiB ||
		result.TemplateCreateIntent != intent ||
		!validID(result.TemplateID) || !validID(result.BuildID) {
		return TemplateCreateReceipt{}, ErrUnknown
	}
	return result, nil
}

// ReconcileTemplateCreation performs one bounded read after a lost Create
// response. Only one exact private template with the requested unique name
// and one resource-matched build can establish the result. Missing, paginated
// or ambiguous inventory remains unknown; no second Create is ever sent.
func (p TemplateCreation) ReconcileTemplateCreation(ctx context.Context) (TemplateCreateReceipt, error) {
	if result, err := p.ReadTemplateCreation(); err == nil {
		return result, nil
	}
	var result TemplateCreateReceipt
	if !privateTemplateName.MatchString(p.Name) || privatePublicationDirectory(p.JournalPath) != nil {
		return result, ErrUnsupported
	}
	intentData, err := readPublicationFile(p.JournalPath)
	var intent TemplateCreateIntent
	if err != nil || json.Unmarshal(intentData, &intent) != nil || intent != (TemplateCreateIntent{
		RuntimeRevision: RuntimeRevision, Name: p.Name, CPUCount: p.CPUCount, MemoryMiB: p.MemoryMiB,
	}) {
		return result, ErrUnknown
	}
	client, err := newClient(Config{Endpoint: p.Endpoint, APIKey: p.APIKey, RootCAs: p.RootCAs})
	if err != nil {
		return result, err
	}
	templates, err := readTemplateInventory(ctx, client)
	if err != nil {
		return result, err
	}
	matches := 0
	for _, candidate := range templates {
		for _, name := range candidate.Names {
			if name != p.Name {
				continue
			}
			matches++
			if matches != 1 || candidate.Public == nil || *candidate.Public ||
				!validID(candidate.TemplateID) || len(candidate.Builds) != 1 ||
				!validID(candidate.Builds[0].BuildID) || candidate.Builds[0].CPUCount != p.CPUCount ||
				candidate.Builds[0].MemoryMiB != p.MemoryMiB {
				return TemplateCreateReceipt{}, ErrUnknown
			}
			result = TemplateCreateReceipt{TemplateCreateIntent: intent, TemplateID: candidate.TemplateID, BuildID: candidate.Builds[0].BuildID}
		}
	}
	if matches != 1 {
		return TemplateCreateReceipt{}, ErrUnknown
	}
	data, _ := json.Marshal(result)
	if err = checkpointPublicationIntent(p.JournalPath+".response", data); errors.Is(err, ErrConflict) {
		return p.ReadTemplateCreation()
	} else if err != nil {
		return TemplateCreateReceipt{}, ErrUnknown
	}
	return result, nil
}

func checkpointPublicationIntent(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if errors.Is(err, os.ErrExist) {
		return ErrConflict
	}
	if err != nil {
		return err
	}
	_, writeErr := f.Write(append(data, '\n'))
	if writeErr == nil {
		writeErr = f.Sync()
	}
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil {
		return ErrUnknown
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return ErrUnknown
	}
	syncErr := dir.Sync()
	_ = dir.Close()
	if syncErr != nil {
		return ErrUnknown
	}
	return nil
}

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

func (p TemplatePublication) createdTemplate() error {
	if !filepath.IsAbs(p.CreateJournalPath) || p.CreateJournalPath == p.JournalPath ||
		privatePublicationDirectory(p.CreateJournalPath) != nil {
		return ErrUnsupported
	}
	data, err := readPublicationFile(p.CreateJournalPath + ".response")
	if err != nil {
		return ErrUnknown
	}
	var receipt TemplateCreateReceipt
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if dec.Decode(&receipt) != nil || dec.Decode(new(any)) != io.EOF ||
		!privateTemplateName.MatchString(receipt.Name) || receipt.RuntimeRevision != RuntimeRevision ||
		receipt.CPUCount < 1 || receipt.CPUCount > 16 || receipt.MemoryMiB < 128 || receipt.MemoryMiB > 32768 {
		return ErrConflict
	}
	created, err := (TemplateCreation{Name: receipt.Name, CPUCount: receipt.CPUCount,
		MemoryMiB: receipt.MemoryMiB, JournalPath: p.CreateJournalPath}).ReadTemplateCreation()
	if err != nil {
		return err
	}
	if created.TemplateID != p.TemplateID || created.BuildID != p.BuildID {
		return ErrConflict
	}
	return nil
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
	if err = p.createdTemplate(); err != nil {
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
	if err = checkpointPublicationIntent(p.JournalPath, data); err != nil {
		return intent, err
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
	if err = p.createdTemplate(); err != nil {
		return "", err
	}
	data, err := readPublicationFile(p.JournalPath)
	if err != nil {
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

func readPublicationFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 ||
		runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 || info.Size() > 4096 {
		return nil, ErrUnknown
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, ErrUnknown
	}
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		_ = f.Close()
		return nil, ErrUnknown
	}
	data, err := io.ReadAll(io.LimitReader(f, 4097))
	_ = f.Close()
	if err != nil || len(data) > 4096 {
		return nil, ErrUnknown
	}
	return data, nil
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
