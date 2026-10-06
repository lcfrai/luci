// Package diagnostics retains bounded, classified API failures for the host
// operator. It never stores error text, request data or caller-chosen paths.
package diagnostics

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
)

const (
	MaxEvents = 128
	MaxBytes  = 64 << 10
	Directory = "diagnostics"
	Filename  = "api-failures.json"
)

type event struct {
	At        string `json:"at"`
	Operation string `json:"operation"`
	Cause     string `json:"cause"`
}

type document struct {
	Version int     `json:"version"`
	Events  []event `json:"events"`
}

// Recorder belongs to one worker. Its directory is outside projects and native
// provider homes, and is never mounted in a sandbox or exposed by an API.
type Recorder struct {
	mu       sync.Mutex
	root     *os.Root
	dir      string
	identity os.FileInfo
	events   []event
}

// New opens only the fixed diagnostics child of an existing private worker
// state directory. Invalid retained data is preserved and refused, not repaired.
func New(stateDir string) (*Recorder, error) {
	if !filepath.IsAbs(stateDir) {
		return nil, errors.New("diagnostics requires an absolute worker state directory")
	}
	if err := directories(stateDir); err != nil {
		return nil, err
	}
	state, err := os.OpenRoot(stateDir)
	if err != nil {
		return nil, err
	}
	defer state.Close()
	if err = state.Mkdir(Directory, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, err
	}
	dir := filepath.Join(stateDir, Directory)
	if err = directories(dir); err != nil {
		return nil, err
	}
	identity, err := state.Lstat(Directory)
	if err != nil {
		return nil, err
	}
	if err = securePrivate(dir, true); err != nil {
		return nil, err
	}
	root, err := state.OpenRoot(Directory)
	if err != nil {
		return nil, err
	}
	r := &Recorder{root: root, dir: dir, identity: identity}
	if err = r.load(); err != nil {
		root.Close()
		return nil, err
	}
	return r, nil
}

func directories(path string) error {
	for {
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return errors.New("diagnostics path is not a regular directory")
		}
		if runtime.GOOS == "windows" {
			if err := hostpath.OneName(path); err != nil {
				return err
			}
		}
		parent := filepath.Dir(path)
		if parent == path {
			return nil
		}
		path = parent
	}
}

func (r *Recorder) validate() error {
	if r.root == nil {
		return errors.New("diagnostics recorder is closed")
	}
	if err := directories(r.dir); err != nil {
		return err
	}
	info, err := os.Lstat(r.dir)
	if err != nil || !os.SameFile(info, r.identity) {
		return errors.New("diagnostics directory changed")
	}
	info, err = r.root.Lstat(Filename)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > MaxBytes {
		return errors.New("diagnostics file is not a bounded regular file")
	}
	return hostpath.OneName(filepath.Join(r.dir, Filename))
}

func (r *Recorder) load() error {
	if err := r.validate(); err != nil {
		return err
	}
	f, err := r.root.Open(Filename)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	before, err := r.root.Lstat(Filename)
	if err != nil {
		return err
	}
	opened, err := f.Stat()
	if err != nil || !os.SameFile(before, opened) || !opened.Mode().IsRegular() {
		return errors.New("diagnostics file changed while opening")
	}
	if err = securePrivate(filepath.Join(r.dir, Filename), false); err != nil {
		return err
	}
	data, err := io.ReadAll(io.LimitReader(f, MaxBytes+1))
	if err != nil {
		return err
	}
	if len(data) > MaxBytes {
		return errors.New("diagnostics file exceeds its byte limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var saved document
	if decoder.Decode(&saved) != nil || decoder.Decode(new(any)) != io.EOF || saved.Version != 1 || len(saved.Events) > MaxEvents {
		return errors.New("diagnostics file is invalid")
	}
	for _, row := range saved.Events {
		if _, err := time.Parse(time.RFC3339Nano, row.At); err != nil || !validOperation(row.Operation) || !validCause(row.Cause) {
			return errors.New("diagnostics event is invalid")
		}
	}
	r.events = saved.Events
	return nil
}

// Record accepts internal operation labels and errors only to classify them.
// The resulting file contains finite codes; neither original string is saved.
// A recording error never changes the API failure being reported to the user.
func (r *Recorder) Record(operation string, cause error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.validate(); err != nil {
		return err
	}
	next := append(append([]event{}, r.events...), event{At: time.Now().UTC().Format(time.RFC3339Nano), Operation: operationCode(operation), Cause: causeCode(cause)})
	if len(next) > MaxEvents {
		next = next[len(next)-MaxEvents:]
	}
	var data []byte
	for {
		var err error
		data, err = json.Marshal(document{Version: 1, Events: next})
		if err != nil {
			return err
		}
		if len(data)+1 <= MaxBytes {
			break
		}
		if len(next) <= 1 {
			return errors.New("diagnostics event exceeds its byte limit")
		}
		next = next[1:]
	}
	if err := r.replace(append(data, '\n')); err != nil {
		return err
	}
	r.events = next
	return nil
}

func (r *Recorder) replace(data []byte) error {
	var suffix [16]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return err
	}
	name := ".failure-" + hex.EncodeToString(suffix[:])
	f, err := r.root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer r.root.Remove(name)
	if err = securePrivate(filepath.Join(r.dir, name), false); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = r.validate(); err != nil {
		return err
	}
	// Match the registry's bounded retry for Windows indexers holding a file.
	for attempt := 0; attempt < 5; attempt++ {
		if err = r.root.Rename(name, Filename); err == nil {
			return nil
		}
		if runtime.GOOS != "windows" || attempt == 4 {
			break
		}
		time.Sleep(time.Duration(20*(attempt+1)) * time.Millisecond)
	}
	return err
}

func (r *Recorder) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.root == nil {
		return nil
	}
	err := r.root.Close()
	r.root = nil
	return err
}

var operations = map[string]string{
	"prepare private provider state":                 "native_state_prepare",
	"find the conversation of the session to resume": "native_conversation_find",
	"read the session to resume":                     "session_resume_read",
	"capture the reviewed session context":           "reviewed_context_capture",
	"delete the session's transcript":                "session_transcript_forget",
	"delete the session's reviewed context":          "reviewed_context_forget",
	"purge the session record":                       "session_record_forget",
	"create the sandbox":                             "sandbox_create",
	"destroy the sandbox":                            "sandbox_destroy",
	"record the kill":                                "session_stop_record",
}

func operationCode(value string) string {
	if code, ok := operations[value]; ok {
		return code
	}
	return "unclassified"
}

func validOperation(value string) bool {
	if value == "unclassified" {
		return true
	}
	for _, code := range operations {
		if value == code {
			return true
		}
	}
	return false
}

var causes = map[string]bool{
	"unclassified": true, "not_exist": true, "permission": true, "no_space": true,
	"context_canceled": true, "deadline": true, "hardlink": true,
	"native_state_link": true, "native_state_special": true, "native_state_hardlink": true,
	"native_state_limit": true, "native_state_changed": true, "native_state_io": true,
}

func validCause(value string) bool { return causes[value] }

func causeCode(err error) string {
	var classified interface{ DiagnosticCode() string }
	if errors.As(err, &classified) {
		if code := classified.DiagnosticCode(); validCause(code) {
			return code
		}
	}
	for _, known := range []struct {
		err  error
		code string
	}{
		{hostpath.ErrHardLinked, "hardlink"}, {os.ErrNotExist, "not_exist"},
		{os.ErrPermission, "permission"}, {syscall.ENOSPC, "no_space"},
		{context.Canceled, "context_canceled"}, {context.DeadlineExceeded, "deadline"},
	} {
		if errors.Is(err, known.err) {
			return known.code
		}
	}
	return "unclassified"
}
