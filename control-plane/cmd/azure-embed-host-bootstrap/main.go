// azure-embed-host-bootstrap prepares private E2B runtime and gateway inputs
// on the dedicated Azure Embed host. It never provisions Azure or starts E2B.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
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

type secretRef struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type config struct {
	VaultURL           string    `json:"vaultUrl"`
	GatewayHost        string    `json:"gatewayHost"`
	PrivateListenIP    string    `json:"privateListenIp"`
	ControllerIP       string    `json:"controllerPrivateIp"`
	Scope              string    `json:"scope"`
	Generation         string    `json:"generation"`
	HostClientID       string    `json:"hostClientId"`
	ControllerClientID string    `json:"controllerClientId"`
	PrivateDirectory   string    `json:"privateDirectory"`
	ComposePath        string    `json:"composePath"`
	ComposeSHA256      string    `json:"composeSha256"`
	ComposeEnvBase     string    `json:"composeEnvBase"`
	ComposeEnvSHA256   string    `json:"composeEnvSha256"`
	TeamAPIKey         secretRef `json:"teamApiKey"`
	AdminToken         secretRef `json:"adminToken"`
	SandboxTokenSeed   secretRef `json:"sandboxTokenSeed"`
	GatewayServerCert  secretRef `json:"gatewayServerCert"`
	GatewayServerKey   secretRef `json:"gatewayServerKey"`
	GatewayClientCA    secretRef `json:"gatewayClientCa"`
}

type staged struct {
	Directory, ComposeEnv, ServerCert, ServerKey, ClientCA, GatewayConfig string
}

type source interface {
	Get(context.Context, secretRef) ([]byte, error)
}

type vaultSource struct{ client *azsecrets.Client }

func (s vaultSource) Get(ctx context.Context, ref secretRef) ([]byte, error) {
	response, err := s.client.GetSecret(ctx, ref.Name, ref.Version, nil)
	if err != nil || response.Value == nil {
		return nil, errors.New("selected host secret is unavailable")
	}
	return []byte(*response.Value), nil
}

var (
	clientIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	versionPattern  = regexp.MustCompile(`^[0-9a-f]{32}$`)
	namePattern     = regexp.MustCompile(`^[a-zA-Z0-9-]{1,127}$`)
	digestPattern   = regexp.MustCompile(`^[0-9a-f]{64}$`)
	hostPattern     = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{2,252}$`)
	idPattern       = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,127}$`)
	teamKeyPattern  = regexp.MustCompile(`^e2b_[0-9a-f]{32}$`)
	hexKeyPattern   = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

func validConfig(c config) bool {
	u, err := url.Parse(c.VaultURL)
	gatewayIP, controllerIP := net.ParseIP(c.PrivateListenIP), net.ParseIP(c.ControllerIP)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" ||
		!strings.HasSuffix(u.Hostname(), ".vault.azure.net") || len(strings.TrimSuffix(u.Hostname(), ".vault.azure.net")) == 0 ||
		!hostPattern.MatchString(c.GatewayHost) || strings.Contains(c.GatewayHost, "..") ||
		gatewayIP == nil || gatewayIP.To4() == nil || !gatewayIP.IsPrivate() ||
		controllerIP == nil || controllerIP.To4() == nil || !controllerIP.IsPrivate() || controllerIP.Equal(gatewayIP) ||
		!idPattern.MatchString(c.Scope) || !idPattern.MatchString(c.Generation) ||
		!clientIDPattern.MatchString(c.HostClientID) || !clientIDPattern.MatchString(c.ControllerClientID) || c.HostClientID == c.ControllerClientID ||
		!filepath.IsAbs(c.PrivateDirectory) || filepath.Clean(c.PrivateDirectory) != c.PrivateDirectory ||
		!filepath.IsAbs(c.ComposePath) || filepath.Clean(c.ComposePath) != c.ComposePath || !digestPattern.MatchString(c.ComposeSHA256) ||
		!filepath.IsAbs(c.ComposeEnvBase) || filepath.Clean(c.ComposeEnvBase) != c.ComposeEnvBase ||
		!digestPattern.MatchString(c.ComposeEnvSHA256) {
		return false
	}
	names := map[string]bool{}
	for _, ref := range []secretRef{c.TeamAPIKey, c.AdminToken, c.SandboxTokenSeed, c.GatewayServerCert, c.GatewayServerKey, c.GatewayClientCA} {
		if !namePattern.MatchString(ref.Name) || !versionPattern.MatchString(ref.Version) || names[ref.Name] {
			return false
		}
		names[ref.Name] = true
	}
	return true
}

func pinnedFile(path, expected string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > limit {
		return nil, errors.New("pinned E2B host artifact is unavailable")
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 || int64(len(data)) > limit {
		return nil, errors.New("pinned E2B host artifact is unavailable")
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != expected {
		return nil, errors.New("pinned E2B host artifact changed")
	}
	return data, nil
}

func newVaultSource(c config) (source, error) {
	if !validConfig(c) {
		return nil, errors.New("Azure host secret contract is invalid")
	}
	credential, err := azidentity.NewManagedIdentityCredential(&azidentity.ManagedIdentityCredentialOptions{ID: azidentity.ClientID(c.HostClientID)})
	if err != nil {
		return nil, errors.New("Azure host managed identity is unavailable")
	}
	vault, _ := url.Parse(c.VaultURL)
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.ResponseHeaderTimeout = 10 * time.Second
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil || host != vault.Hostname() || port != "443" {
			return nil, errors.New("Azure host Key Vault destination changed")
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil || len(ips) == 0 {
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
	httpClient := &http.Client{Transport: transport, Timeout: 15 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("Azure host Key Vault redirect refused") }}
	client, err := azsecrets.NewClient(c.VaultURL, credential, &azsecrets.ClientOptions{ClientOptions: azcore.ClientOptions{Transport: httpClient}})
	if err != nil {
		return nil, errors.New("Azure host Key Vault endpoint is invalid")
	}
	return vaultSource{client}, nil
}

// The base Compose environment is an immutable nonsecret upstream artifact.
// Secret overrides must be absent so the selected Key Vault versions own all
// three runtime keys. No shell expansion or untrusted secret bytes are used.
func readBase(c config) ([]byte, error) {
	if _, err := pinnedFile(c.ComposePath, c.ComposeSHA256, 1<<20); err != nil {
		return nil, err
	}
	data, err := pinnedFile(c.ComposeEnvBase, c.ComposeEnvSHA256, 64<<10)
	if err != nil {
		return nil, err
	}
	for _, line := range bytes.Split(data, []byte{'\n'}) {
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) == 0 || trimmed[0] == '#' {
			continue
		}
		key, _, found := bytes.Cut(trimmed, []byte{'='})
		key = bytes.TrimSpace(bytes.TrimPrefix(bytes.TrimSpace(key), []byte("export ")))
		if !found || bytes.Equal(key, []byte("TEAM_API_KEY")) || bytes.Equal(key, []byte("ADMIN_TOKEN")) ||
			bytes.Equal(key, []byte("SANDBOX_ACCESS_TOKEN_HASH_SEED")) {
			return nil, errors.New("pinned E2B environment contains an unsafe secret override")
		}
	}
	return data, nil
}

func privateTmpfs(path string) bool {
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 {
		return false
	}
	data, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.Split(line, " - ")
		if len(parts) != 2 || !strings.HasPrefix(parts[1], "tmpfs ") {
			continue
		}
		fields := strings.Fields(parts[0])
		if len(fields) > 4 && fields[4] == path {
			return true
		}
	}
	return false
}

func stage(ctx context.Context, c config, s source, tmpfs func(string) bool) (staged, error) {
	var result staged
	if !validConfig(c) || s == nil || !tmpfs(c.PrivateDirectory) {
		return result, errors.New("Azure host identity or private tmpfs is unavailable")
	}
	base, err := readBase(c)
	if err != nil {
		return result, err
	}
	root, err := os.MkdirTemp(c.PrivateDirectory, "luci-embed-")
	if err != nil {
		return result, errors.New("Azure host private staging failed")
	}
	success := false
	defer func() {
		if !success {
			_ = os.RemoveAll(root)
		}
	}()
	values := make([][]byte, 0, 6)
	for _, ref := range []secretRef{c.TeamAPIKey, c.AdminToken, c.SandboxTokenSeed, c.GatewayServerCert, c.GatewayServerKey, c.GatewayClientCA} {
		readCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		value, readErr := s.Get(readCtx, ref)
		cancel()
		if readErr != nil || len(value) < 1 || len(value) > 1<<20 || ctx.Err() != nil {
			return staged{}, errors.New("selected Azure host secret is unavailable")
		}
		values = append(values, value)
	}
	if !teamKeyPattern.Match(values[0]) || !hexKeyPattern.Match(values[1]) || !hexKeyPattern.Match(values[2]) {
		return staged{}, errors.New("selected E2B runtime key format is invalid")
	}
	pair, err := tls.X509KeyPair(values[3], values[4])
	if err != nil || len(pair.Certificate) == 0 {
		return staged{}, errors.New("selected gateway server certificate is invalid")
	}
	server, err := x509.ParseCertificate(pair.Certificate[0])
	now := time.Now()
	if err != nil || now.Before(server.NotBefore) || !now.Before(server.NotAfter) ||
		server.VerifyHostname(c.GatewayHost) != nil || server.VerifyHostname("49983-sandbox."+c.GatewayHost) != nil ||
		server.VerifyHostname("8080-sandbox."+c.GatewayHost) != nil {
		return staged{}, errors.New("selected gateway server identity is invalid")
	}
	block, rest := pem.Decode(values[5])
	if block == nil || block.Type != "CERTIFICATE" || len(bytes.TrimSpace(rest)) != 0 {
		return staged{}, errors.New("selected gateway client CA is invalid")
	}
	ca, err := x509.ParseCertificate(block.Bytes)
	if err != nil || !ca.IsCA || ca.KeyUsage&x509.KeyUsageCertSign == 0 || now.Before(ca.NotBefore) || !now.Before(ca.NotAfter) {
		return staged{}, errors.New("selected gateway client CA is invalid")
	}
	write := func(name string, data []byte) (string, error) {
		path := filepath.Join(root, name)
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return "", errors.New("Azure host private file could not be created")
		}
		_, err = f.Write(data)
		if err == nil {
			err = f.Sync()
		}
		closeErr := f.Close()
		if err != nil || closeErr != nil {
			return "", errors.New("Azure host private file could not be persisted")
		}
		return path, nil
	}
	env := append(bytes.TrimRight(base, "\n"), '\n')
	env = append(env, []byte("TEAM_API_KEY="+string(values[0])+"\nADMIN_TOKEN="+string(values[1])+"\nSANDBOX_ACCESS_TOKEN_HASH_SEED="+string(values[2])+"\n")...)
	if result.ComposeEnv, err = write("runtime.env", env); err != nil {
		return staged{}, err
	}
	if result.ServerCert, err = write("server.crt", values[3]); err != nil {
		return staged{}, err
	}
	if result.ServerKey, err = write("server.key", values[4]); err != nil {
		return staged{}, err
	}
	if result.ClientCA, err = write("client-ca.crt", values[5]); err != nil {
		return staged{}, err
	}
	// This is the exact private input accepted by azure-gateway-config. It
	// contains file paths and generation identity, never secret values.
	gateway, err := json.Marshal(struct {
		GatewayHost     string `json:"gatewayHost"`
		PrivateListenIP string `json:"privateListenIp"`
		ControllerIP    string `json:"controllerPrivateIp"`
		Scope           string `json:"scope"`
		Generation      string `json:"generation"`
		ServerCertFile  string `json:"serverCertFile"`
		ServerKeyFile   string `json:"serverKeyFile"`
		ClientCAFile    string `json:"clientCaFile"`
	}{c.GatewayHost, c.PrivateListenIP, c.ControllerIP, c.Scope, c.Generation, result.ServerCert, result.ServerKey, result.ClientCA})
	if err != nil {
		return staged{}, errors.New("Azure host gateway input is invalid")
	}
	if result.GatewayConfig, err = write("gateway.json", gateway); err != nil {
		return staged{}, err
	}
	result.Directory = root
	success = true
	return result, nil
}

// Retirement is exact to one generated directory. It refuses unknown files
// and links so an operator cannot use this command to delete shared data.
func retire(c config, path string, tmpfs func(string) bool) error {
	if !validConfig(c) || !tmpfs(c.PrivateDirectory) || filepath.Dir(path) != c.PrivateDirectory ||
		!strings.HasPrefix(filepath.Base(path), "luci-embed-") || len(filepath.Base(path)) <= len("luci-embed-") {
		return errors.New("Azure host secret retirement target is not owned")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 {
		return errors.New("Azure host secret retirement target is not private")
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return errors.New("Azure host secret retirement target is unreadable")
	}
	allowed := map[string]bool{"runtime.env": true, "server.crt": true, "server.key": true, "client-ca.crt": true, "gateway.json": true}
	for _, entry := range entries {
		if !allowed[entry.Name()] {
			return errors.New("Azure host secret retirement target contains unknown data")
		}
		file, err := os.Lstat(filepath.Join(path, entry.Name()))
		if err != nil || !file.Mode().IsRegular() || file.Mode().Perm()&0077 != 0 {
			return errors.New("Azure host secret retirement target contains an unsafe file")
		}
	}
	if err := os.RemoveAll(path); err != nil {
		return errors.New("Azure host secret retirement is incomplete")
	}
	return nil
}

func main() {
	retiring := len(os.Args) == 4 && os.Args[1] == "--retire"
	if len(os.Args) != 2 && !retiring {
		fmt.Fprintln(os.Stderr, "usage: azure-embed-host-bootstrap [--retire] PRIVATE_CONFIG.json [STAGED_DIRECTORY]")
		os.Exit(2)
	}
	configPath := os.Args[1]
	if retiring {
		configPath = os.Args[2]
	}
	info, err := os.Lstat(configPath)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() < 1 || info.Size() > 16<<10 {
		fmt.Fprintln(os.Stderr, "private Azure host configuration is unavailable")
		os.Exit(2)
	}
	file, err := os.Open(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "private Azure host configuration is unavailable")
		os.Exit(2)
	}
	defer file.Close()
	var c config
	decoder := json.NewDecoder(io.LimitReader(file, (16<<10)+1))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&c) != nil || decoder.Decode(new(any)) != io.EOF || !validConfig(c) {
		fmt.Fprintln(os.Stderr, "private Azure host configuration is invalid")
		os.Exit(2)
	}
	if retiring {
		if err := retire(c, os.Args[3], privateTmpfs); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	s, err := newVaultSource(c)
	var result staged
	if err == nil {
		result, err = stage(context.Background(), c, s, privateTmpfs)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	// Only the path is printed. The selected files remain 0600 under a private
	// tmpfs directory and must be retired by the dedicated host service.
	fmt.Println(result.Directory)
}
