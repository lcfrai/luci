package dockerbe

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
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

const documentResultMagic = "LUCI-DOCUMENT-RESULT/1\n"
const documentResultEnd = "\nLUCI-DOCUMENT-END/1\n"
const documentRawStreamBytes = backend.DocumentResultMaxBytes + (16 << 10)

type documentAttachSource interface {
	DaemonHost() string
	ClientVersion() string
	Dialer() func(context.Context) (net.Conn, error)
	HTTPClient() *http.Client
}
type documentResultStream struct {
	response  types.HijackedResponse
	client    *client.Client
	stop      func() bool
	closeOnce sync.Once
	mu        sync.Mutex
	used      bool
	closed    bool
	identity  backend.DocumentExtractionIdentity
	onClose   func()
}

func (stream *documentResultStream) Close() {
	stream.closeOnce.Do(func() {
		stream.mu.Lock()
		stream.closed = true
		stop := stream.stop
		stream.mu.Unlock()
		if stop != nil {
			stop()
		}
		if stream.response.Conn != nil {
			stream.response.Close()
		}
		if stream.client != nil {
			_ = stream.client.Close()
		}
		if stream.onClose != nil {
			stream.onClose()
		}
	})
}
func (stream *documentResultStream) Capture(ctx context.Context) (backend.DocumentExtractionOutput, error) {
	stream.mu.Lock()
	if stream.used {
		stream.mu.Unlock()
		return backend.DocumentExtractionOutput{}, backend.ErrUnsupported
	}
	stream.used = true
	stream.mu.Unlock()
	defer stream.Close()
	stop := context.AfterFunc(ctx, stream.Close)
	defer stop()
	return readDocumentResultStream(ctx, stream.response.Reader)
}

// Docker framing is bounded before declared allocation. The raw budget
// includes every header/byte, including hostile empty or foreign frames.
func readDocumentResultStream(ctx context.Context, reader io.Reader) (backend.DocumentExtractionOutput, error) {
	if reader == nil {
		return backend.DocumentExtractionOutput{}, backend.ErrUnavailable
	}
	output := []byte{}
	rawBytes := 0
	maximum := backend.DocumentResultMaxBytes + len(documentResultMagic) + 4 + 32 + len(documentResultEnd)
	for {
		if ctx.Err() != nil {
			return backend.DocumentExtractionOutput{}, backend.ErrUnavailable
		}
		var header [8]byte
		n, err := io.ReadFull(reader, header[:])
		rawBytes += n
		if err == io.EOF {
			break
		}
		if err != nil || rawBytes > documentRawStreamBytes || header[0] != 1 || header[1] != 0 || header[2] != 0 || header[3] != 0 {
			return backend.DocumentExtractionOutput{}, backend.ErrUnavailable
		}
		size := uint64(binary.BigEndian.Uint32(header[4:]))
		if size == 0 || size > uint64(maximum-len(output)) || size > uint64(documentRawStreamBytes-rawBytes) {
			return backend.DocumentExtractionOutput{}, backend.ErrUnavailable
		}
		start := len(output)
		output = append(output, make([]byte, int(size))...)
		if _, err = io.ReadFull(reader, output[start:]); err != nil {
			return backend.DocumentExtractionOutput{}, backend.ErrUnavailable
		}
		rawBytes += int(size)
	}
	minimum := len(documentResultMagic) + 4 + 32 + 1 + len(documentResultEnd)
	if ctx.Err() != nil || len(output) < minimum || !bytes.HasPrefix(output, []byte(documentResultMagic)) {
		return backend.DocumentExtractionOutput{}, backend.ErrUnavailable
	}
	lengthAt := len(documentResultMagic)
	size := uint64(binary.BigEndian.Uint32(output[lengthAt : lengthAt+4]))
	payloadAt := lengthAt + 4 + 32
	if size < 1 || size > backend.DocumentResultMaxBytes || uint64(len(output)) != uint64(payloadAt+len(documentResultEnd))+size || !bytes.Equal(output[len(output)-len(documentResultEnd):], []byte(documentResultEnd)) {
		return backend.DocumentExtractionOutput{}, backend.ErrUnavailable
	}
	data := output[payloadAt : payloadAt+int(size)]
	digest := sha256.Sum256(data)
	if !bytes.Equal(digest[:], output[lengthAt+4:payloadAt]) {
		return backend.DocumentExtractionOutput{}, backend.ErrUnavailable
	}
	return backend.DocumentExtractionOutput{Data: append([]byte{}, data...), Digest: hex.EncodeToString(digest[:])}, nil
}

func (b *DocumentExtractor) AttachDocumentResult(ctx context.Context, identity backend.DocumentExtractionIdentity, ref string) (backend.DocumentExtractionResultStream, error) {
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	row, absent, err := b.inspectDocument(bounded, identity, ref, true)
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
	var connectionMu sync.Mutex
	connections := []net.Conn{}
	closed := false
	closeConnections := func() {
		connectionMu.Lock()
		defer connectionMu.Unlock()
		closed = true
		for _, conn := range connections {
			_ = conn.Close()
		}
	}
	stopHandshake := context.AfterFunc(bounded, closeConnections)
	defer stopHandshake()
	attach, err := client.NewClientWithOpts(client.WithHost(source.DaemonHost()), client.WithVersion(source.ClientVersion()), client.WithHTTPClient(&ownedHTTP), client.WithHTTPHeaders(b.attachHeaders), client.WithDialContext(func(dialCtx context.Context, _ string, _ string) (net.Conn, error) {
		conn, e := dial(dialCtx)
		if e != nil {
			return nil, e
		}
		connectionMu.Lock()
		defer connectionMu.Unlock()
		if closed || dialCtx.Err() != nil {
			conn.Close()
			return nil, backend.ErrUnavailable
		}
		connections = append(connections, conn)
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
	stream := &documentResultStream{response: response, client: attach, identity: identity}
	stream.onClose = func() { b.attached.CompareAndDelete(ref, stream) }
	deadline := time.Now().Add(backend.DocumentComputeBudget)
	if actual, ok := ctx.Deadline(); ok && actual.Before(deadline) {
		deadline = actual
	}
	if response.Conn.SetDeadline(deadline) != nil {
		stream.Close()
		return nil, backend.ErrUnavailable
	}
	fresh, absent, err := b.inspectDocument(bounded, identity, ref, true)
	if err != nil || absent || fresh.State.Status != "created" || fresh.State.Running || fresh.State.Pid != 0 {
		stream.Close()
		return nil, backend.ErrUnavailable
	}
	if _, loaded := b.attached.LoadOrStore(ref, stream); loaded {
		stream.Close()
		return nil, backend.ErrUnavailable
	}
	stream.mu.Lock()
	stream.stop = context.AfterFunc(ctx, stream.Close)
	stream.mu.Unlock()
	return stream, nil
}
