package product

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/platform"
)

const functionProjectRoot = "supabase/functions"

type functionSourceConfiguration struct {
	Version  int    `json:"version"`
	AuthMode string `json:"authMode"`
}

func (s *Server) FunctionSources() *platform.DataFunctionSources {
	return &platform.DataFunctionSources{
		List: func(ctx context.Context, id string) ([]platform.DataFunctionSource, error) {
			release, err := s.AcquireGitSessionAdmission(ctx, id)
			if err != nil {
				return nil, err
			}
			defer release()
			return s.listFunctionSource(ctx, id)
		},
		Read: func(ctx context.Context, id, name string) (platform.DataFunctionSource, error) {
			release, err := s.AcquireGitSessionAdmission(ctx, id)
			if err != nil {
				return platform.DataFunctionSource{}, err
			}
			defer release()
			return s.readFunctionSource(ctx, id, name)
		},
		Save: func(ctx context.Context, id, name string, input platform.DataFunctionSave) (platform.DataFunctionSource, error) {
			release, err := s.AcquireGitSessionAdmission(ctx, id)
			if err != nil {
				return platform.DataFunctionSource{}, err
			}
			defer release()
			return s.saveFunctionSource(ctx, id, name, input)
		},
		Committed: func(ctx context.Context, id, name, commit string) (platform.DataFunctionSource, error) {
			release, err := s.AcquireGitSessionAdmission(ctx, id)
			if err != nil {
				return platform.DataFunctionSource{}, err
			}
			defer release()
			return s.committedFunctionSource(ctx, id, name, commit)
		},
	}
}
func functionRelative(name string) (string, error) {
	if !platform.ValidDataFunctionName(name) {
		return "", errors.New("invalid project function name")
	}
	return functionProjectRoot + "/" + name, nil
}
func functionConfiguration(data []byte) (string, error) {
	var v functionSourceConfiguration
	d := json.NewDecoder(strings.NewReader(string(data)))
	d.DisallowUnknownFields()
	if d.Decode(&v) != nil || d.Decode(new(any)) != io.EOF || v.Version != 1 || !platform.ValidDataFunctionAuth(v.AuthMode) {
		return "", errors.New("unsupported function configuration")
	}
	return v.AuthMode, nil
}
func (s *Server) listFunctionSource(ctx context.Context, id string) ([]platform.DataFunctionSource, error) {
	p, e := s.resolveProject(id)
	if e != nil {
		return nil, e
	}
	root := filepath.Join(p.Path, filepath.FromSlash(functionProjectRoot))
	if _, e = os.Lstat(root); errors.Is(e, os.ErrNotExist) {
		return []platform.DataFunctionSource{}, nil
	}
	if _, e = guardedProjectPath(p.Path, functionProjectRoot, false); e != nil {
		return nil, e
	}
	f, e := os.Open(root)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	entries, e := f.ReadDir(65)
	if e != nil && e != io.EOF {
		return nil, e
	}
	if len(entries) > 64 {
		return nil, errors.New("too many project functions")
	}
	out := []platform.DataFunctionSource{}
	for _, entry := range entries {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if !platform.ValidDataFunctionName(entry.Name()) {
			continue
		}
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return nil, errors.New("function directory must be ordinary")
		}
		v, e := s.readFunctionSource(ctx, id, entry.Name())
		if e != nil {
			return nil, e
		}
		v.Files = nil
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
func (s *Server) functionWorkingSource(ctx context.Context, id, name string) (platform.DataFunctionSource, error) {
	relative, e := functionRelative(name)
	out := platform.DataFunctionSource{Name: name, Path: relative, AuthMode: "authenticated", Files: []platform.DataFunctionFile{}, WorkingChanged: true}
	if e != nil {
		return out, e
	}
	p, e := s.resolveProject(id)
	if e != nil {
		return out, e
	}
	dir, e := guardedProjectPath(p.Path, relative, false)
	if e != nil {
		return out, e
	}
	count, total := 0, 0
	e = filepath.WalkDir(dir, func(file string, entry fs.DirEntry, walk error) error {
		if walk != nil {
			return walk
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		count++
		if count > 96 {
			return errors.New("function inventory exceeds limit")
		}
		rel, e := filepath.Rel(dir, file)
		if e != nil {
			return e
		}
		if rel == "." {
			return nil
		}
		portable := filepath.ToSlash(rel)
		projectPath := relative + "/" + portable
		if _, e = guardedProjectPath(p.Path, projectPath, false); e != nil {
			return e
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() || hostpath.OneName(file) != nil {
			return errors.New("function files must have one ordinary name")
		}
		if portable != "luci.function.json" && !platform.ValidDataFunctionPath(portable) {
			return errors.New("unsupported function source file")
		}
		before, e := os.Lstat(file)
		if e != nil || before.Size() > platform.DataFunctionMaxFileBytes {
			return errors.New("function source too large")
		}
		f, e := os.Open(file)
		if e != nil {
			return e
		}
		info, e := f.Stat()
		if e != nil || !os.SameFile(before, info) {
			f.Close()
			return errors.New("function source changed")
		}
		b, e := io.ReadAll(io.LimitReader(f, platform.DataFunctionMaxFileBytes+1))
		after, se := f.Stat()
		f.Close()
		if e != nil || se != nil || len(b) > platform.DataFunctionMaxFileBytes || int64(len(b)) != before.Size() || after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) {
			return errors.New("function source changed")
		}
		total += len(b)
		if total > platform.DataFunctionMaxBytes {
			return errors.New("function source exceeds limit")
		}
		if portable == "luci.function.json" {
			out.AuthMode, e = functionConfiguration(b)
			return e
		}
		out.Files = append(out.Files, platform.DataFunctionFile{Path: portable, Content: string(b)})
		return nil
	})
	if e != nil {
		return out, e
	}
	if e = platform.ValidateDataFunctionFiles(out.Files); e != nil {
		return out, e
	}
	sort.Slice(out.Files, func(i, j int) bool { return out.Files[i].Path < out.Files[j].Path })
	out.SourceDigest = platform.DataFunctionDigest(out.AuthMode, out.Files)
	return out, nil
}
func (s *Server) readFunctionSource(ctx context.Context, id, name string) (platform.DataFunctionSource, error) {
	s.functionSourceMu.RLock()
	out, e := s.functionWorkingSource(ctx, id, name)
	s.functionSourceMu.RUnlock()
	if e != nil {
		return out, e
	}
	p, e := s.resolveProject(id)
	if e != nil {
		return out, e
	}
	head, e := s.journeyGit(ctx, p.Path, "rev-parse", "--verify", "HEAD")
	if e == nil && deliverySHA.MatchString(strings.TrimSpace(head)) {
		out.Commit = strings.TrimSpace(head)
		committed, e := s.functionCommitBytes(ctx, id, name, out.Commit)
		if e == nil {
			out.CommittedDigest = committed.SourceDigest
			out.WorkingChanged = out.SourceDigest != committed.SourceDigest
		}
	}
	return out, nil
}
func ensureFunctionDirectory(root, relative string) error {
	current := ""
	for _, part := range strings.Split(relative, "/") {
		if current != "" {
			current += "/"
		}
		current += part
		target := filepath.Join(root, filepath.FromSlash(current))
		if _, e := os.Lstat(target); errors.Is(e, os.ErrNotExist) {
			if _, e = guardedProjectPath(root, current, true); e != nil {
				return e
			}
			if e = os.Mkdir(target, 0700); e != nil {
				return e
			}
		} else if e != nil {
			return e
		}
		if _, e := guardedProjectPath(root, current, false); e != nil {
			return e
		}
	}
	return nil
}
func (s *Server) saveFunctionSource(ctx context.Context, id, name string, in platform.DataFunctionSave) (platform.DataFunctionSource, error) {
	relative, e := functionRelative(name)
	if e != nil || !platform.ValidDataFunctionAuth(in.AuthMode) || len(in.Files) == 0 || len(in.Files) > platform.DataFunctionMaxFiles {
		return platform.DataFunctionSource{}, errors.New("invalid function edit")
	}
	// Validate a patch independently; the merged bundle must retain index.ts.
	patch := map[string]platform.DataFunctionFile{}
	total := 0
	for _, f := range in.Files {
		if !platform.ValidDataFunctionPath(f.Path) || patch[strings.ToLower(f.Path)].Path != "" || len(f.Content) > platform.DataFunctionMaxFileBytes {
			return platform.DataFunctionSource{}, errors.New("unsupported function edit")
		}
		patch[strings.ToLower(f.Path)] = f
		total += len(f.Content)
	}
	if total > platform.DataFunctionMaxBytes {
		return platform.DataFunctionSource{}, errors.New("function edit exceeds limit")
	}
	p, e := s.resolveProject(id)
	if e != nil {
		return platform.DataFunctionSource{}, e
	}
	s.functionSourceMu.Lock()
	current, e := s.functionWorkingSource(ctx, id, name)
	newFunction := false
	if e != nil {
		target := filepath.Join(p.Path, filepath.FromSlash(relative))
		if _, missing := os.Lstat(target); errors.Is(missing, os.ErrNotExist) && in.ExpectedDigest == "" {
			newFunction = true
			current = platform.DataFunctionSource{Files: []platform.DataFunctionFile{}}
		} else {
			s.functionSourceMu.Unlock()
			return current, e
		}
	}
	if current.SourceDigest != in.ExpectedDigest {
		s.functionSourceMu.Unlock()
		return current, errors.New("function source changed")
	}
	merged := map[string]platform.DataFunctionFile{}
	for _, f := range current.Files {
		merged[f.Path] = f
	}
	for _, f := range in.Files {
		merged[f.Path] = f
	}
	files := []platform.DataFunctionFile{}
	for _, f := range merged {
		files = append(files, f)
	}
	if e = platform.ValidateDataFunctionFiles(files); e != nil {
		s.functionSourceMu.Unlock()
		return current, e
	}
	if e = ensureFunctionDirectory(p.Path, relative); e != nil {
		s.functionSourceMu.Unlock()
		return current, e
	}
	if !newFunction {
		again, e := s.functionWorkingSource(ctx, id, name)
		if e != nil || again.SourceDigest != in.ExpectedDigest {
			s.functionSourceMu.Unlock()
			return current, errors.New("function source changed before save")
		}
	}
	for _, f := range in.Files {
		if ctx.Err() != nil {
			s.functionSourceMu.Unlock()
			return current, ctx.Err()
		}
		parent := filepath.ToSlash(filepath.Dir(f.Path))
		if parent != "." {
			if e = ensureFunctionDirectory(p.Path, relative+"/"+parent); e != nil {
				s.functionSourceMu.Unlock()
				return current, e
			}
		}
		if e = writeProjectAtomic(p.Path, relative+"/"+f.Path, []byte(f.Content)); e != nil {
			s.functionSourceMu.Unlock()
			return current, e
		}
	}
	b, _ := json.Marshal(functionSourceConfiguration{Version: 1, AuthMode: in.AuthMode})
	e = writeProjectAtomic(p.Path, relative+"/luci.function.json", b)
	s.functionSourceMu.Unlock()
	if e != nil {
		return current, e
	}
	return s.readFunctionSource(ctx, id, name)
}
func (s *Server) functionCommitBytes(ctx context.Context, id, name, commit string) (platform.DataFunctionSource, error) {
	relative, e := functionRelative(name)
	out := platform.DataFunctionSource{Name: name, Path: relative, Commit: commit, AuthMode: "authenticated", Files: []platform.DataFunctionFile{}}
	if e != nil || !deliverySHA.MatchString(commit) {
		return out, errors.New("invalid function commit")
	}
	p, e := s.resolveProject(id)
	if e != nil {
		return out, e
	}
	tree, e := s.journeyGit(ctx, p.Path, "ls-tree", "-r", "-l", "-z", commit, "--", relative+"/")
	if e != nil || len(tree) > 32<<10 {
		return out, errors.New("function inventory unavailable")
	}
	total := 0
	for _, line := range strings.Split(tree, "\x00") {
		if line == "" {
			continue
		}
		meta, file, ok := strings.Cut(line, "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 4 || fields[0] != "100644" && fields[0] != "100755" || fields[1] != "blob" || !strings.HasPrefix(file, relative+"/") {
			return out, errors.New("unsupported function committed path")
		}
		name := strings.TrimPrefix(file, relative+"/")
		if name != "luci.function.json" && !platform.ValidDataFunctionPath(name) {
			return out, errors.New("private or unsupported function file")
		}
		size, e := strconv.Atoi(fields[3])
		if e != nil || size < 0 || size > platform.DataFunctionMaxFileBytes || total+size > platform.DataFunctionMaxBytes || (name != "luci.function.json" && len(out.Files) >= platform.DataFunctionMaxFiles) {
			return out, errors.New("committed function exceeds limit")
		}
		b, e := s.journeyGit(ctx, p.Path, "cat-file", "blob", fields[2])
		if e != nil || len(b) != size {
			return out, errors.New("committed function changed")
		}
		total += len(b)
		if name == "luci.function.json" {
			out.AuthMode, e = functionConfiguration([]byte(b))
			if e != nil {
				return out, e
			}
		} else {
			out.Files = append(out.Files, platform.DataFunctionFile{Path: name, Content: b})
		}
	}
	if e = platform.ValidateDataFunctionFiles(out.Files); e != nil {
		return out, e
	}
	out.SourceDigest = platform.DataFunctionDigest(out.AuthMode, out.Files)
	out.CommittedDigest = out.SourceDigest
	return out, nil
}
func (s *Server) committedFunctionSource(ctx context.Context, id, name, commit string) (platform.DataFunctionSource, error) {
	if e := s.ValidateDeploymentSource(ctx, id, commit); e != nil {
		return platform.DataFunctionSource{}, e
	}
	return s.functionCommitBytes(ctx, id, name, commit)
}
