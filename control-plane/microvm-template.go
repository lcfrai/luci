//go:build ignore

// Run from the control-plane module with: go run ./microvm-template.go
// This explicit operator tool starts or inspects one pinned Embed template
// build. It never provisions a host or allocates a task VM.
package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend/e2bbe"
)

type operatorInput struct {
	Endpoint          string `json:"endpoint"`
	APIKeyFile        string `json:"apiKeyFile"`
	RootCAFile        string `json:"rootCaFile,omitempty"`
	ClientCertFile    string `json:"clientCertFile,omitempty"`
	ClientKeyFile     string `json:"clientKeyFile,omitempty"`
	Name              string `json:"name,omitempty"`
	CPUCount          int64  `json:"cpuCount,omitempty"`
	MemoryMiB         int64  `json:"memoryMB,omitempty"`
	TemplateID        string `json:"templateId"`
	BuildID           string `json:"buildId"`
	FromImage         string `json:"fromImage"`
	CreateJournalPath string `json:"createJournalPath,omitempty"`
	JournalPath       string `json:"journalPath"`
}

func privateFile(path string, limit int64) ([]byte, error) {
	if !filepath.IsAbs(path) {
		return nil, errors.New("private file path must be absolute")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 ||
		runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 || info.Size() < 1 || info.Size() > limit {
		return nil, errors.New("private file is missing, linked, public or oversized")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, errors.New("private file changed while opening")
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || int64(len(data)) > limit {
		return nil, errors.New("private file changed while reading")
	}
	return data, nil
}

func run(mode, configPath string) error {
	data, err := privateFile(configPath, 16<<10)
	if err != nil {
		return err
	}
	var input operatorInput
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || decoder.Decode(new(any)) != io.EOF {
		return errors.New("invalid private template publication configuration")
	}
	if (input.ClientCertFile == "") != (input.ClientKeyFile == "") {
		return errors.New("template gateway client certificate and key must be selected together")
	}
	key, err := privateFile(input.APIKeyFile, 4096)
	if err != nil {
		return err
	}
	var roots *x509.CertPool
	if input.RootCAFile != "" {
		pem, err := privateFile(input.RootCAFile, 1<<20)
		if err != nil {
			return err
		}
		// A selected private gateway root replaces public system roots.
		roots = x509.NewCertPool()
		if !roots.AppendCertsFromPEM(pem) {
			return errors.New("invalid private TLS root certificate")
		}
	}
	var clientCertificate *tls.Certificate
	if input.ClientCertFile != "" {
		certPEM, certErr := privateFile(input.ClientCertFile, 1<<20)
		if certErr != nil {
			return certErr
		}
		keyPEM, keyErr := privateFile(input.ClientKeyFile, 1<<20)
		if keyErr != nil {
			return keyErr
		}
		cert, pairErr := tls.X509KeyPair(certPEM, keyPEM)
		if pairErr != nil || len(cert.Certificate) == 0 {
			return errors.New("template gateway client identity is invalid")
		}
		leaf, parseErr := x509.ParseCertificate(cert.Certificate[0])
		if parseErr != nil || time.Now().Before(leaf.NotBefore) || !time.Now().Before(leaf.NotAfter) {
			return errors.New("template gateway client identity is not currently valid")
		}
		cert.Leaf = leaf
		clientCertificate = &cert
	}
	auth := strings.TrimSpace(string(key))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if mode == "create" || mode == "created" || mode == "reconcile" {
		p := e2bbe.TemplateCreation{Endpoint: input.Endpoint, APIKey: auth, RootCAs: roots, ClientCertificate: clientCertificate,
			Name: input.Name, CPUCount: input.CPUCount, MemoryMiB: input.MemoryMiB, JournalPath: input.JournalPath}
		var receipt e2bbe.TemplateCreateReceipt
		if mode == "create" {
			receipt, err = p.CreateTemplate(ctx)
		} else if mode == "reconcile" {
			receipt, err = p.ReconcileTemplateCreation(ctx)
		} else {
			receipt, err = p.ReadTemplateCreation()
		}
		if err != nil {
			return err
		}
		fmt.Printf("private template %s build %s acknowledged\n", receipt.TemplateID, receipt.BuildID)
		return nil
	}
	p := e2bbe.TemplatePublication{Endpoint: input.Endpoint, APIKey: auth, RootCAs: roots, ClientCertificate: clientCertificate,
		TemplateID: input.TemplateID, BuildID: input.BuildID, FromImage: input.FromImage,
		CreateJournalPath: input.CreateJournalPath, JournalPath: input.JournalPath}
	switch mode {
	case "start":
		intent, err := p.StartTemplateBuild(ctx)
		if err != nil {
			return err
		}
		fmt.Printf("build requested: %s; intent binding %s\n", intent, e2bbe.TemplateImageBinding(intent))
	case "status":
		status, err := p.InspectTemplateBuild(ctx)
		if err != nil {
			return err
		}
		fmt.Printf("pinned provider build status: %s\n", status)
	default:
		return errors.New("mode must be create, created, reconcile, start or status")
	}
	return nil
}

func main() {
	mode := flag.String("mode", "status", "create, created, reconcile, start or status")
	config := flag.String("config", "", "absolute path to private operator JSON")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected positional argument")
		os.Exit(2)
	}
	if err := run(*mode, *config); err != nil {
		fmt.Fprintf(os.Stderr, "microVM template publication: %v\n", err)
		os.Exit(1)
	}
}
