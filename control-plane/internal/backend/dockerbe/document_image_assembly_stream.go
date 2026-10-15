package dockerbe

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/lcfrai/luci/control-plane/internal/backend"
)

const assemblyResultMagic = "LUCI-DOCUMENT-OCI/1\n"
const assemblyResultEnd = "\nLUCI-DOCUMENT-OCI-END/1\n"

type assemblyAttachOwner struct {
	mu          sync.Mutex
	identity    backend.DocumentImageAssemblyIdentity
	connections []net.Conn
	closed      bool
}

func (o *assemblyAttachOwner) Close() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.closed = true
	remaining := []net.Conn{}
	for _, conn := range o.connections {
		if err := conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			remaining = append(remaining, conn)
		}
	}
	o.connections = remaining
	if len(remaining) != 0 {
		return backend.ErrUnavailable
	}
	return nil
}

func (stream *assemblyResultStream) Capture(ctx context.Context, sink io.Writer) (out backend.DocumentImageAssemblyCapture, err error) {
	stream.mu.Lock()
	if stream.used || stream.closed || sink == nil {
		stream.mu.Unlock()
		return backend.DocumentImageAssemblyCapture{}, backend.ErrUnsupported
	}
	stream.used = true
	stream.mu.Unlock()
	defer func() {
		if closeErr := stream.Close(); err == nil && closeErr != nil {
			out = backend.DocumentImageAssemblyCapture{}
			err = closeErr
		}
	}()
	stop := context.AfterFunc(ctx, func() { _ = stream.Close() })
	defer stop()
	return readAssemblyResultStream(ctx, stream.response.Reader, sink)
}

// Constant-memory Docker demultiplexer. Every raw header/byte and frame is
// charged before reading; zero, foreign, truncated or oversized frames refuse.
type assemblyDockerReader struct {
	ctx       context.Context
	reader    io.Reader
	raw       int64
	remaining int64
	frames    int
}

func (r *assemblyDockerReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if r.ctx.Err() != nil {
		return 0, backend.ErrUnavailable
	}
	if r.remaining == 0 {
		var header [8]byte
		n, err := io.ReadFull(r.reader, header[:])
		r.raw += int64(n)
		if err == io.EOF && n == 0 {
			return 0, io.EOF
		}
		r.frames++
		if err != nil || r.raw > backend.DocumentImageAssemblyRawMaxBytes || r.frames > 65536 || header[0] != 1 || header[1] != 0 || header[2] != 0 || header[3] != 0 {
			return 0, backend.ErrUnavailable
		}
		r.remaining = int64(binary.BigEndian.Uint32(header[4:]))
		if r.remaining < 1 || r.remaining > 16<<20 || r.remaining > backend.DocumentImageAssemblyRawMaxBytes-r.raw {
			return 0, backend.ErrUnavailable
		}
	}
	if int64(len(p)) > r.remaining {
		p = p[:int(r.remaining)]
	}
	n, err := r.reader.Read(p)
	r.raw += int64(n)
	r.remaining -= int64(n)
	if r.raw > backend.DocumentImageAssemblyRawMaxBytes || n == 0 && err == nil || err == io.EOF && r.remaining != 0 {
		return n, backend.ErrUnavailable
	}
	// A physical EOF at the exact frame boundary still needs the outer parser's
	// complete trailer and EOF check. io.Reader may return final bytes with EOF.
	if n > 0 && err == io.EOF {
		err = nil
	}
	return n, err
}
func readAssemblyResultStream(ctx context.Context, reader io.Reader, sink io.Writer) (backend.DocumentImageAssemblyCapture, error) {
	zero := backend.DocumentImageAssemblyCapture{}
	if reader == nil || sink == nil {
		return zero, backend.ErrUnavailable
	}
	r := &assemblyDockerReader{ctx: ctx, reader: reader}
	prefix := make([]byte, len(assemblyResultMagic)+8+32)
	if _, err := io.ReadFull(r, prefix); err != nil || !bytes.HasPrefix(prefix, []byte(assemblyResultMagic)) {
		return zero, backend.ErrUnavailable
	}
	at := len(assemblyResultMagic)
	size := binary.BigEndian.Uint64(prefix[at : at+8])
	if size < 1 || size > backend.DocumentImageAssemblyOutputMaxBytes {
		return zero, backend.ErrUnavailable
	}
	h := sha256.New()
	var buffer [64 << 10]byte
	remaining := int64(size)
	for remaining > 0 {
		if ctx.Err() != nil {
			return zero, backend.ErrUnavailable
		}
		chunk := int64(len(buffer))
		if remaining < chunk {
			chunk = remaining
		}
		n, err := io.ReadFull(r, buffer[:int(chunk)])
		if err != nil {
			return zero, backend.ErrUnavailable
		}
		_, _ = h.Write(buffer[:n])
		written, err := sink.Write(buffer[:n])
		if err != nil || written != n {
			return zero, backend.ErrUnavailable
		}
		remaining -= int64(n)
	}
	trailer := make([]byte, len(assemblyResultEnd))
	if _, err := io.ReadFull(r, trailer); err != nil || !bytes.Equal(trailer, []byte(assemblyResultEnd)) || !bytes.Equal(h.Sum(nil), prefix[at+8:]) {
		return zero, backend.ErrUnavailable
	}
	var extra [1]byte
	if n, err := r.Read(extra[:]); n != 0 || err != io.EOF || ctx.Err() != nil {
		return zero, backend.ErrUnavailable
	}
	return backend.DocumentImageAssemblyCapture{Digest: hex.EncodeToString(h.Sum(nil)), Bytes: int64(size)}, nil
}

type assemblyResultStream struct {
	response  types.HijackedResponse
	client    *client.Client
	stop      func() bool
	closeOnce sync.Once
	closeErr  error
	mu        sync.Mutex
	used      bool
	closed    bool
	identity  backend.DocumentImageAssemblyIdentity
	owner     *assemblyAttachOwner
	onClose   func()
}

func (stream *assemblyResultStream) Close() error {
	stream.closeOnce.Do(func() {
		stream.mu.Lock()
		stream.closed = true
		stop := stream.stop
		stream.mu.Unlock()
		if stop != nil {
			stop()
		}
		if stream.owner != nil {
			if stream.owner.Close() != nil {
				stream.closeErr = backend.ErrUnavailable
			}
		} else if stream.response.Conn != nil {
			if err := stream.response.Conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				stream.closeErr = backend.ErrUnavailable
			}
		}
		if stream.client != nil {
			if err := stream.client.Close(); err != nil {
				stream.closeErr = backend.ErrUnavailable
			}
		}
		if stream.onClose != nil && stream.closeErr == nil {
			stream.onClose()
		}
	})
	return stream.closeErr
}
func (b *DocumentImageAssembler) AttachDocumentImageAssembly(ctx context.Context, identity backend.DocumentImageAssemblyIdentity, ref string) (backend.DocumentImageAssemblyStream, error) {
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	row, absent, err := b.inspectAssembly(bounded, identity, ref, true)
	if err != nil {
		return nil, err
	}
	if absent || row.State.Status != "created" || row.State.Running || row.State.Restarting || row.State.Paused || row.State.Dead || row.State.Pid != 0 {
		return nil, backend.ErrNotRunning
	}
	source, ok := b.api.(documentAttachSource)
	if !ok {
		return nil, backend.ErrUnsupported
	}
	selected := source.HTTPClient()
	if selected == nil {
		return nil, backend.ErrUnsupported
	}
	transport, ok := selected.Transport.(*http.Transport)
	// Only the selected local socket transport's context-aware dial path is
	// supported. Refuse opaque/TLS fallback transports rather than silently
	// changing their semantics or invoking the SDK's unbounded fallback dial.
	if !ok || transport.DialContext == nil || transport.TLSClientConfig != nil || !localFenceEndpoint(source.DaemonHost()) || !regexp.MustCompile(`^1\.[0-9]{1,3}$`).MatchString(source.ClientVersion()) {
		return nil, backend.ErrUnsupported
	}
	ownedHTTP := *selected
	ownedHTTP.Transport = transport.Clone()
	// The pinned SDK hijack performs raw HTTP ReadResponse. Bound its actual
	// connection before handshake; a context alone does not close that read.
	dial := source.Dialer()
	if dial == nil {
		return nil, backend.ErrUnsupported
	}
	handshakeDeadline, _ := bounded.Deadline()
	owner := &assemblyAttachOwner{identity: identity}
	if _, loaded := b.handshakes.LoadOrStore(ref, owner); loaded {
		return nil, backend.ErrUnavailable
	}
	transferred := false
	defer func() {
		if !transferred && owner.Close() == nil {
			b.handshakes.CompareAndDelete(ref, owner)
		}
	}()
	closeConnections := func() { _ = owner.Close() }
	stopHandshake := context.AfterFunc(bounded, closeConnections)
	defer stopHandshake()
	attach, err := client.NewClientWithOpts(client.WithHost(source.DaemonHost()), client.WithVersion(source.ClientVersion()), client.WithHTTPClient(&ownedHTTP), client.WithHTTPHeaders(b.attachHeaders), client.WithDialContext(func(dialCtx context.Context, _ string, _ string) (net.Conn, error) {
		conn, e := dial(dialCtx)
		if e != nil || conn == nil {
			if e == nil {
				e = backend.ErrUnavailable
			}
			return nil, e
		}
		owner.mu.Lock()
		defer owner.mu.Unlock()
		owner.connections = append(owner.connections, conn)
		if owner.closed || dialCtx.Err() != nil || len(owner.connections) > 2 {
			// Keep even a failed close reachable in the registered owner.
			_ = conn.Close()
			return nil, backend.ErrUnavailable
		}
		if e = conn.SetDeadline(handshakeDeadline); e != nil {
			conn.Close()
			return nil, e
		}
		return conn, nil
	}))
	if err != nil {
		closeConnections()
		return nil, backend.ErrUnsupported
	}
	response, err := attach.ContainerAttach(bounded, ref, container.AttachOptions{Stream: true, Stdout: true})
	if err != nil || bounded.Err() != nil || response.Conn == nil || response.Reader == nil {
		closeConnections()
		if response.Conn != nil {
			response.Close()
		}
		_ = attach.Close()
		return nil, backend.ErrUnavailable
	}
	if !stopHandshake() {
		closeConnections()
		response.Close()
		_ = attach.Close()
		return nil, backend.ErrUnavailable
	}
	stream := &assemblyResultStream{response: response, client: attach, identity: identity, owner: owner}
	stream.onClose = func() { b.attached.CompareAndDelete(ref, stream); b.handshakes.CompareAndDelete(ref, owner) }
	deadline := time.Now().Add(backend.DocumentImageAssemblyComputeBudget)
	if actual, ok := ctx.Deadline(); ok && actual.Before(deadline) {
		deadline = actual
	}
	if response.Conn.SetDeadline(deadline) != nil {
		stream.Close()
		return nil, backend.ErrUnavailable
	}
	fresh, absent, err := b.inspectAssembly(bounded, identity, ref, true)
	if err != nil || absent || fresh.State.Status != "created" || fresh.State.Running || fresh.State.Pid != 0 {
		stream.Close()
		return nil, backend.ErrUnavailable
	}
	if _, loaded := b.attached.LoadOrStore(ref, stream); loaded {
		stream.Close()
		return nil, backend.ErrUnavailable
	}
	stream.mu.Lock()
	stream.stop = context.AfterFunc(ctx, func() { _ = stream.Close() })
	stream.mu.Unlock()
	transferred = true
	return stream, nil
}
