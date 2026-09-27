package main

// Phase 7's wiring: the Panacea connection, and the credential store it keeps
// the user's own key in.

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/api"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/connection"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/panacea"
)

// errSecretNotFound stands in for credstore.ErrNotFound.
var errSecretNotFound = errors.New("credential store: no such row")

// memorySecrets keeps the user's Panacea key for the life of THIS PROCESS and
// nowhere else, until Hub task C1's credstore lands (Hub contract 9.1:
// WinCred on Windows, a memory store elsewhere, chosen by PANACEA_APP_CREDSTORE).
//
// It deliberately does not fall back to a file. A pnc_ key in plaintext beside
// the state directory is readable by every process this user runs and survives
// the uninstaller; being asked for the key again after a restart is the smaller
// harm, and it is temporary. The swap is this one constructor.
type memorySecrets struct {
	mu   sync.Mutex
	rows map[string][]byte
}

func newMemorySecrets() *memorySecrets { return &memorySecrets{rows: map[string][]byte{}} }

func (s *memorySecrets) Get(target string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	blob, ok := s.rows[target]
	if !ok {
		return nil, fmt.Errorf("%s: %w", target, errSecretNotFound)
	}
	return blob, nil
}

func (s *memorySecrets) Put(target, _ string, blob []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rows[target] = blob
	return nil
}

func (s *memorySecrets) Delete(target string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.rows, target)
	return nil
}

// newConnection builds the Manager the deploy routes call through. It makes no
// network call: an unreachable Panacea must never stop the cockpit starting,
// and serve runs the first Check in a goroutine once the listener is up.
func newConnection(cfg Config, log *slog.Logger) (*connection.Manager, error) {
	dir := cfg.DeployStateDir
	// statepath.Dir creates nothing (contract 2.7). The origin file is written
	// into this directory, and WriteAtomic's temporary file needs it to exist.
	// A StateDir that is a FILE fails here, which is the whole reason this is
	// fatal: the connect route would otherwise answer every attempt with a
	// write error the user cannot act on.
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create the deploy state directory %s: %w", dir, err)
	}
	agent := "PanaceaApp/" + api.DefaultVersion
	return connection.New(connection.Config{
		Secrets:      newMemorySecrets(),
		IsNotFound:   func(err error) bool { return errors.Is(err, errSecretNotFound) },
		StateDir:     dir,
		LockedOrigin: cfg.PanaceaURL,
		CAFile:       cfg.PanaceaCAFile,
		Timeout:      cfg.PanaceaTimeout,
		UserAgent:    agent,
		Log:          log,
		NewClient: func(origin string, tokens panacea.TokenSource) (deploy.Deployer, error) {
			c, err := panacea.New(panacea.Config{
				Origin: origin, Tokens: tokens, CAFile: cfg.PanaceaCAFile,
				Timeout: cfg.PanaceaTimeout, UserAgent: agent, Log: log,
			})
			if err != nil {
				return nil, err
			}
			// PanaceaDeployer until task 12 gives *panacea.Client its Stream.
			return connection.PanaceaDeployer{Client: c}, nil
		},
	})
}
