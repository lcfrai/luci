package binding

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/statepath"
)

// ChoicesFileName is the one file this package writes. It holds configuration
// only: no Panacea response, no Hub data, no token (plan 7P-13). No persist
// version is claimed - this is not the desktop's store.
const ChoicesFileName = "bindings.json"

type choiceEntry struct {
	PanaceaProject string `json:"panaceaProject"`
	ChosenAt       string `json:"chosenAt"`
}

type choiceDoc struct {
	Version int                    `json:"version"`
	Choices map[string]choiceEntry `json:"choices"`
}

type choiceFile struct {
	path string
	mu   sync.Mutex
	doc  choiceDoc
}

// choiceKey is (repository key, Panacea address). A choice made against one
// Panacea is never applied to another - the same repository can be deployed by
// a project called "portal" on dev and a different one on prod.
func choiceKey(repoKey, origin string) string { return repoKey + " " + origin }

// loadChoices reads the file at start. A file that does not parse is RENAMED
// and treated as empty, with one warning: the user re-answers one question,
// and nothing crashes at start over a file that holds no irreplaceable fact.
func loadChoices(dir string, log *slog.Logger) (*choiceFile, error) {
	c := &choiceFile{path: filepath.Join(dir, ChoicesFileName), doc: choiceDoc{Version: 1, Choices: map[string]choiceEntry{}}}
	data, err := os.ReadFile(c.path)
	if os.IsNotExist(err) {
		return c, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", c.path, err)
	}
	var doc choiceDoc
	if err := json.Unmarshal(data, &doc); err != nil || doc.Choices == nil {
		aside := c.path + ".unreadable-" + strconv.FormatInt(time.Now().Unix(), 10)
		if rerr := os.Rename(c.path, aside); rerr != nil {
			return nil, fmt.Errorf("set aside %s: %w", c.path, rerr)
		}
		log.Warn("binding: the deploy binding choices file could not be read and was set aside",
			"path", c.path, "movedTo", aside, "err", err)
		return c, nil
	}
	c.doc = doc
	if c.doc.Choices == nil {
		c.doc.Choices = map[string]choiceEntry{}
	}
	return c, nil
}

func (c *choiceFile) get(repoKey, origin string) (deploy.ProjectRef, bool) {
	if repoKey == "" {
		return "", false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.doc.Choices[choiceKey(repoKey, origin)]
	if !ok || e.PanaceaProject == "" {
		return "", false
	}
	return deploy.ProjectRef(e.PanaceaProject), true
}

func (c *choiceFile) set(repoKey, origin string, ref deploy.ProjectRef, now time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.doc.Version = 1
	c.doc.Choices[choiceKey(repoKey, origin)] = choiceEntry{
		PanaceaProject: string(ref),
		ChosenAt:       now.UTC().Format("2006-01-02T15:04:05.000Z07:00"),
	}
	data, err := json.MarshalIndent(c.doc, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0o700); err != nil {
		return err
	}
	return statepath.WriteAtomic(c.path, append(data, '\n'))
}
