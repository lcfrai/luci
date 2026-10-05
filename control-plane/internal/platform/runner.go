package platform

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Runner never places runtime secrets in arguments. Input may contain a source
// archive; output is bounded even when an application writes continuously.
type Runner func(context.Context, string, []string, string, []byte) ([]byte, error)
type boundedBuffer struct {
	bytes.Buffer
	limit int
}

// Diagnostics stay in process memory and only an allowlisted cause leaves the
// runner. Docker/Compose stderr can include runtime configuration or tokens.
type commandDiagnosticBuffer struct{ data []byte }

func (b *commandDiagnosticBuffer) Write(p []byte) (int, error) {
	const limit = 8192
	n := len(p)
	if len(p) >= limit {
		b.data = append(b.data[:0], p[len(p)-limit:]...)
	} else {
		if len(b.data)+len(p) > limit {
			b.data = b.data[len(b.data)+len(p)-limit:]
		}
		b.data = append(b.data, p...)
	}
	return n, nil
}

type platformCommandFailure struct {
	kind, processStatus string
	exitCode            int
	cause               error
}

func (e *platformCommandFailure) Error() string {
	return fmt.Sprintf("platform command failed (%s; %s)", e.kind, e.processStatus)
}
func (e *platformCommandFailure) Unwrap() error { return e.cause }
func classifyPlatformCommand(ctx context.Context, cmd *exec.Cmd, err error, diagnostic []byte) error {
	failure := &platformCommandFailure{kind: "start-failed", processStatus: "not-started", exitCode: -1}
	if cmd.ProcessState != nil {
		failure.kind = "exit"
		failure.exitCode = cmd.ProcessState.ExitCode()
		failure.processStatus = cmd.ProcessState.String()
	}
	if ctx.Err() != nil {
		failure.cause = ctx.Err()
		failure.kind = "context-cancelled"
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			failure.kind = "context-deadline"
		}
		return failure
	}
	value := strings.ToLower(string(diagnostic))
	switch {
	case strings.Contains(value, "permission denied") || strings.Contains(value, "operation not permitted"):
		failure.kind = "permission"
	case strings.Contains(value, "cannot connect to the docker daemon") || strings.Contains(value, "error during connect") || strings.Contains(value, "dial unix") || strings.Contains(value, "is the docker daemon running"):
		failure.kind = "docker-connection"
	case strings.Contains(value, "unauthorized") || strings.Contains(value, "pull access denied") || strings.Contains(value, "toomanyrequests"):
		failure.kind = "registry-access"
	case strings.Contains(value, "manifest unknown") || strings.Contains(value, "no matching manifest"):
		failure.kind = "registry-manifest"
	case strings.Contains(value, "failed to resolve reference") || strings.Contains(value, "tls handshake timeout") || strings.Contains(value, "unexpected eof") || strings.Contains(value, "network is unreachable"):
		failure.kind = "registry-network"
	case strings.Contains(value, "validating ") || strings.Contains(value, "invalid compose") || strings.Contains(value, "additional property") || strings.Contains(value, "interpolation") || strings.Contains(value, "is not a docker command") || strings.Contains(value, "unknown flag"):
		failure.kind = "compose-configuration"
	case errors.Is(err, exec.ErrWaitDelay):
		failure.kind = "output-timeout"
	}
	return failure
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.Len() {
		return 0, errors.New("command output exceeds limit")
	}
	return b.Buffer.Write(p)
}
func commandRunner(dockerHost string) Runner {
	return func(ctx context.Context, exe string, args []string, dir string, input []byte) ([]byte, error) {
		cmd := exec.CommandContext(ctx, exe, args...)
		hide(cmd)
		cmd.Dir = dir
		cmd.Stdin = bytes.NewReader(input)
		cmd.WaitDelay = 3 * time.Second
		// Keep normal OS/Docker discovery, but never inherit provider/model credentials
		// or repository-controlled GIT_* configuration from the host environment.
		allowed := map[string]bool{"PATH": true, "SYSTEMROOT": true, "WINDIR": true, "SYSTEMDRIVE": true, "COMSPEC": true, "TEMP": true, "TMP": true, "TMPDIR": true, "HOME": true, "USERPROFILE": true, "LOCALAPPDATA": true, "APPDATA": true, "PROGRAMFILES": true, "PROGRAMFILES(X86)": true, "PROGRAMW6432": true, "PROGRAMDATA": true, "ALLUSERSPROFILE": true, "DOCKER_CONFIG": true, "DOCKER_CONTEXT": true, "DOCKER_CERT_PATH": true, "DOCKER_TLS_VERIFY": true}
		for _, v := range os.Environ() {
			key, _, _ := strings.Cut(v, "=")
			if allowed[strings.ToUpper(key)] {
				cmd.Env = append(cmd.Env, v)
			}
		}
		cmd.Env = append(cmd.Env, "GIT_TERMINAL_PROMPT=0", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
		if dockerHost != "" {
			cmd.Env = append(cmd.Env, "DOCKER_HOST="+dockerHost)
		}
		out := &boundedBuffer{limit: 64 << 20}
		diagnostic := &commandDiagnosticBuffer{}
		cmd.Stdout = out
		cmd.Stderr = diagnostic
		if exe == "docker" && len(args) > 0 && args[0] == "logs" {
			cmd.Stderr = out
		}
		if err := cmd.Run(); err != nil {
			return nil, classifyPlatformCommand(ctx, cmd, err, diagnostic.data)
		}
		return out.Bytes(), nil
	}
}
func providerHTTPClient() *http.Client {
	return &http.Client{Timeout: 12 * time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}
