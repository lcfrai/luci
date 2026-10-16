// Source-only fixed PDF pilot driver. No product route or accepted runtime
// packages this command. The selected owner must prove the cgroup, image,
// original inputs and pre-Start attach before executing it.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image/png"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
)

const (
	roleRender = "render"
	roleRead = "read"
	fontPath = "/input/NotoSansCJK-Regular.ttc"
	pdfPath = "/input/pilot.pdf"
	writerPath = "/opt/luci/luci-pdf-pilot"
	pdfinfoPath = "/usr/bin/pdfinfo"
	pdftotextPath = "/usr/bin/pdftotext"
	pdftoppmPath = "/usr/bin/pdftoppm"
	fontBytes = 19484784
	fontDigest = "b76b0433203017ca80401b2ee0dd69350349871c4b19d504c34dbdd80541690a"
	maxPDF = 16 << 20
	maxText = 1 << 20
	maxPNG = 16 << 20
	maxPDFInfo = 16 << 10
	// Two seconds remain for bounded child-pipe reaping within the owner's
	// 30-second role deadline. The independent CID watchdog is still required.
	roleBudget = 28 * time.Second
)

var errRefused = errors.New("fixed PDF pilot role refused")

type driverRequest struct {
	role string
	attempt string
	allocation string
	inputDigest string
}

func validHex(value string, size int) bool {
	if len(value) != size * 2 || strings.ToLower(value) != value {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == size
}

// No free-form command, filesystem path, URL or environment value is accepted.
func parseRequest(args []string) (driverRequest, error) {
	if len(args) != 8 || args[0] != "--role" || args[2] != "--attempt" || args[4] != "--allocation" || args[6] != "--input-sha256" {
		return driverRequest{}, errRefused
	}
	r := driverRequest{role: args[1], attempt: args[3], allocation: args[5], inputDigest: args[7]}
	if (r.role != roleRender && r.role != roleRead) || !validHex(r.attempt, 16) || !validHex(r.allocation, 32) || !validHex(r.inputDigest, 32) ||
		(r.role == roleRender && r.inputDigest != fontDigest) {
		return driverRequest{}, errRefused
	}
	return r, nil
}

type pilotStage interface {
	Read(string, int) ([]byte, error)
	Only(...string) error
	Close() error
}

type driverOps struct {
	readInput func(string, int) ([]byte, error)
	newStage func(string) (pilotStage, error)
	runTool func(context.Context, string, []string, int) ([]byte, error)
}

func defaultOps() driverOps {
	return driverOps{readInput: readBoundedRegular, newStage: newPilotStage, runTool: runFixedTool}
}

func inputIdentity(r driverRequest, ops driverOps) error {
	path, limit := pdfPath, maxPDF
	if r.role == roleRender {
		path, limit = fontPath, fontBytes
	}
	data, err := ops.readInput(path, limit)
	if err != nil || len(data) == 0 || len(data) > limit || (r.role == roleRender && len(data) != fontBytes) {
		return errRefused
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != r.inputDigest {
		return errRefused
	}
	return nil
}

func readExact(stage pilotStage, name string, limit int) ([]byte, error) {
	data, err := stage.Read(name, limit)
	if err != nil || len(data) == 0 || len(data) > limit {
		return nil, errRefused
	}
	return data, nil
}

// pdfinfo is a separate standard-reader check. First-page pdftoppm output
// does not prove the PDF has only one page. Its complete bounded C-locale
// stdout must contain exactly one unambiguous Pages: 1 field.
func onePDFInfoPage(output []byte) bool {
	if len(output) == 0 || len(output) > maxPDFInfo || !utf8.Valid(output) || bytes.IndexByte(output, 0) >= 0 {
		return false
	}
	count := 0
	for _, b := range output {
		if b < 0x20 && b != '\n' && b != '\r' && b != '\t' || b == 0x7f {
			return false
		}
	}
	for _, line := range strings.Split(strings.TrimSuffix(string(output), "\n"), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if strings.HasPrefix(line, "Pages:") {
			value := strings.TrimPrefix(line, "Pages:")
			if value == "" || strings.Trim(value, " \t") != "1" || strings.TrimLeft(value, " \t") == value {
				return false
			}
			count++
		} else if strings.HasPrefix(line, "Pages") {
			return false
		}
	}
	return count == 1
}

func exactText(data []byte) bool {
	return utf8.Valid(data) && (bytes.Equal(data, []byte("日本語\n")) || bytes.Equal(data, []byte("日本語\r\n")))
}

func exactPNG(data []byte) bool {
	reader := bytes.NewReader(data)
	config, err := png.DecodeConfig(reader)
	if err != nil || config.Width != 595 || config.Height != 842 {
		return false
	}
	reader = bytes.NewReader(data)
	if _, err = png.Decode(reader); err != nil || reader.Len() != 0 {
		return false
	}
	return true
}

func runRole(ctx context.Context, r driverRequest, ops driverOps, output io.Writer) error {
	if ctx.Err() != nil || ops.readInput == nil || ops.newStage == nil || ops.runTool == nil || output == nil || inputIdentity(r, ops) != nil {
		return errRefused
	}
	stage, err := ops.newStage(r.attempt)
	if err != nil || stage == nil {
		return errRefused
	}
	defer stage.Close()
	base := "/out/pilot-" + r.attempt
	if r.role == roleRender {
		if _, err = ops.runTool(ctx, writerPath, []string{"--font", fontPath, "--out-dir", base}, 0); err != nil || ctx.Err() != nil || stage.Only("pilot.pdf") != nil {
			return errRefused
		}
		pdf, err := readExact(stage, "pilot.pdf", maxPDF)
		if err != nil || ctx.Err() != nil || inputIdentity(r, ops) != nil {
			return errRefused
		}
		if stage.Close() != nil || ctx.Err() != nil {
			return errRefused
		}
		return emitFrame(ctx, output, r, []frameRecord{{kind: recordPDF, data: pdf}})
	}
	info, err := ops.runTool(ctx, pdfinfoPath, []string{pdfPath}, maxPDFInfo)
	if err != nil || ctx.Err() != nil || !onePDFInfoPage(info) {
		return errRefused
	}
	if _, err = ops.runTool(ctx, pdftotextPath, []string{"-enc", "UTF-8", "-nopgbrk", pdfPath, base + "/pilot.txt"}, 0); err != nil || ctx.Err() != nil {
		return errRefused
	}
	if _, err = ops.runTool(ctx, pdftoppmPath, []string{"-f", "1", "-l", "1", "-singlefile", "-r", "72", "-png", pdfPath, base + "/page"}, 0); err != nil || ctx.Err() != nil || stage.Only("pilot.txt", "page.png") != nil {
		return errRefused
	}
	text, err := readExact(stage, "pilot.txt", maxText)
	if err != nil || !exactText(text) {
		return errRefused
	}
	page, err := readExact(stage, "page.png", maxPNG)
	if err != nil || !exactPNG(page) || ctx.Err() != nil || inputIdentity(r, ops) != nil {
		return errRefused
	}
	if stage.Close() != nil || ctx.Err() != nil {
		return errRefused
	}
	return emitFrame(ctx, output, r, []frameRecord{{kind: recordText, data: text}, {kind: recordPNG, data: page}})
}

type limitedOutput struct {
	limit int
	data []byte
	count int
	keep bool
}

func (w *limitedOutput) Write(p []byte) (int, error) {
	if len(p) > w.limit-w.count {
		return 0, errRefused
	}
	w.count += len(p)
	if w.keep {
		w.data = append(w.data, p...)
	}
	return len(p), nil
}

func runFixedTool(ctx context.Context, path string, args []string, stdoutLimit int) ([]byte, error) {
	if ctx.Err() != nil || (path != writerPath && path != pdfinfoPath && path != pdftotextPath && path != pdftoppmPath) || stdoutLimit < 0 || stdoutLimit > maxPDFInfo {
		return nil, errRefused
	}
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Env = []string{"LC_ALL=C", "LANG=C", "HOME=/tmp", "TMPDIR=/tmp"}
	cmd.WaitDelay = 2 * time.Second
	stdout := &limitedOutput{limit: stdoutLimit, keep: path == pdfinfoPath}
	stderr := &limitedOutput{limit: 4 << 10}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if cmd.Run() != nil || ctx.Err() != nil || stderr.count != 0 || (path != pdfinfoPath && stdout.count != 0) {
		return nil, errRefused
	}
	return stdout.data, nil
}

func driverMain(args []string, output io.Writer) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, roleBudget)
	defer cancel()
	r, err := parseRequest(args)
	if err != nil {
		return errRefused
	}
	return runRole(ctx, r, defaultOps(), output)
}

func main() {
	if driverMain(os.Args[1:], os.Stdout) != nil {
		_, _ = os.Stderr.WriteString("pdf pilot driver refused\n")
		os.Exit(1)
	}
}
