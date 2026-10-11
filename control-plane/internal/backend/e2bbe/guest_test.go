package e2bbe

import (
	"bytes"
	"context"
	"crypto/x509"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/lcfrai/luci/control-plane/internal/backend"
)

type guestFixture struct {
	mu                                     sync.Mutex
	t                                      *testing.T
	server                                 *httptest.Server
	record                                 Record
	input, output                          []byte
	launchKernel                           []byte
	attachFrames                           [][]byte
	attachEntered                          chan struct{}
	attachHeld                             bool
	posts, gets, upgrades                  int
	lostACK, corrupt, reject, cancelOnPost bool
	denied                                 bool
	rpc                                    http.Handler
}

func newGuestFixture(t *testing.T) (*guestFixture, GuestConfig, []byte) {
	t.Helper()
	req := taskRequest("alpha", "one")
	var archive bytes.Buffer
	_, hash, err := WriteTransfer(&archive, transferBinding(), []Payload{payload("source/main.go", "package main"), payload("context/approved.json", `{"revision":17}`)})
	if err != nil {
		t.Fatal(err)
	}
	req.BundleDigest = hash
	f := &guestFixture{t: t, record: Record{Request: req, RequestDigest: digest(req), State: Allocated, SandboxID: "vm-one", ResourcesHeld: true, Revision: 2, CreatedAt: time.Now().UTC()}}
	f.server = httptest.NewTLSServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.server.Close)
	pool := x509.NewCertPool()
	pool.AddCert(f.server.Certificate())
	cfg := GuestConfig{Record: f.record, EnvdEndpoint: f.server.URL, SandboxEndpoint: f.server.URL, EnvdVersion: EnvdVersion, EnvdAccessToken: "fixture-envd", TrafficAccessToken: "fixture-traffic", RootCAs: pool,
		CurrentRecord: func(context.Context, string) (Record, error) { f.mu.Lock(); defer f.mu.Unlock(); return f.record, nil },
		Authorize: func(context.Context, Request, string) error {
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.denied {
				return backend.ErrUnauthorized
			}
			return nil
		}}
	return f, cfg, archive.Bytes()
}
func (f *guestFixture) handle(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	rpcHandler := f.rpc
	f.mu.Unlock()
	if strings.HasPrefix(r.URL.Path, "/process.Process/") && rpcHandler != nil {
		rpcHandler.ServeHTTP(w, r)
		return
	}
	if r.Header.Get("X-API-Key") != "" || r.Header.Get("e2b-traffic-access-token") != "fixture-traffic" {
		f.t.Error("wrong credential boundary")
		w.WriteHeader(401)
		return
	}
	if r.URL.Path == backend.AttachPath {
		if r.Header.Get("X-Access-Token") != "" || r.Header.Get("Authorization") != "Bearer fixture-attach" {
			w.WriteHeader(401)
			return
		}
		f.mu.Lock()
		f.upgrades++
		frames, entered, held := f.attachFrames, f.attachEntered, f.attachHeld
		f.mu.Unlock()
		conn, e := websocket.Accept(w, r, nil)
		if e != nil {
			return
		}
		defer conn.CloseNow()
		conn.SetReadLimit(backend.MaxMessageBytes)
		if entered != nil {
			close(entered)
		}
		if frames != nil {
			for _, frame := range frames {
				if conn.Write(r.Context(), websocket.MessageBinary, frame) != nil {
					return
				}
			}
			if held {
				_, _, _ = conn.Read(r.Context())
			}
			return
		}
		kind, b, e := conn.Read(r.Context())
		if e == nil {
			_ = conn.Write(r.Context(), kind, b)
		}
		return
	}
	if r.URL.Path != "/files" || r.URL.Query().Get("username") != "root" || len(r.URL.Query()) != 2 || r.Header.Get("Authorization") != "" {
		f.t.Error("wrong pinned file request")
		w.WriteHeader(400)
		return
	}
	if r.Header.Get("X-Access-Token") != "fixture-envd" {
		w.WriteHeader(401)
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.reject {
		w.WriteHeader(403)
		return
	}
	path := r.URL.Query().Get("path")
	if r.Method == "POST" && path == guestInputPath {
		f.posts++
		if r.Header.Get("Content-Type") != "application/octet-stream" || r.ContentLength <= 0 {
			f.t.Error("unbounded upload")
			w.WriteHeader(400)
			return
		}
		f.input, _ = io.ReadAll(io.LimitReader(r.Body, MaxTransferBytes+2<<20))
		if f.cancelOnPost {
			f.record.CancelRequested = true
		}
		if f.lostACK {
			w.WriteHeader(503)
			return
		}
		w.Write([]byte("{}"))
		return
	}
	if r.Method == "GET" {
		f.gets++
		var b []byte
		switch path {
		case guestInputPath:
			b = f.input
		case guestOutputPath:
			b = f.output
		case guestLaunchPath:
			b = f.launchKernel
		default:
			w.WriteHeader(400)
			return
		}
		if f.corrupt {
			b = []byte("corrupt readback")
		}
		w.Write(b)
		return
	}
	w.WriteHeader(405)
}
func openGuest(t *testing.T, cfg GuestConfig) *GuestTransport {
	t.Helper()
	g, e := NewGuestTransport(cfg)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(g.Close)
	return g
}

func TestGuestStagesOnlyVerifiedExactContextAndRemoteBytes(t *testing.T) {
	f, cfg, archive := newGuestFixture(t)
	g := openGuest(t, cfg)
	if _, e := g.StageInput(context.Background(), bytes.NewReader(archive)); e != nil {
		t.Fatal(e)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.posts != 1 || f.gets != 1 || !bytes.Equal(archive, f.input) {
		t.Fatal("transfer changed", f.posts, f.gets)
	}
}
func TestGuestDoesNotReplayLostUploadOrContinueAfterCancellation(t *testing.T) {
	for _, name := range []string{"lost-ack", "cancelled", "corrupt-readback", "denied"} {
		t.Run(name, func(t *testing.T) {
			f, cfg, archive := newGuestFixture(t)
			f.lostACK = name == "lost-ack"
			f.cancelOnPost = name == "cancelled"
			f.corrupt = name == "corrupt-readback"
			f.reject = name == "denied"
			g := openGuest(t, cfg)
			if _, e := g.StageInput(context.Background(), bytes.NewReader(archive)); e == nil {
				t.Fatal("uncertain upload accepted")
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			wantPosts := 1
			if name == "denied" {
				wantPosts = 0
			}
			wantGets := 0
			if name == "corrupt-readback" {
				wantGets = 1
			}
			if f.posts != wantPosts || f.gets != wantGets {
				t.Fatal(f.posts, f.gets)
			}
		})
	}
}
func TestGuestRejectsInvalidOrRevokedTransferBeforeHTTP(t *testing.T) {
	for _, name := range []string{"tamper", "foreign-record", "changed-template", "changed-created", "inconsistent-record", "expired", "denied", "cancelled-context", "closed", "nil"} {
		t.Run(name, func(t *testing.T) {
			f, cfg, archive := newGuestFixture(t)
			if name == "expired" {
				cfg.Record.CreatedAt = time.Now().Add(-time.Hour)
				f.record.CreatedAt = cfg.Record.CreatedAt
			}
			g := openGuest(t, cfg)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var input io.ReadSeeker = bytes.NewReader(archive)
			switch name {
			case "tamper":
				input = bytes.NewReader([]byte("wrong"))
			case "foreign-record":
				f.record.SandboxID = "vm-other"
			case "changed-template":
				f.record.Template.ID = "foreign"
			case "changed-created":
				f.record.CreatedAt = f.record.CreatedAt.Add(time.Second)
			case "inconsistent-record":
				f.record.Request.Identity.ProjectID = "other"
			case "denied":
				f.denied = true
			case "cancelled-context":
				cancel()
			case "closed":
				g.Close()
			case "nil":
				input = nil
			}
			if _, e := g.StageInput(ctx, input); e == nil {
				t.Fatal("invalid transfer accepted")
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.posts+f.gets != 0 {
				t.Fatal("unauthorized HTTP effect")
			}
		})
	}
}
func TestGuestWSSPreservesSandboxdFrameAndPrivateCredentialBoundaries(t *testing.T) {
	_, cfg, _ := newGuestFixture(t)
	g := openGuest(t, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, e := g.Attach(ctx, "fixture-attach")
	if e != nil {
		t.Fatal(e)
	}
	defer conn.CloseNow()
	frame := append([]byte{2}, bytes.Repeat([]byte("upstream output"), 6000)...)
	if e = conn.Write(ctx, websocket.MessageBinary, frame); e != nil {
		t.Fatal(e)
	}
	kind, got, e := conn.Read(ctx)
	if e != nil || kind != websocket.MessageBinary || !bytes.Equal(frame, got) {
		t.Fatal(kind, e, len(got))
	}
}
func TestGuestRejectsWrongAttachAndFileCredentials(t *testing.T) {
	_, cfg, archive := newGuestFixture(t)
	g := openGuest(t, cfg)
	if _, e := g.Attach(context.Background(), "wrong-token"); !errors.Is(e, backend.ErrUnauthorized) {
		t.Fatal(e)
	}
	cfg.EnvdAccessToken = "wrong-token"
	g2 := openGuest(t, cfg)
	if _, e := g2.StageInput(context.Background(), bytes.NewReader(archive)); e == nil {
		t.Fatal("wrong envd token accepted")
	}
}
func TestGuestReturnedArtifactsRetainExactTaskScopeInDurableStore(t *testing.T) {
	f, cfg, _ := newGuestFixture(t)
	g := openGuest(t, cfg)
	s := openStore(t, storeConfig(t))
	b := transferBinding()
	b.Direction = "output"
	var out bytes.Buffer
	_, hash, e := WriteTransfer(&out, b, []Payload{payload("output/result.patch", "untrusted upstream changes")})
	if e != nil {
		t.Fatal(e)
	}
	f.output = out.Bytes()
	if _, e = g.ReceiveOutput(context.Background(), hash, s); e != nil {
		t.Fatal(e)
	}
	var got bytes.Buffer
	if _, e = s.CopyTransfer(context.Background(), b, hash, &got); e != nil || !bytes.Equal(got.Bytes(), out.Bytes()) {
		t.Fatal(e)
	}
	b.Identity.ProjectID = "other"
	if _, e = s.CopyTransfer(context.Background(), b, hash, io.Discard); e == nil {
		t.Fatal("cross-project artifact accepted")
	}
}
func TestGuestRefusesUnsupportedVersionsOriginsAndSecretEchoes(t *testing.T) {
	_, cfg, _ := newGuestFixture(t)
	for _, origin := range []string{"http://127.0.0.1", "https://127.0.0.1:5008", "https://8.8.8.8", "https://user:password@127.0.0.1", "https://127.0.0.1/files", "https://127.0.0.1?token=secret"} {
		copy := cfg
		copy.EnvdEndpoint = origin
		if _, e := NewGuestTransport(copy); !errors.Is(e, ErrUnsupported) {
			t.Fatal(origin, e)
		}
	}
	cfg.EnvdVersion = "0.4.0"
	if _, e := NewGuestTransport(cfg); !errors.Is(e, ErrUnsupported) {
		t.Fatal(e)
	}
	if strings.Contains(cfg.String(), cfg.EnvdAccessToken) || strings.Contains(cfg.GoString(), cfg.TrafficAccessToken) {
		t.Fatal("configuration exposed credentials")
	}
}
