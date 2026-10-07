package management

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/conversationpath"
	"github.com/lcfrai/luci/control-plane/internal/harness"
	"github.com/lcfrai/luci/control-plane/internal/hostpath"
)

const conversationsMarker = ".luci-conversations.json"

var errConversationsActive = errors.New("stop the user's active agent sessions before migrating private conversation storage")

type conversationsBinding struct {
	Version int    `json:"version"`
	UserID  string `json:"userId"`
	Source  string `json:"source"`
}

func prepareWorkerConversations(gatewayDir, workerRoot, userID string) (string, error) {
	legacy := filepath.Join(workerRoot, "state", "conversations")
	if runtime.GOOS != "windows" {
		return legacy, nil
	}
	return migrateWorkerConversations(gatewayDir, workerRoot, userID)
}

// The old tree is never moved, rewritten or removed. An entirely new private
// tree is published by one directory rename, after every file and the binding
// have been copied. A failed attempt is never used by a worker and its staging
// tree is removed. Completed trees never reimport old
// credentials after the CLI has rotated them.
func migrateWorkerConversations(gatewayDir, workerRoot, userID string) (string, error) {
	source := filepath.Join(workerRoot, "state", "conversations")
	target := conversationpath.UserRoot(gatewayDir, userID)
	want := conversationsBinding{Version: 1, UserID: userID, Source: source}
	if err := conversationDirectory(gatewayDir); err != nil {
		return "", err
	}
	if info, err := os.Lstat(target); err == nil {
		if !info.IsDir() || conversationDirectory(target) != nil {
			return "", errors.New("private conversation target is not a regular directory")
		}
		if err = privatePath(target); err != nil {
			return "", err
		}
		marker := filepath.Join(target, conversationsMarker)
		markerInfo, err := os.Lstat(marker)
		if err != nil || !markerInfo.Mode().IsRegular() || markerInfo.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 || markerInfo.Size() > 8192 {
			return "", errors.New("private conversation storage has no regular account binding")
		}
		if err = hostpath.OneName(marker); err != nil {
			return "", err
		}
		data, err := os.ReadFile(marker)
		var binding conversationsBinding
		if err != nil || len(data) > 8192 || json.Unmarshal(data, &binding) != nil || binding != want {
			return "", errors.New("private conversation storage has no matching account binding")
		}
		return target, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := legacyConversationsStopped(workerRoot); err != nil {
		return "", err
	}
	parent := filepath.Dir(target)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return "", err
	}
	if err := conversationDirectory(parent); err != nil {
		return "", err
	}
	if err := privatePath(parent); err != nil {
		return "", err
	}
	stage, err := os.MkdirTemp(parent, ".m-")
	if err != nil {
		return "", err
	}
	// Staging paths are generated under the checked private parent. A failed
	// copy must not leave a second obsolete credential tree behind.
	defer func() {
		if root, openErr := os.OpenRoot(parent); openErr == nil {
			_ = root.RemoveAll(filepath.Base(stage))
			_ = root.Close()
		}
	}()
	if err = privatePath(stage); err != nil {
		return "", err
	}
	if err = copyWorkerConversations(source, stage); err != nil {
		return "", err
	}
	// Recheck immediately before publication. The gateway has not started this
	// account's worker, so only a surviving old sandbox could still own a home.
	if err = legacyConversationsStopped(workerRoot); err != nil {
		return "", err
	}
	data, _ := json.Marshal(want)
	if err = atomicPrivate(filepath.Join(stage, conversationsMarker), data); err != nil {
		return "", err
	}
	if err = os.Rename(stage, target); err != nil {
		return "", err
	}
	return target, nil
}

func conversationDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
		return errors.New("private conversation path is not a regular directory")
	}
	// Unix directories normally have several links. Windows directories have
	// one name; reject an unexpected identity before securing its DACL.
	if runtime.GOOS == "windows" {
		return hostpath.OneName(path)
	}
	return nil
}

func legacyConversationsStopped(workerRoot string) error {
	path := filepath.Join(workerRoot, "state", "sessions", "sessions.json")
	if err := hostpath.OneName(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (16<<20)+1))
	if err != nil || len(data) > 16<<20 {
		return errors.New("private session registry could not be checked before migration")
	}
	var doc struct {
		SchemaVersion int `json:"schemaVersion"`
		Sessions      []struct {
			Status string `json:"status"`
		} `json:"sessions"`
	}
	if json.Unmarshal(data, &doc) != nil || doc.SchemaVersion != 1 {
		return errors.New("private session registry could not be checked before migration")
	}
	for _, session := range doc.Sessions {
		if session.Status != "exited" {
			return errConversationsActive
		}
	}
	return nil
}

var conversationSegment = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)

func migratedConversationName(name string) (string, error) {
	parts := strings.Split(filepath.ToSlash(name), "/")
	if parts[0] != "native" {
		if name == conversationsMarker {
			return "", errors.New("legacy storage contains a reserved binding")
		}
		return name, nil
	}
	if len(parts) < 5 {
		return "", nil // Container directories have no files of their own.
	}
	d, ok := harness.Lookup(harness.ID(parts[3]))
	if parts[1] != "local" || !conversationSegment.MatchString(parts[2]) || !ok || !d.NativeState() || !harness.IsConversationID(parts[4]) {
		return "", errors.New("legacy provider home has an invalid identity")
	}
	rel := conversationpath.NativeRelative(parts[1], parts[2], parts[3], parts[4])
	if len(parts) > 5 {
		rel = filepath.Join(append([]string{rel}, parts[5:]...)...)
	}
	return rel, nil
}

func copyWorkerConversations(source, target string) error {
	info, err := os.Lstat(source)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || conversationDirectory(source) != nil {
		return errors.New("legacy conversation root is not a regular directory")
	}
	root, err := os.OpenRoot(source)
	if err != nil {
		return err
	}
	defer root.Close()
	entries, total := 0, int64(0)
	return fs.WalkDir(root.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		entries++
		if entries > 65536 {
			return errors.New("private conversation migration exceeds its entry limit")
		}
		info, err := root.Lstat(name)
		if err != nil {
			return err
		}
		if info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 || !info.IsDir() && !info.Mode().IsRegular() {
			return errors.New("legacy conversations contain a link or special file")
		}
		if name == "." {
			return nil
		}
		rel, err := migratedConversationName(name)
		if err != nil {
			return err
		}
		if rel == "" {
			if !info.IsDir() {
				return errors.New("legacy provider container contains an unexpected file")
			}
			return nil
		}
		dest := filepath.Join(target, rel)
		if info.IsDir() {
			if err = os.MkdirAll(dest, 0700); err != nil {
				return err
			}
			return privatePath(dest)
		}
		total += info.Size()
		if info.Size() > 1<<30 || total > 8<<30 {
			return errors.New("private conversation migration exceeds its size limit")
		}
		if err = hostpath.OneName(filepath.Join(source, name)); err != nil {
			return err
		}
		in, err := root.Open(name)
		if err != nil {
			return err
		}
		if err = os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
			in.Close()
			return err
		}
		out, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			in.Close()
			return err
		}
		if err = privatePath(dest); err != nil {
			in.Close()
			out.Close()
			return err
		}
		n, copyErr := io.Copy(out, io.LimitReader(in, info.Size()+1))
		in.Close()
		if copyErr == nil {
			copyErr = out.Sync()
		}
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if n != info.Size() {
			return errors.New("legacy conversation data changed during migration")
		}
		return nil
	})
}
