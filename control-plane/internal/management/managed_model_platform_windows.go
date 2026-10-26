//go:build windows

package management

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
	"golang.org/x/sys/windows"
)

type windowsManagedModelPlatform struct{ root string }

func newManagedModelPlatform(root string) managedModelPlatform {
	return windowsManagedModelPlatform{root: root}
}

type managedModelPinnedFile struct {
	handle                                 windows.Handle
	path, digest                           string
	size                                   int64
	closeAttempted, closed, cleanupUnknown bool
}

// A mandatory no-write/no-delete sharing lock spans signature verification,
// hashing, atomic creation and the entire original child lifetime.
func pinManagedModelBinary(path, vendor string) (*managedModelPinnedFile, error) {
	if err := managedModelOrdinaryPath(path, false); err != nil {
		return nil, err
	}
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, err
	}
	pin := &managedModelPinnedFile{handle: h, path: path}
	fail := func(e error) (*managedModelPinnedFile, error) {
		pin.closeAttempted = true
		if ce := windows.CloseHandle(h); ce != nil {
			return pin, errors.Join(e, ce)
		}
		pin.closed = true
		if pin.cleanupUnknown {
			return pin, e
		}
		return nil, e
	}
	if err = verifyManagedModelSigner(p, h, vendor); err != nil {
		if strings.Contains(err.Error(), "cleanup unavailable") {
			pin.cleanupUnknown = true
		}
		return fail(err)
	}
	// Duplicate the read handle for Go's file wrapper; the original lock is never
	// consumed by a finalizer or reopened by PID/path after process launch.
	var duplicate windows.Handle
	if err = windows.DuplicateHandle(windows.CurrentProcess(), h, windows.CurrentProcess(), &duplicate, 0, false, windows.DUPLICATE_SAME_ACCESS); err != nil {
		return fail(err)
	}
	reader := os.NewFile(uintptr(duplicate), path)
	info, err := reader.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > 512<<20 {
		pin.cleanupUnknown = reader.Close() != nil
		return fail(errors.New("binary size invalid"))
	}
	_, err = reader.Seek(0, io.SeekStart)
	if err != nil {
		pin.cleanupUnknown = reader.Close() != nil
		return fail(err)
	}
	sum := sha256.New()
	n, e := io.Copy(sum, io.LimitReader(reader, info.Size()+1))
	closeErr := reader.Close()
	pin.cleanupUnknown = closeErr != nil
	if e != nil || closeErr != nil || n != info.Size() {
		return fail(errors.New("binary hash unavailable"))
	}
	pin.size = info.Size()
	pin.digest = hex.EncodeToString(sum.Sum(nil))
	return pin, nil
}

// Offline trust only: never acquire certificates or contact a registry. Extract
// the verified leaf signer; a trusted certificate from another vendor is refused.
func verifyManagedModelSigner(path *uint16, file windows.Handle, vendor string) (result error) {
	fi := windows.WinTrustFileInfo{FilePath: path, File: file}
	fi.Size = uint32(unsafe.Sizeof(fi))
	d := windows.WinTrustData{UIChoice: windows.WTD_UI_NONE, RevocationChecks: windows.WTD_REVOKE_NONE, UnionChoice: windows.WTD_CHOICE_FILE, FileOrCatalogOrBlobOrSgnrOrCert: unsafe.Pointer(&fi), StateAction: windows.WTD_STATEACTION_VERIFY, ProvFlags: windows.WTD_CACHE_ONLY_URL_RETRIEVAL}
	d.Size = uint32(unsafe.Sizeof(d))
	err := windows.WinVerifyTrustEx(windows.InvalidHWND, &windows.WINTRUST_ACTION_GENERIC_VERIFY_V2, &d)
	defer func() {
		if d.StateData == 0 {
			return
		}
		d.StateAction = windows.WTD_STATEACTION_CLOSE
		if e := windows.WinVerifyTrustEx(windows.InvalidHWND, &windows.WINTRUST_ACTION_GENERIC_VERIFY_V2, &d); e != nil {
			result = errors.Join(result, errors.New("signature provider cleanup unavailable"))
		}
	}()
	if err != nil {
		if vendor == "Microsoft Windows Hardware Compatibility Publisher" {
			return verifyManagedModelCatalog(path, file, vendor)
		}
		return errors.New("installed vendor signature cannot be verified offline")
	}
	return managedModelTrustVendor(&d, vendor, "embedded")
}

// The public error remains fixed. Only the explicit installed-tool test may
// report this verified certificate's public leaf name, never raw vendor output.
type managedModelVendorMismatch struct {
	publisher, verifiedVia string
}

func (e *managedModelVendorMismatch) Error() string {
	return "installed executable is not signed by the supported vendor"
}

// These are distinct verified routes, not interchangeable trusted publishers.
// The GPU selector retains its WHCP catalog policy, while the exact installed
// NVIDIA tool may carry NVIDIA's own embedded signature. Ollama stays embedded.
func managedModelPublisherAllowed(vendor, verifiedVia, publisher string) bool {
	switch vendor {
	case "Microsoft Windows Hardware Compatibility Publisher":
		return verifiedVia == "embedded" && publisher == "NVIDIA Corporation" ||
			verifiedVia == "catalog" && publisher == vendor
	case "Ollama Inc.":
		return verifiedVia == "embedded" && publisher == vendor
	default:
		return false
	}
}

func managedModelTrustVendor(d *windows.WinTrustData, vendor, verifiedVia string) error {
	dll := windows.NewLazySystemDLL("wintrust.dll")
	data, _, _ := dll.NewProc("WTHelperProvDataFromStateData").Call(uintptr(d.StateData))
	if data == 0 {
		return errors.New("vendor signer unavailable")
	}
	signer, _, _ := dll.NewProc("WTHelperGetProvSignerFromChain").Call(data, 0, 0, 0)
	type providerSigner struct {
		size  uint32
		at    windows.Filetime
		count uint32
		certs uintptr
	}
	type providerCert struct {
		size uint32
		cert *windows.CertContext
	}
	if signer == 0 {
		return errors.New("vendor signer unavailable")
	}
	s := (*providerSigner)(unsafe.Pointer(signer))
	if s.count < 1 || s.count > 32 || s.certs == 0 {
		return errors.New("vendor chain unavailable")
	}
	cert := (*providerCert)(unsafe.Pointer(s.certs)).cert
	name := make([]uint16, 512)
	n := windows.CertGetNameString(cert, windows.CERT_NAME_SIMPLE_DISPLAY_TYPE, 0, nil, &name[0], uint32(len(name)))
	if n < 2 || n > uint32(len(name)) {
		return errors.New("vendor identity unavailable")
	}
	actual := windows.UTF16ToString(name[:n])
	if !managedModelPublisherAllowed(vendor, verifiedVia, actual) {
		return &managedModelVendorMismatch{publisher: actual, verifiedVia: verifiedVia}
	}
	return nil
}
func verifyManagedNVIDIAMetadata(path string) error {
	p, e := windows.UTF16PtrFromString(path)
	if e != nil {
		return e
	}
	dll := windows.NewLazySystemDLL("version.dll")
	var ignored uint32
	n, _, _ := dll.NewProc("GetFileVersionInfoSizeW").Call(uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&ignored)))
	if n == 0 || n > 256<<10 {
		return errors.New("NVIDIA version metadata unavailable")
	}
	buf := make([]byte, int(n))
	ok, _, _ := dll.NewProc("GetFileVersionInfoW").Call(uintptr(unsafe.Pointer(p)), 0, n, uintptr(unsafe.Pointer(&buf[0])))
	if ok == 0 {
		return errors.New("NVIDIA version metadata unavailable")
	}
	query := func(key string) (unsafe.Pointer, uint32, error) {
		q, _ := windows.UTF16PtrFromString(key)
		var value unsafe.Pointer
		var size uint32
		r, _, _ := dll.NewProc("VerQueryValueW").Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(q)), uintptr(unsafe.Pointer(&value)), uintptr(unsafe.Pointer(&size)))
		if r == 0 || value == nil {
			return nil, 0, errors.New("NVIDIA version metadata missing")
		}
		start, end := uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&buf[0]))+uintptr(len(buf))
		if uintptr(value) < start || uintptr(value) >= end {
			return nil, 0, errors.New("NVIDIA version metadata invalid")
		}
		return value, size, nil
	}
	translation, size, e := query(`\VarFileInfo\Translation`)
	if e != nil || size < 4 || uintptr(translation)+4 > uintptr(unsafe.Pointer(&buf[0]))+uintptr(len(buf)) {
		return errors.New("NVIDIA version translation unavailable")
	}
	raw := unsafe.Slice((*byte)(translation), 4)
	lang, codepage := binary.LittleEndian.Uint16(raw[:2]), binary.LittleEndian.Uint16(raw[2:])
	text := func(name string) (string, error) {
		value, n, e := query(fmt.Sprintf(`\StringFileInfo\%04x%04x\%s`, lang, codepage, name))
		if e != nil || n == 0 || n > 512 || uintptr(value)+uintptr(n)*2 > uintptr(unsafe.Pointer(&buf[0]))+uintptr(len(buf)) {
			return "", errors.New("NVIDIA version text invalid")
		}
		return windows.UTF16ToString(unsafe.Slice((*uint16)(value), n)), nil
	}
	company, e := text("CompanyName")
	if e != nil || company != "NVIDIA Corporation" {
		return errors.New("NVIDIA vendor metadata differs")
	}
	product, e := text("ProductName")
	if e != nil || !strings.HasPrefix(product, "NVIDIA-SMI ") {
		return errors.New("NVIDIA product metadata differs")
	}
	return nil
}

type managedModelPipe struct {
	read, write                                                  windows.Handle
	mu                                                           sync.Mutex
	tail                                                         []byte
	pending                                                      []byte
	line                                                         func([]byte)
	done                                                         chan struct{}
	readErr                                                      error
	writerAttempted, readerAttempted, writerClosed, readerClosed bool
}

func newManagedModelPipe(line func([]byte)) (*managedModelPipe, error) {
	p := &managedModelPipe{done: make(chan struct{}), line: line}
	sa := windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), InheritHandle: 1}
	if e := windows.CreatePipe(&p.read, &p.write, &sa, 0); e != nil {
		return nil, e
	}
	if e := windows.SetHandleInformation(p.read, windows.HANDLE_FLAG_INHERIT, 0); e != nil {
		close(p.done)
		return p, e
	}
	go func() {
		defer close(p.done)
		buf := make([]byte, 4096)
		for {
			var n uint32
			e := windows.ReadFile(p.read, buf, &n, nil)
			if n > 0 {
				p.mu.Lock()
				p.tail = append(p.tail, buf[:n]...)
				if len(p.tail) > 32768 {
					p.tail = append([]byte(nil), p.tail[len(p.tail)-32768:]...)
				}
				if p.line != nil {
					p.pending = append(p.pending, buf[:n]...)
					for {
						at := strings.IndexByte(string(p.pending), '\n')
						if at < 0 {
							break
						}
						row := append([]byte(nil), p.pending[:at]...)
						p.pending = p.pending[at+1:]
						if len(row) <= 1024 {
							p.line(row)
						} else {
							p.readErr = errors.New("bounded vendor metadata exceeded")
						}
					}
					if len(p.pending) > 1024 {
						p.readErr = errors.New("bounded vendor metadata exceeded")
						p.pending = nil
					}
				}
				p.mu.Unlock()
			}
			if e != nil {
				if e != windows.ERROR_BROKEN_PIPE && e != windows.ERROR_OPERATION_ABORTED {
					p.mu.Lock()
					p.readErr = e
					p.mu.Unlock()
				}
				return
			}
			if n == 0 {
				return
			}
		}
	}()
	return p, nil
}
func (p *managedModelPipe) closeWriter() error {
	if p.writerClosed {
		return nil
	}
	if p.writerAttempted {
		return errors.New("original pipe writer closure unknown")
	}
	p.writerAttempted = true
	e := windows.CloseHandle(p.write)
	p.writerClosed = e == nil
	return e
}
func (p *managedModelPipe) close(ctx context.Context) error {
	if e := p.closeWriter(); e != nil {
		return e
	}
	select {
	case <-p.done:
	default:
		_ = windows.CancelIoEx(p.read, nil)
		select {
		case <-p.done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if p.readerClosed {
		return nil
	}
	if p.readerAttempted {
		return errors.New("original pipe reader closure unknown")
	}
	p.readerAttempted = true
	e := windows.CloseHandle(p.read)
	p.readerClosed = e == nil
	return e
}
func (p *managedModelPipe) failureCode() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	v := strings.ToLower(string(p.tail))
	switch {
	case strings.Contains(v, "out of memory"), strings.Contains(v, "cuda_error_out_of_memory"):
		return "model_memory_unavailable"
	case strings.Contains(v, "address already in use"), strings.Contains(v, "only one usage"):
		return "port_unavailable"
	case strings.Contains(v, "model") && strings.Contains(v, "not found"):
		return "model_unavailable"
	default:
		return "startup_exit"
	}
}

func (p *managedModelPipe) observationError() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.readErr == nil {
		select {
		case <-p.done:
			return errors.New("GPU observation pipe closed")
		default:
		}
	}
	return p.readErr
}

type managedModelGPU struct {
	mu          sync.Mutex
	uuid        string
	free, total uint64
	at          time.Time
	invalid     bool
}

func (g *managedModelGPU) line(raw []byte) {
	rows, e := csv.NewReader(strings.NewReader(strings.TrimSpace(string(raw)))).Read()
	g.mu.Lock()
	defer g.mu.Unlock()
	if e != nil || len(rows) != 3 {
		g.invalid = true
		return
	}
	id := strings.TrimSpace(rows[0])
	total, e1 := strconv.ParseUint(strings.TrimSpace(rows[1]), 10, 64)
	free, e2 := strconv.ParseUint(strings.TrimSpace(rows[2]), 10, 64)
	if id == "" || len(id) > 100 || e1 != nil || e2 != nil || total == 0 || total > 1<<20 || free > total || g.uuid != "" && g.uuid != id {
		g.invalid = true
		return
	}
	g.uuid = id
	g.total = total << 20
	g.free = free << 20
	g.at = time.Now()
}
func (g *managedModelGPU) check(start bool) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	min := uint64(1 << 30)
	if start {
		min = 6 << 30
	}
	if g.invalid || g.at.IsZero() || time.Since(g.at) > 5*time.Second || time.Since(g.at) < 0 || g.free < min {
		return errors.New("GPU capacity unavailable")
	}
	return nil
}

// Startup diagnostics use only fixed codes and the retained query's exit status.
// Missing/low samples may recover within the original five-second wait; malformed
// data, a failed pipe or a terminal original cannot become successful evidence.
func (g *managedModelGPU) startupFailure(now time.Time) string {
	g.mu.Lock()
	defer g.mu.Unlock()
	switch {
	case g.invalid:
		return "gpu_sample_invalid"
	case g.at.IsZero():
		return "gpu_sample_missing"
	case now.Before(g.at) || now.Sub(g.at) > 5*time.Second:
		return "gpu_sample_stale"
	case g.free < 6<<30:
		return "gpu_capacity_low"
	default:
		return ""
	}
}

func waitManagedGPUStartup(ctx context.Context, gpu *managedModelGPU,
	observe func() (managedModelOwnerStatus, error), pipeError func() error,
	now func() time.Time, pause func(context.Context, time.Duration) error) error {
	deadline := now().Add(5 * time.Second)
	for {
		if e := ctx.Err(); e != nil {
			return e
		}
		state, e := observe()
		if e != nil {
			return &managedModelStartError{code: "gpu_process_unavailable", cause: e}
		}
		if state.terminal {
			code := state.exitCode
			return &managedModelStartError{code: "gpu_process_exited", exitCode: &code}
		}
		if e = pipeError(); e != nil {
			return &managedModelStartError{code: "gpu_pipe_unavailable", cause: e}
		}
		if e = ctx.Err(); e != nil {
			return e
		}
		observed := now()
		code := gpu.startupFailure(observed)
		if code == "" && !observed.After(deadline) {
			return nil
		}
		if code == "" {
			code = "gpu_sample_missing"
		}
		if code == "gpu_sample_invalid" || !observed.Before(deadline) {
			return &managedModelStartError{code: code}
		}
		if e = pause(ctx, 100*time.Millisecond); e != nil {
			return e
		}
	}
}

type windowsManagedModelService struct {
	config                 managedModelConfig
	owner, gpuOwner        *managedModelProcessOwner
	binary, gpuBinary      *managedModelPinnedFile
	diagnostics, gpuOutput *managedModelPipe
	gpu                    managedModelGPU
	host                   *resourcebudget.HostProbe
	initialHostSample      bool
	endpoint               string
	http                   *http.Client
	transport              *http.Transport
	modelDigest            string
	content                []*managedModelPinnedFile
	startupDeadline        time.Time
}

func (platform windowsManagedModelPlatform) start(ctx context.Context, c managedModelConfig, gen uint64) (managedModelService, error) {
	s := &windowsManagedModelService{config: c, host: resourcebudget.NewHostProbe(resourcebudget.ProbeOptions{}), initialHostSample: true, endpoint: "http://127.0.0.1:" + strconv.Itoa(c.Port)}
	s.startupDeadline, _ = ctx.Deadline()
	stage := "model_store_unavailable"
	fail := func(e error) (managedModelService, error) {
		failure := &managedModelStartError{code: stage, cause: e}
		var detail *managedModelStartError
		if errors.As(e, &detail) {
			failure.code, failure.exitCode = detail.code, detail.exitCode
		}
		var hostFailure *managedHostCapacityError
		if errors.As(e, &hostFailure) {
			failure.code = "host_capacity_unavailable"
		}
		cleanup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if ce := s.stop(cleanup); ce != nil {
			return s, errors.Join(failure, ce)
		}
		return nil, failure
	}
	if e := managedModelOrdinaryPath(c.ModelStore, true); e != nil {
		return fail(e)
	}
	stage = "binary_untrusted"
	var e error
	s.binary, e = pinManagedModelBinary(c.ExecutablePath, "Ollama Inc.")
	if e != nil {
		if errors.Is(e, os.ErrNotExist) {
			stage = "binary_unavailable"
		}
		return fail(e)
	}
	stage = "gpu_binary_untrusted"
	sys, e := windows.GetSystemDirectory()
	if e != nil {
		return fail(e)
	}
	s.gpuBinary, e = pinManagedModelBinary(filepath.Join(sys, "nvidia-smi.exe"), "Microsoft Windows Hardware Compatibility Publisher")
	if e != nil {
		return fail(e)
	}
	stage = "gpu_metadata_unavailable"
	if e = verifyManagedNVIDIAMetadata(s.gpuBinary.path); e != nil {
		return fail(e)
	}
	stage = "startup_exit"
	s.diagnostics, e = newManagedModelPipe(nil)
	if e != nil {
		return fail(e)
	}
	stage = "gpu_pipe_unavailable"
	s.gpuOutput, e = newManagedModelPipe(s.gpu.line)
	if e != nil {
		return fail(e)
	}
	stage = "startup_exit"
	root, e := windows.GetWindowsDirectory()
	if e != nil {
		return fail(e)
	}
	home := filepath.Join(platform.root, "managed-model-home")
	for _, p := range []string{home, filepath.Join(home, "temp"), filepath.Join(home, "AppData", "Local"), filepath.Join(home, "AppData", "Roaming")} {
		if e = os.MkdirAll(p, 0700); e != nil {
			return fail(e)
		}
		if e = managedModelOrdinaryPath(p, true); e != nil {
			return fail(e)
		}
	}
	env := map[string]string{"SystemRoot": root, "WINDIR": root, "PATH": filepath.Dir(c.ExecutablePath) + ";" + sys, "USERPROFILE": home, "LOCALAPPDATA": filepath.Join(home, "AppData", "Local"), "APPDATA": filepath.Join(home, "AppData", "Roaming"), "TEMP": filepath.Join(home, "temp"), "TMP": filepath.Join(home, "temp"), "OLLAMA_HOST": "127.0.0.1:" + strconv.Itoa(c.Port), "OLLAMA_MODELS": c.ModelStore, "OLLAMA_CONTEXT_LENGTH": "16384", "OLLAMA_KEEP_ALIVE": "120s", "OLLAMA_NUM_PARALLEL": "1", "OLLAMA_MAX_LOADED_MODELS": "1", "OLLAMA_KV_CACHE_TYPE": "q8_0", "OLLAMA_FLASH_ATTENTION": "1", "OLLAMA_NOPRUNE": "true"}
	env["OLLAMA_NO_CLOUD"] = "true"
	env["OLLAMA_NOHISTORY"] = "true"
	env["OLLAMA_DEBUG"] = "0"
	env["OLLAMA_DEBUG_LOG_REQUESTS"] = "false"
	env["OLLAMA_MAX_TRANSFER_STREAMS"] = "1"
	// Refuse every preexisting listener. A free-port race is detected again by
	// original Job listener membership before any HTTP metadata can be trusted.
	stage = "port_unavailable"
	if _, e = managedModelListener(c.Port); !errors.Is(e, os.ErrNotExist) {
		return fail(errors.New("configured port is already occupied or cannot be observed"))
	}
	stage = "gpu_process_unavailable"
	s.gpuOwner, e = startManagedModelProcess(ctx, managedModelProcessSpec{inheritEnvironment: true, stdout: s.gpuOutput.write, executable: s.gpuBinary.path, directory: sys, args: []string{"--query-gpu=uuid,memory.total,memory.free", "--format=csv,noheader,nounits", "--loop-ms=1000"}, generation: gen, limits: managedModelLimits{jobBytes: 256 << 20, processBytes: 192 << 20, processes: 1, cpuRate: 200, stopTimeout: 30 * time.Second}})
	if e != nil {
		return fail(e)
	}
	stage = "gpu_pipe_unavailable"
	if e = s.gpuOutput.closeWriter(); e != nil {
		return fail(e)
	}
	s.host.Sample(ctx)
	if e = waitManagedGPUStartup(ctx, &s.gpu, s.gpuOwner.observe, s.gpuOutput.observationError, time.Now, func(ctx context.Context, delay time.Duration) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			return nil
		}
	}); e != nil {
		return fail(e)
	}
	stage = "startup_exit"
	// Two increasing samples are required for CPU; no fabricated utilization.
	select {
	case <-ctx.Done():
		return fail(ctx.Err())
	case <-time.After(time.Second):
	}
	stage = "host_capacity_unavailable"
	if _, e = observeManagedModelHost(ctx, s.host.Sample, time.Now, waitManagedModelHostRecheck, true, managedHostStartupAdmission, false); e != nil {
		return fail(e)
	}
	stage = "model_unavailable"
	s.content, s.modelDigest, e = pinManagedModelContent(ctx, c.ModelStore, c.Model)
	if e != nil {
		return fail(e)
	}
	stage = "startup_exit"
	if c.authorize == nil {
		return fail(errors.New("managed model authorization unavailable"))
	}
	beforeResume := func() error {
		if e := c.authorize(); e != nil {
			return e
		}
		if _, e := observeManagedModelHost(ctx, s.host.Sample, time.Now, waitManagedModelHostRecheck, true, managedHostPreResume, true); e != nil {
			return e
		}
		if e := s.gpu.check(true); e != nil {
			return e
		}
		state, e := s.gpuOwner.observe()
		if e != nil || state.terminal {
			return errors.New("original GPU query unavailable before model resume")
		}
		return ctx.Err()
	}
	s.owner, e = startManagedModelProcess(ctx, managedModelProcessSpec{beforeResume: beforeResume, stderr: s.diagnostics.write, executable: c.ExecutablePath, directory: filepath.Dir(c.ExecutablePath), args: []string{"serve"}, environment: env, generation: gen, limits: managedModelLimits{jobBytes: 7 << 30, processBytes: 6656 << 20, processes: 12, cpuRate: 2000, stopTimeout: 30 * time.Second}})
	if e != nil {
		return fail(e)
	}
	if e = s.diagnostics.closeWriter(); e != nil {
		return fail(e)
	}
	s.transport = &http.Transport{Proxy: nil, DisableKeepAlives: true, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		if address != "127.0.0.1:"+strconv.Itoa(c.Port) {
			return nil, errors.New("model endpoint changed")
		}
		if e := s.listener(); e != nil {
			return nil, e
		}
		conn, e := (&net.Dialer{Timeout: time.Second}).DialContext(ctx, network, address)
		if e != nil {
			return nil, e
		}
		if e = s.peer(conn); e != nil {
			closeErr := conn.Close()
			return nil, errors.Join(e, closeErr)
		}
		return conn, nil
	}}
	s.http = &http.Client{Transport: s.transport, Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return s, nil
}
func managedModelListener(port int) (uint32, error) {
	proc := windows.NewLazySystemDLL("iphlpapi.dll").NewProc("GetExtendedTcpTable")
	var n uint32
	r, _, _ := proc.Call(0, uintptr(unsafe.Pointer(&n)), 0, 2, 3, 0)
	if r != uintptr(windows.ERROR_INSUFFICIENT_BUFFER) || n < 4 || n > 1<<20 {
		return 0, errors.New("listener inventory unavailable")
	}
	buf := make([]byte, n)
	r, _, _ = proc.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n)), 0, 2, 3, 0)
	if r != 0 {
		return 0, errors.New("listener inventory unavailable")
	}
	count := binary.LittleEndian.Uint32(buf[:4])
	if uint64(count)*24+4 > uint64(len(buf)) {
		return 0, errors.New("listener inventory incomplete")
	}
	var pid uint32
	for i := uint32(0); i < count; i++ {
		row := buf[4+i*24 : 4+(i+1)*24]
		p := int(binary.BigEndian.Uint16(row[8:10]))
		if p == port {
			if row[4] != 127 || row[5] != 0 || row[6] != 0 || row[7] != 1 || pid != 0 {
				return 0, errors.New("listener address or multiplicity differs")
			}
			pid = binary.LittleEndian.Uint32(row[20:24])
		}
	}
	if pid == 0 {
		return 0, os.ErrNotExist
	}
	return pid, nil
}
func (s *windowsManagedModelService) listener() error {
	pid, e := managedModelListener(s.config.Port)
	if e != nil {
		return e
	}
	return s.ownedPID(pid)
}
func (s *windowsManagedModelService) ownedPID(pid uint32) error {
	s.owner.mu.Lock()
	defer s.owner.mu.Unlock()
	if s.owner.stopRequested || s.owner.process.attempted {
		return errors.New("model owner is stopping")
	}
	original := s.owner.latest.Load().(managedModelOwnerStatus)
	if pid != original.identity.pid {
		return errors.New("model socket does not belong to original server")
	}
	terminal, e := s.owner.kernel.wait(s.owner.process.value, 0)
	if e != nil || terminal {
		return errors.New("model original server is not live")
	}
	state, e := s.owner.kernel.jobState(s.owner.job.value)
	if e != nil {
		return e
	}
	for _, p := range state.pids {
		if p == pid {
			return nil
		}
	}
	return errors.New("model server is outside original owned Job")
}
func (s *windowsManagedModelService) peer(conn net.Conn) error {
	local, ok := conn.LocalAddr().(*net.TCPAddr)
	if !ok {
		return errors.New("model client socket identity unavailable")
	}
	remote, ok := conn.RemoteAddr().(*net.TCPAddr)
	if !ok || remote.Port != s.config.Port || !local.IP.Equal(net.IPv4(127, 0, 0, 1)) || !remote.IP.Equal(net.IPv4(127, 0, 0, 1)) {
		return errors.New("model connected peer differs")
	}
	proc := windows.NewLazySystemDLL("iphlpapi.dll").NewProc("GetExtendedTcpTable")
	var n uint32
	r, _, _ := proc.Call(0, uintptr(unsafe.Pointer(&n)), 0, 2, 5, 0)
	if r != uintptr(windows.ERROR_INSUFFICIENT_BUFFER) || n < 4 || n > 1<<20 {
		return errors.New("model peer inventory unavailable")
	}
	buf := make([]byte, n)
	r, _, _ = proc.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n)), 0, 2, 5, 0)
	if r != 0 {
		return errors.New("model peer inventory unavailable")
	}
	pid, e := managedModelPeerPID(buf, remote.Port, local.Port)
	if e != nil {
		return e
	}
	return s.ownedPID(pid)
}
func managedModelPeerPID(buf []byte, serverPort, clientPort int) (uint32, error) {
	if len(buf) < 4 {
		return 0, errors.New("model peer table incomplete")
	}
	count := binary.LittleEndian.Uint32(buf[:4])
	if uint64(count)*24+4 > uint64(len(buf)) {
		return 0, errors.New("model peer table incomplete")
	}
	var pid uint32
	for i := uint32(0); i < count; i++ {
		r := buf[4+i*24 : 4+(i+1)*24]
		if binary.LittleEndian.Uint32(r[:4]) == 5 && int(binary.BigEndian.Uint16(r[8:10])) == serverPort && int(binary.BigEndian.Uint16(r[16:18])) == clientPort && r[4] == 127 && r[5] == 0 && r[6] == 0 && r[7] == 1 && r[12] == 127 && r[13] == 0 && r[14] == 0 && r[15] == 1 {
			if pid != 0 {
				return 0, errors.New("model peer is ambiguous")
			}
			pid = binary.LittleEndian.Uint32(r[20:24])
		}
	}
	if pid == 0 {
		return 0, errors.New("model exact connected peer unavailable")
	}
	return pid, nil
}
func (s *windowsManagedModelService) metadata(ctx context.Context, path string, body string, out any) error {
	method := http.MethodGet
	var input io.Reader
	if body != "" {
		method = http.MethodPost
		input = strings.NewReader(body)
	}
	request, e := http.NewRequestWithContext(ctx, method, s.endpoint+path, input)
	if e != nil {
		return e
	}
	request.Header.Set("Content-Type", "application/json")
	response, e := s.http.Do(request)
	if e != nil {
		return e
	}
	defer response.Body.Close()
	if e = s.listener(); e != nil {
		return e
	}
	raw, e := io.ReadAll(io.LimitReader(response.Body, 256<<10+1))
	if e != nil || len(raw) > 256<<10 || response.StatusCode != 200 {
		return errors.New("model metadata unavailable")
	}
	return json.Unmarshal(raw, out)
}
func (s *windowsManagedModelService) sample(ctx context.Context) (managedModelSample, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	result := managedModelSample{}
	phase, initial := managedHostMonitor, s.initialHostSample
	s.initialHostSample = false
	if initial {
		phase = managedHostInitialMonitor
	}
	host, e := observeManagedModelHost(ctx, s.host.Sample, time.Now, waitManagedModelHostRecheck, false, phase, initial)
	if e != nil {
		result.failure = "host_capacity_unavailable"
		return result, e
	}
	if e := s.gpu.check(false); e != nil {
		result.failure = "gpu_capacity_unavailable"
		return result, e
	}
	result.at = host.ObservedAt
	s.gpu.mu.Lock()
	if s.gpu.at.Before(result.at) {
		result.at = s.gpu.at
	}
	s.gpu.mu.Unlock()
	gpu, e := s.gpuOwner.observe()
	if e != nil || gpu.terminal {
		result.failure = "gpu_observation_unavailable"
		return result, errors.New("GPU original query exited")
	}
	owner, e := s.owner.observe()
	if e != nil || owner.terminal {
		result.failure = s.diagnostics.failureCode()
		if owner.terminal {
			code := owner.exitCode
			result.exitCode = &code
		}
		return result, errors.New("model original process exited")
	}
	if e = s.listener(); e != nil {
		if errors.Is(e, os.ErrNotExist) {
			result.failure = "starting"
			return result, nil
		}
		result.failure = "listener_ownership_unavailable"
		return result, e
	}
	var tags struct {
		Models []struct {
			Name   string `json:"name"`
			Digest string `json:"digest"`
		} `json:"models"`
	}
	if e = s.metadata(ctx, "/api/tags", "", &tags); e != nil {
		return s.metadataFailure(result, e)
	}
	digest := ""
	for _, m := range tags.Models {
		if m.Name == s.config.Model {
			digest = m.Digest
		}
	}
	if len(digest) != 64 || strings.Trim(digest, "0123456789abcdef") != "" || s.modelDigest != "" && s.modelDigest != digest {
		result.failure = "model_unavailable"
		return result, errors.New("installed model identity differs")
	}
	s.modelDigest = digest
	var show struct {
		Parameters string `json:"parameters"`
	}
	payload, _ := json.Marshal(map[string]any{"model": s.config.Model, "verbose": false})
	if e = s.metadata(ctx, "/api/show", string(payload), &show); e != nil {
		return s.metadataFailure(result, e)
	}
	params := map[string]string{}
	for _, line := range strings.Split(show.Parameters, "\n") {
		parts := strings.Fields(line)
		if len(parts) == 2 {
			if _, duplicate := params[parts[0]]; duplicate {
				return result, errors.New("model parameters ambiguous")
			}
			params[parts[0]] = parts[1]
		}
	}
	if params["num_ctx"] != "16384" || params["num_thread"] != "3" || params["num_batch"] != "64" || params["num_gpu"] != "-1" {
		result.failure = "model_profile_mismatch"
		return result, errors.New("installed model does not match reviewed serving profile")
	}
	var loaded struct {
		Models []struct {
			Name    string `json:"name"`
			Digest  string `json:"digest"`
			Context int    `json:"context_length"`
			VRAM    uint64 `json:"size_vram"`
		} `json:"models"`
	}
	if e = s.metadata(ctx, "/api/ps", "", &loaded); e != nil {
		return s.metadataFailure(result, e)
	}
	if len(loaded.Models) > 1 {
		return result, errors.New("loaded model count differs")
	}
	for _, m := range loaded.Models {
		if m.Name != s.config.Model || m.Digest != digest || m.Context != 16384 || m.VRAM == 0 {
			result.failure = "loaded_model_profile_mismatch"
			return result, errors.New("loaded model profile differs")
		}
		result.loaded = true
	}
	return result, nil
}
func (s *windowsManagedModelService) metadataFailure(result managedModelSample, e error) (managedModelSample, error) {
	var transport net.Error
	if time.Now().Before(s.startupDeadline) && (errors.As(e, &transport) || errors.Is(e, io.EOF) || errors.Is(e, io.ErrUnexpectedEOF)) {
		result.failure = "starting"
		return result, nil
	}
	return result, e
}
func (s *windowsManagedModelService) stop(ctx context.Context) error {
	var failures []error
	for _, o := range []*managedModelProcessOwner{s.owner, s.gpuOwner} {
		if o != nil {
			state, e := o.stop(ctx)
			if e != nil || !state.handlesClosed || !state.jobEmpty {
				failures = append(failures, errors.New("original owned process retirement unverified"))
			}
		}
	}
	if len(failures) > 0 {
		return errors.Join(failures...)
	}
	for _, p := range []*managedModelPipe{s.diagnostics, s.gpuOutput} {
		if p != nil {
			if e := p.close(ctx); e != nil {
				failures = append(failures, e)
			}
		}
	}
	if s.transport != nil {
		s.transport.CloseIdleConnections()
	}
	for _, pin := range append([]*managedModelPinnedFile{s.binary, s.gpuBinary}, s.content...) {
		if pin != nil && pin.handle != 0 {
			if pin.cleanupUnknown || pin.closeAttempted && !pin.closed {
				failures = append(failures, errors.New("original content pin closure unknown"))
				continue
			}
			if pin.closed {
				continue
			}
			pin.closeAttempted = true
			if e := windows.CloseHandle(pin.handle); e != nil {
				failures = append(failures, e)
			} else {
				pin.closed = true
			}
		}
	}
	return errors.Join(failures...)
}
