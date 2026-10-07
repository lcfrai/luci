package platform

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/gitcommand"
)

func TestDeploymentArchiveAcceptsNativeGitCommitMetadata(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("native Git is unavailable")
	}
	s := fixture(t, commandRunner(""))
	p, _ := s.opts.ResolveProject("owned")
	git := func(args ...string) []byte {
		t.Helper()
		out, err := s.opts.Runner(context.Background(), "git", gitcommand.PlatformArgs(p.Path, args), p.Path, nil)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	git("-c", "init.defaultBranch=main", "init", "--quiet", "--template=")
	write := func(name, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(p.Path, name)), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(p.Path, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	longName := strings.Repeat("nested/", 20) + "committed.txt"
	for name, content := range map[string]string{
		"Dockerfile": "FROM scratch\n", "app.txt": "first committed revision\n", longName: "safe extended path\n",
		".env": "test-only credential\n", ".luci/state.json": "test-only state\n", "nested/secret.key": "test-only key\n",
	} {
		write(name, content)
	}
	commit := func(message string) string {
		t.Helper()
		git("-c", "core.hooksPath="+os.DevNull, "add", "--all")
		git("-c", "user.name=Luci archive regression", "-c", "user.email=archive@example.invalid", "-c", "commit.gpgsign=false", "-c", "core.hooksPath="+os.DevNull, "commit", "--quiet", "-m", message)
		return strings.TrimSpace(string(git("rev-parse", "HEAD")))
	}
	first := commit("first exact source")
	write("app.txt", "later committed revision\n")
	commit("later source")
	write("app.txt", "uncommitted revision\n")
	raw := git("archive", "--format=tar", first)
	metadata, err := tar.NewReader(bytes.NewReader(raw)).Next()
	if err != nil || metadata.Typeflag != tar.TypeXGlobalHeader || metadata.PAXRecords["comment"] != first {
		t.Fatal("native Git did not produce the commit-bound global PAX header")
	}
	data, err := s.archive(context.Background(), p, first)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	reader := tar.NewReader(bytes.NewReader(data))
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if header.Typeflag == tar.TypeXGlobalHeader {
			t.Fatal("global Git metadata reached the Docker build context")
		}
		if header.Typeflag == tar.TypeReg {
			content, err := io.ReadAll(reader)
			if err != nil {
				t.Fatal(err)
			}
			files[header.Name] = string(content)
		}
	}
	if len(files) != 3 || files["Dockerfile"] != "FROM scratch\n" || files["app.txt"] != "first committed revision\n" || files[longName] != "safe extended path\n" {
		t.Fatal("archive changed the exact committed files or retained private files")
	}
}

func TestDeploymentArchiveRejectsUnsafePAXAndFileEntries(t *testing.T) {
	commit := strings.Repeat("a", 40)
	tests := []struct {
		name     string
		metadata *tar.Header
		file     *tar.Header
	}{
		{"wrong commit", &tar.Header{Name: "pax_global_header", Typeflag: tar.TypeXGlobalHeader, PAXRecords: map[string]string{"comment": strings.Repeat("b", 40)}}, nil},
		{"global path override", &tar.Header{Name: "pax_global_header", Typeflag: tar.TypeXGlobalHeader, PAXRecords: map[string]string{"comment": commit, "path": "../escape"}}, nil},
		{"global credential override", &tar.Header{Name: "pax_global_header", Typeflag: tar.TypeXGlobalHeader, PAXRecords: map[string]string{"comment": commit, "path": ".env"}}, nil},
		{"global size override", &tar.Header{Name: "pax_global_header", Typeflag: tar.TypeXGlobalHeader, PAXRecords: map[string]string{"comment": commit, "size": "33554432"}}, nil},
		{"unexpected metadata name", &tar.Header{Name: "../metadata", Typeflag: tar.TypeXGlobalHeader, PAXRecords: map[string]string{"comment": commit}}, nil},
		{"symlink after safe metadata", nil, &tar.Header{Name: "app.txt", Typeflag: tar.TypeSymlink, Linkname: "../private"}},
		{"hardlink after safe metadata", nil, &tar.Header{Name: "app.txt", Typeflag: tar.TypeLink, Linkname: ".env"}},
		{"extended parent path", nil, &tar.Header{Name: "../" + strings.Repeat("nested/", 20) + "escape", Typeflag: tar.TypeReg, Format: tar.FormatPAX}},
		{"oversized file", nil, &tar.Header{Name: "large.txt", Typeflag: tar.TypeReg, Size: 17 << 20}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var source bytes.Buffer
			writer := tar.NewWriter(&source)
			metadata := test.metadata
			if metadata == nil {
				metadata = &tar.Header{Name: "pax_global_header", Typeflag: tar.TypeXGlobalHeader, PAXRecords: map[string]string{"comment": commit}}
			}
			if err := writer.WriteHeader(metadata); err != nil {
				t.Fatal(err)
			}
			if err := writer.WriteHeader(&tar.Header{Name: "Dockerfile", Typeflag: tar.TypeReg, Mode: 0600}); err != nil {
				t.Fatal(err)
			}
			if test.file != nil {
				if err := writer.WriteHeader(test.file); err != nil {
					t.Fatal(err)
				}
			}
			if test.file == nil || test.file.Size == 0 {
				if err := writer.Close(); err != nil {
					t.Fatal(err)
				}
			}
			s := fixture(t, func(context.Context, string, []string, string, []byte) ([]byte, error) { return source.Bytes(), nil })
			p, _ := s.opts.ResolveProject("owned")
			if _, err := s.archive(context.Background(), p, commit); err == nil {
				t.Fatal("unsafe metadata or file entry entered the build context")
			}
		})
	}
}
