package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
)

type azureSecretRef struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Values are never accepted in the operator JSON. The exact Key Vault secret
// versions and user-assigned managed identity are pinned by private config.
type azureSecretMaterialization struct {
	VaultURL   string         `json:"vaultUrl"`
	ClientID   string         `json:"clientId"`
	Directory  string         `json:"directory"`
	APIKey     azureSecretRef `json:"apiKey"`
	AdminKey   azureSecretRef `json:"adminKey"`
	AttachKey  azureSecretRef `json:"attachKey"`
	ClientCert azureSecretRef `json:"clientCert"`
	ClientKey  azureSecretRef `json:"clientKey"`
}

type azureSecretFiles struct {
	APIKey, AdminKey, AttachKey, ClientCert, ClientKey string
}

type azureSecretSource interface {
	GetSecret(context.Context, string, string) ([]byte, error)
}

type keyVaultSecretSource struct{ client *azsecrets.Client }

func (s keyVaultSecretSource) GetSecret(ctx context.Context, name, version string) ([]byte, error) {
	response, err := s.client.GetSecret(ctx, name, version, nil)
	if err != nil || response.Value == nil {
		return nil, errors.New("selected Azure Key Vault secret is unavailable")
	}
	return []byte(*response.Value), nil
}

var (
	azureSecretNamePattern    = regexp.MustCompile(`^[a-zA-Z0-9-]{1,127}$`)
	azureSecretVersionPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)
	azureClientIDPattern      = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

func validAzureSecretMaterialization(cfg azureSecretMaterialization) bool {
	u, err := url.Parse(cfg.VaultURL)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" ||
		!strings.HasSuffix(u.Hostname(), ".vault.azure.net") || len(strings.TrimSuffix(u.Hostname(), ".vault.azure.net")) < 1 ||
		!azureClientIDPattern.MatchString(cfg.ClientID) || !filepath.IsAbs(cfg.Directory) || filepath.Clean(cfg.Directory) != cfg.Directory {
		return false
	}
	refs := []azureSecretRef{cfg.APIKey, cfg.AdminKey, cfg.AttachKey, cfg.ClientCert, cfg.ClientKey}
	names := make(map[string]bool, len(refs))
	for _, ref := range refs {
		if !azureSecretNamePattern.MatchString(ref.Name) || !azureSecretVersionPattern.MatchString(ref.Version) || names[ref.Name] {
			return false
		}
		names[ref.Name] = true
	}
	return true
}

func newAzureSecretSource(cfg azureSecretMaterialization) (azureSecretSource, error) {
	if !validAzureSecretMaterialization(cfg) {
		return nil, errors.New("Azure secret references or managed identity are invalid")
	}
	credential, err := azidentity.NewManagedIdentityCredential(&azidentity.ManagedIdentityCredentialOptions{ID: azidentity.ClientID(cfg.ClientID)})
	if err != nil {
		return nil, errors.New("Azure managed identity is unavailable")
	}
	vault, _ := url.Parse(cfg.VaultURL) // structural validation above succeeded
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	transport.ResponseHeaderTimeout = 10 * time.Second
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, splitErr := net.SplitHostPort(address)
		if splitErr != nil || host != vault.Hostname() || port != "443" {
			return nil, errors.New("Azure Key Vault destination changed")
		}
		ips, lookupErr := net.DefaultResolver.LookupIPAddr(ctx, host)
		if lookupErr != nil || len(ips) == 0 {
			return nil, errors.New("private Azure Key Vault address is unavailable")
		}
		for _, ip := range ips {
			if !ip.IP.IsPrivate() {
				return nil, errors.New("Azure Key Vault did not resolve privately")
			}
		}
		var last error
		for _, ip := range ips {
			conn, dialErr := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
			if dialErr == nil {
				return conn, nil
			}
			last = dialErr
		}
		return nil, last
	}
	httpClient := &http.Client{Transport: transport, Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
		return errors.New("Azure Key Vault redirect refused")
	}}
	client, err := azsecrets.NewClient(cfg.VaultURL, credential, &azsecrets.ClientOptions{ClientOptions: azcore.ClientOptions{Transport: httpClient}})
	if err != nil {
		return nil, errors.New("Azure Key Vault endpoint is invalid")
	}
	return keyVaultSecretSource{client}, nil
}

// materializeAzureSecrets fetches exact versions using only the named managed
// identity. The private files live on tmpfs and are removed on close/failure.
// It performs no provider VM operation and never exposes a secret in errors.
func materializeAzureSecrets(ctx context.Context, cfg azureSecretMaterialization, source azureSecretSource) (azureSecretFiles, func(), error) {
	var files azureSecretFiles
	if source == nil || !validAzureSecretMaterialization(cfg) || !azureSecretTmpfs(cfg.Directory) {
		return files, nil, errors.New("Azure secret directory or references are unsupported")
	}
	info, err := os.Lstat(cfg.Directory)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 {
		return files, nil, errors.New("Azure secret directory is not private")
	}
	root, err := os.MkdirTemp(cfg.Directory, "luci-azure-")
	if err != nil {
		return files, nil, errors.New("Azure private secret directory could not be created")
	}
	cleanup := func() { _ = os.RemoveAll(root) }
	success := false
	defer func() {
		if !success {
			cleanup()
		}
	}()
	for _, row := range []struct {
		name string
		ref  azureSecretRef
		to   *string
	}{
		{"api-key", cfg.APIKey, &files.APIKey}, {"admin-key", cfg.AdminKey, &files.AdminKey},
		{"attach-key", cfg.AttachKey, &files.AttachKey}, {"client-cert", cfg.ClientCert, &files.ClientCert}, {"client-key", cfg.ClientKey, &files.ClientKey},
	} {
		readCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		data, readErr := source.GetSecret(readCtx, row.ref.Name, row.ref.Version)
		cancel()
		if readErr != nil || len(data) == 0 || len(data) > 1<<20 || ctx.Err() != nil {
			return azureSecretFiles{}, nil, fmt.Errorf("Azure %s secret unavailable", row.name)
		}
		path := filepath.Join(root, row.name)
		f, createErr := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if createErr != nil {
			return azureSecretFiles{}, nil, errors.New("Azure private secret file could not be created")
		}
		_, writeErr := f.Write(data)
		if writeErr == nil {
			writeErr = f.Sync()
		}
		closeErr := f.Close()
		if writeErr != nil || closeErr != nil {
			return azureSecretFiles{}, nil, errors.New("Azure private secret file could not be persisted")
		}
		*row.to = path
	}
	success = true
	return files, cleanup, nil
}
