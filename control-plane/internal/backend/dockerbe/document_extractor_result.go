package dockerbe

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/lcfrai/luci/control-plane/internal/backend"
)

type documentArchiveClient interface {
	CopyFromContainer(context.Context, string, string) (io.ReadCloser, container.PathStat, error)
}

// This is data ingestion, never tar extraction onto the host. Both advertised
// stat and actual single regular entry must be complete and independently bounded.
func readDocumentResult(ctx context.Context, reader io.Reader, stat container.PathStat) (backend.DocumentExtractionOutput, error) {
	if stat.Name != "result.json" || !stat.Mode.IsRegular() || stat.Size < 1 || stat.Size > backend.DocumentResultMaxBytes || stat.LinkTarget != "" {
		return backend.DocumentExtractionOutput{}, backend.ErrUnsupported
	}
	limited := &io.LimitedReader{R: reader, N: backend.DocumentResultMaxBytes + (16 << 10)}
	archive := tar.NewReader(limited)
	header, err := archive.Next()
	if err != nil || header.Name != "result.json" || header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA || header.Linkname != "" || header.Size != stat.Size {
		return backend.DocumentExtractionOutput{}, backend.ErrUnavailable
	}
	data, err := io.ReadAll(io.LimitReader(archive, backend.DocumentResultMaxBytes+1))
	if err != nil || int64(len(data)) != stat.Size || ctx.Err() != nil {
		return backend.DocumentExtractionOutput{}, backend.ErrUnavailable
	}
	if _, err = archive.Next(); err != io.EOF {
		return backend.DocumentExtractionOutput{}, backend.ErrUnavailable
	}
	// The finite trailing archive blocks must contain only padding. A saturated
	// response, unexpected second file or tail cannot masquerade as a complete result.
	padding, err := io.ReadAll(limited)
	if err != nil || limited.N == 0 || ctx.Err() != nil {
		return backend.DocumentExtractionOutput{}, backend.ErrUnavailable
	}
	for _, value := range padding {
		if value != 0 {
			return backend.DocumentExtractionOutput{}, backend.ErrUnavailable
		}
	}
	digest := sha256.Sum256(data)
	return backend.DocumentExtractionOutput{Data: data, Digest: hex.EncodeToString(digest[:])}, nil
}
func (b *DocumentExtractor) DocumentResult(ctx context.Context, identity backend.DocumentExtractionIdentity, ref string) (backend.DocumentExtractionOutput, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	row, absent, err := b.inspectDocument(ctx, identity, ref, true)
	if err != nil {
		return backend.DocumentExtractionOutput{}, err
	}
	if absent || row.State.Status != "exited" || row.State.Running || row.State.Pid != 0 || row.State.OOMKilled {
		return backend.DocumentExtractionOutput{}, backend.ErrNotRunning
	}
	client, ok := b.api.(documentArchiveClient)
	if !ok {
		return backend.DocumentExtractionOutput{}, backend.ErrUnsupported
	}
	reader, stat, err := client.CopyFromContainer(ctx, ref, documentResultPath)
	if err != nil || reader == nil {
		return backend.DocumentExtractionOutput{}, backend.ErrUnavailable
	}
	defer reader.Close()
	stop := context.AfterFunc(ctx, func() { _ = reader.Close() })
	defer stop()
	result, err := readDocumentResult(ctx, reader, stat)
	if err != nil {
		return result, err
	}
	fresh, absent, err := b.inspectDocument(ctx, identity, ref, true)
	if err != nil || absent || fresh.State.Status != "exited" || fresh.State.Running || fresh.State.Pid != 0 || fresh.State.OOMKilled {
		return backend.DocumentExtractionOutput{}, backend.ErrUnavailable
	}
	return result, nil
}
