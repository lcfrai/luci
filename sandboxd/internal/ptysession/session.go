// Package ptysession runs a command attached to a pseudo-terminal.
// It is deliberately ignorant of what the command is: the same code runs
// Claude Code, Codex, or a bare shell.
package ptysession

import (
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
)

// Session is a running process attached to a PTY.
type Session struct {
	cmd  *exec.Cmd
	file *os.File

	closeOnce sync.Once
	closeErr  error
}

// Start launches cmd with args in a new PTY. If env is nil the parent
// environment is inherited. TERM is forced to xterm-256color so agent CLIs
// emit the colours and box drawing we expect.
func Start(cmd string, args []string, env []string) (*Session, error) {
	c := exec.Command(cmd, args...)
	if env == nil {
		env = os.Environ()
	}
	c.Env = append(env, "TERM=xterm-256color")

	f, err := pty.Start(c)
	if err != nil {
		return nil, err
	}
	return &Session{cmd: c, file: f}, nil
}

// Read returns raw PTY output.
func (s *Session) Read(p []byte) (int, error) { return s.file.Read(p) }

// Write sends raw bytes to the process as if typed.
func (s *Session) Write(p []byte) (int, error) { return s.file.Write(p) }

// Resize informs the process that its terminal dimensions changed.
func (s *Session) Resize(cols, rows uint16) error {
	return pty.Setsize(s.file, &pty.Winsize{Rows: rows, Cols: cols})
}

// Wait blocks until the process exits.
func (s *Session) Wait() error { return s.cmd.Wait() }

// Close kills the process and releases the PTY. Safe to call more than once.
func (s *Session) Close() error {
	s.closeOnce.Do(func() {
		if s.cmd.Process != nil {
			_ = s.cmd.Process.Kill()
		}
		s.closeErr = s.file.Close()
	})
	return s.closeErr
}
