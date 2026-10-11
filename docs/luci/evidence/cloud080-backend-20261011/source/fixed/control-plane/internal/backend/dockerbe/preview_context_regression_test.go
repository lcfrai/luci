package dockerbe

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

// This reader never reaches EOF, as a file could continue growing after Stat.
type previewGrowingReader struct{ consumed int }

func (r *previewGrowingReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'x'
	}
	r.consumed += len(p)
	return len(p), nil
}

func TestPreviewContextReadBoundsGrowth(t *testing.T) {
	const limit = 1024
	r := &previewGrowingReader{}
	got, err := readPreviewContextFile(r, limit)
	if !errors.Is(err, backend.ErrUnsupported) || got != nil {
		t.Fatalf("growing input must be rejected without retained bytes: len=%d err=%v", len(got), err)
	}
	if r.consumed != limit+1 {
		t.Fatalf("read %d bytes; expected only the limit and one overflow witness", r.consumed)
	}
	exact := bytes.Repeat([]byte{'x'}, limit)
	got, err = readPreviewContextFile(bytes.NewReader(exact), limit)
	if err != nil || !bytes.Equal(got, exact) {
		t.Fatalf("exact-limit input rejected: len=%d err=%v", len(got), err)
	}
	if _, err = readPreviewContextFile(strings.NewReader("x"), 0); !errors.Is(err, backend.ErrUnsupported) {
		t.Fatalf("zero remaining budget accepted data: %v", err)
	}
}

func TestPreviewContextRejectsOversizeSparseFile(t *testing.T) {
	_, _, spec := newPreviewProbe(t)
	f, err := os.Create(filepath.Join(spec.SourceDirectory, "oversize"))
	if err != nil {
		t.Fatal(err)
	}
	err = f.Truncate(maxPreviewContext + 1)
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("create sparse fixture: truncate=%v close=%v", err, closeErr)
	}
	files, _, err := previewContext(spec)
	if !errors.Is(err, backend.ErrUnsupported) || files != nil {
		t.Fatalf("oversize context accepted: files=%d err=%v", len(files), err)
	}
}

func TestPreviewContextRejectsDiskEstimateOverflow(t *testing.T) {
	for _, tc := range []struct {
		name   string
		size   int64
		stages int
	}{
		{"reserve-addition", math.MaxInt64, 1},
		{"base-multiplication", math.MaxInt64, 2},
		{"base-addition", math.MaxInt64, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, probe, spec := newPreviewProbe(t)
			base := probe.images["node:fixture"]
			base.Size = tc.size
			probe.images["node:fixture"] = base
			if err := os.WriteFile(filepath.Join(spec.SourceDirectory, "Dockerfile"), []byte(strings.Repeat("FROM node:fixture\n", tc.stages)), 0600); err != nil {
				t.Fatal(err)
			}
			prepared, _, _, demand, err := b.preparePreviewContext(context.Background(), spec)
			if !errors.Is(err, backend.ErrUnsupported) || prepared != nil || demand != 0 {
				t.Fatalf("unrepresentable estimate accepted: bytes=%d demand=%d err=%v", len(prepared), demand, err)
			}
		})
	}
}

func TestPreviewContextAllowsOrdinaryLuciMentions(t *testing.T) {
	b, _, spec := newPreviewProbe(t)
	dockerfile := "FROM node:fixture\n# luci.preview is an ownership label\nRUN echo luci.example\nLABEL description=\"mentions luci.preview\" other=\"ordinary\"\nLABEL legacy \"also mentions luci.preview\"\nRUN echo continued \\\n  luci.example\n"
	if err := os.WriteFile(filepath.Join(spec.SourceDirectory, "Dockerfile"), []byte(dockerfile), 0600); err != nil {
		t.Fatal(err)
	}
	prepared, _, _, _, err := b.preparePreviewContext(context.Background(), spec)
	if err != nil {
		t.Fatalf("ordinary text rejected: %v", err)
	}
	r := tarDockerfile(t, prepared)
	for _, text := range []string{"RUN echo luci.example", "description=\"mentions luci.preview\"", "LABEL legacy \"also mentions luci.preview\""} {
		if !strings.Contains(r, text) {
			t.Fatalf("prepared Dockerfile lost ordinary instruction %q", text)
		}
	}
}

func TestPreviewContextRejectsReservedLabelKeys(t *testing.T) {
	for _, instruction := range []string{
		"LABEL luci.preview=other\n",
		"LABEL LuCi.preview=other\n",
		"LABEL luci.preview other\n",
		"LABEL \"luci.preview\"=other\n",
		"LABEL 'luci.preview'=other\n",
		"LABEL luci\\.preview=other\n",
		"LABEL ordinary=value \\\n  luci.preview=other\n",
		"LABEL ordinary=value \\\n  # continuation comment\n  luci.preview=other\n",
		"LA\\\n  BEL luci.preview=other\n",
		"LABEL lu\\\n  ci.preview=other\n",
		"ARG KEY=luci.preview\nLABEL $KEY=other\n",
	} {
		t.Run(strings.ReplaceAll(instruction, "\n", "|"), func(t *testing.T) {
			b, _, spec := newPreviewProbe(t)
			if err := os.WriteFile(filepath.Join(spec.SourceDirectory, "Dockerfile"), []byte("FROM node:fixture\n"+instruction), 0600); err != nil {
				t.Fatal(err)
			}
			if _, _, _, _, err := b.preparePreviewContext(context.Background(), spec); !errors.Is(err, backend.ErrUnsupported) {
				t.Fatalf("reserved or dynamically determined label key accepted: %v", err)
			}
		})
	}
}

func tarDockerfile(t *testing.T, data []byte) string {
	t.Helper()
	// Reuse the ingestion path to inspect the emitted build context.
	files, _, err := previewContext(backend.PreviewSpec{SourceArchive: data})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if f.name == "Dockerfile" {
			return string(f.data)
		}
	}
	t.Fatal("prepared context has no Dockerfile")
	return ""
}

var _ io.Reader = (*previewGrowingReader)(nil)
