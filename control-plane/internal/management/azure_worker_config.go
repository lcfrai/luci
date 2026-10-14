package management

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// prepareAzureWorkerConfig is a trusted launcher join, not a request parameter.
// The controller gateway keeps account authentication and authority. Each owned
// worker gets separate durable journals/transfers and private materialization.
func prepareAzureWorkerConfig(base []string, root string) (string, error) {
	selected := ""
	found := false
	for _, entry := range base {
		key, value, ok := strings.Cut(entry, "=")
		if ok && strings.EqualFold(key, "LUCI_AZURE_OPERATOR_TEMPLATE") {
			if found {
				return "", errors.New("duplicate Azure controller selection")
			}
			selected = value
			found = true
		}
	}
	if selected == "" {
		return "", nil
	}
	if !filepath.IsAbs(selected) || !filepath.IsAbs(root) {
		return "", errors.New("Azure controller requires absolute private paths")
	}
	for _, path := range []string{selected, root} {
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil || resolved != path {
			return "", errors.New("Azure controller selection and worker root must not traverse symlinks")
		}
	}
	info, err := os.Lstat(selected)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() < 1 || info.Size() > 16<<10 || runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
		return "", errors.New("Azure controller selection is unavailable or not private")
	}
	f, err := os.Open(selected)
	if err != nil {
		return "", err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return "", errors.New("Azure controller selection changed")
	}
	var config map[string]json.RawMessage
	dec := json.NewDecoder(io.LimitReader(f, (16<<10)+1))
	if dec.Decode(&config) != nil || dec.Decode(new(any)) != io.EOF {
		return "", errors.New("invalid Azure controller selection")
	}
	if _, deferred := config["setupDeferred"]; deferred {
		return "", errors.New("Azure task operator is deferred; complete exact template and signed controller finalization first")
	}
	var deployment struct {
		Provider string `json:"provider"`
	}
	var mount struct {
		MountPath string `json:"mountPath"`
	}
	var store map[string]json.RawMessage
	var secrets map[string]json.RawMessage
	if json.Unmarshal(config["deployment"], &deployment) != nil || deployment.Provider != "azure" ||
		json.Unmarshal(config["azureMount"], &mount) != nil || !filepath.IsAbs(mount.MountPath) ||
		json.Unmarshal(config["store"], &store) != nil || store == nil ||
		json.Unmarshal(config["azureSecrets"], &secrets) != nil || secrets == nil {
		return "", errors.New("Azure controller requires protected mount and managed identity selection")
	}
	rel, err := filepath.Rel(mount.MountPath, root)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("Azure worker state must remain on the fenced controller mount")
	}
	for _, key := range []string{"apiKeyFile", "adminApiKeyFile", "attachKeyFile", "clientCertFile", "clientKeyFile"} {
		var value string
		if raw, exists := config[key]; exists && (json.Unmarshal(raw, &value) != nil || value != "") {
			return "", errors.New("Azure worker credentials must use managed identity references")
		}
	}
	encode := func(value string) json.RawMessage { data, _ := json.Marshal(value); return data }
	config["controllerStateDir"] = encode(filepath.Join(root, "state", "cloud-controller"))
	config["stagingDir"] = encode(filepath.Join(root, "state", "cloud-stage"))
	store["directory"] = encode(filepath.Join(root, "state", "cloud-transfer"))
	config["store"], _ = json.Marshal(store)
	// The existing secret adapter creates a separate owner-private MkdirTemp
	// beneath the selected tmpfs directory for every worker, then removes it.
	// Keep exact Key Vault references and controller mount identity unchanged.
	target := filepath.Join(root, "state", "azure-operator.json")
	data, err := json.Marshal(config)
	if err != nil {
		return "", err
	}
	if err := atomicPrivate(target, data); err != nil {
		return "", err
	}
	return target, nil
}
