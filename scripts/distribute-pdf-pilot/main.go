// This is an isolated, fixed-input source pilot. It is never imported by the
// product or release build. An approved owned controller must provide verified
// offline modules, the exact font and a private empty output directory.
package main

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf16"

	"github.com/tdewolff/font"
)

const (
	fontSize         = 19484784
	fontSHA256       = "b76b0433203017ca80401b2ee0dd69350349871c4b19d504c34dbdd80541690a"
	maxFontBytes     = 32 << 20
	maxPDFBytes      = 16 << 20
	maxSubsetBytes   = 16 << 20
	maxCombinedBytes = 32 << 20
	originalText     = "日本語"
	faceIndex        = 0
)

var ownedStage = regexp.MustCompile(`^pilot-[0-9a-f]{32}$`)

func main() {
	fontPath := flag.String("font", "", "exact offline NotoSansCJK-Regular.ttc input")
	outDir := flag.String("out-dir", "", "empty private owned stage")
	flag.Parse()
	if flag.NArg() != 0 || *fontPath == "" || *outDir == "" {
		fail(errors.New("exact --font and --out-dir are required"))
	}
	if err := pilot(*fontPath, *outDir); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "pdf pilot refused:", err)
	os.Exit(1)
}

func pilot(fontPath, outDir string) error {
	if !filepath.IsAbs(fontPath) || !filepath.IsAbs(outDir) || !ownedStage.MatchString(filepath.Base(outDir)) {
		return errors.New("font and owned stage need absolute, exact paths")
	}
	stage, err := os.Lstat(outDir)
	if err != nil || !stage.IsDir() || stage.Mode()&os.ModeSymlink != 0 {
		return errors.New("owned stage is missing, linked or not a directory")
	}
	if entries, err := os.ReadDir(outDir); err != nil || len(entries) != 0 {
		return errors.New("owned stage must be empty")
	}
	st, err := os.Lstat(fontPath)
	if err != nil || !st.Mode().IsRegular() || st.Size() != fontSize || st.Size() > maxFontBytes {
		return errors.New("font input is missing, linked or outside exact byte bound")
	}
	f, err := os.Open(fontPath)
	if err != nil {
		return err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(st, opened) {
		return errors.New("font input changed before bounded read")
	}
	input, err := io.ReadAll(io.LimitReader(f, maxFontBytes+1))
	if err != nil {
		return err
	}
	closed, err := f.Stat()
	named, nameErr := os.Lstat(fontPath)
	if err != nil || nameErr != nil || len(input) != fontSize || !os.SameFile(st, closed) || !os.SameFile(st, named) || closed.Size() != int64(len(input)) {
		return errors.New("font input changed during bounded read")
	}
	if fmt.Sprintf("%x", sha256.Sum256(input)) != fontSHA256 {
		return errors.New("font original-byte digest differs")
	}
	pdf, err := buildCFFPilot(input)
	if err != nil {
		return err
	}
	if len(pdf) > maxPDFBytes {
		return errors.New("PDF output exceeds 16 MiB")
	}
	path := filepath.Join(outDir, "pilot.pdf")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	if _, err = f.Write(pdf); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func buildCFFPilot(input []byte) ([]byte, error) {
	original, err := font.ParseSFNT(input, faceIndex)
	if err != nil || original == nil || !original.IsCFF || original.IsTrueType || original.Tables["CFF "] == nil || original.Tables["CFF2"] != nil {
		return nil, errors.New("selected face is not the pinned CFF1 outline")
	}
	units := original.UnitsPerEm()
	if units == 0 {
		return nil, errors.New("font units per em unavailable")
	}
	runes := []rune(originalText)
	glyphIDs := []uint16{0} // CID zero is .notdef; the fixed sample starts at CID one.
	seen := map[uint16]bool{0: true}
	widths := make([]int, 0, len(runes))
	var xMin, yMin, xMax, yMax int16
	for i, r := range runes {
		gid := original.GlyphIndex(r)
		if gid == 0 || seen[gid] {
			return nil, errors.New("fixed text has a missing or ambiguous glyph")
		}
		seen[gid] = true
		glyphIDs = append(glyphIDs, gid)
		advance := original.GlyphAdvance(gid)
		if advance == 0 {
			return nil, errors.New("selected glyph has no advance")
		}
		widths = append(widths, int(advance)*1000/int(units))
		a, b, c, d := original.GlyphBounds(gid)
		if i == 0 || a < xMin {
			xMin = a
		}
		if i == 0 || b < yMin {
			yMin = b
		}
		if i == 0 || c > xMax {
			xMax = c
		}
		if i == 0 || d > yMax {
			yMax = d
		}
	}
	subset, err := original.Subset(glyphIDs, font.SubsetOptions{Tables: font.KeepPDFTables})
	if err != nil || subset == nil || !subset.IsCFF || subset.IsTrueType || subset.Tables["CFF "] == nil || subset.Tables["CFF2"] != nil {
		return nil, errors.New("CFF1 subset unavailable")
	}
	fontProgram := subset.Write()
	if len(fontProgram) < 12 || len(fontProgram) > maxSubsetBytes || len(input)+len(fontProgram) > maxCombinedBytes || string(fontProgram[:4]) != "OTTO" {
		return nil, errors.New("standalone CFF1 subset is invalid or oversized")
	}
	check, err := font.ParseSFNT(fontProgram, 0)
	if err != nil || check == nil || !check.IsCFF || check.Tables["CFF "] == nil || check.Tables["CFF2"] != nil || int(check.NumGlyphs()) != len(glyphIDs) {
		return nil, errors.New("standalone CFF1 subset failed structural reparse")
	}
	for i, r := range runes {
		if check.GlyphIndex(r) != uint16(i+1) {
			return nil, errors.New("subset cmap no longer binds original Unicode")
		}
	}
	var toUnicode strings.Builder
	toUnicode.WriteString("/CIDInit /ProcSet findresource begin\n12 dict begin\nbegincmap\n/CIDSystemInfo << /Registry (Adobe) /Ordering (UCS) /Supplement 0 >> def\n/CMapName /LuciPilot-UCS def\n/CMapType 2 def\n1 begincodespacerange\n<0000> <FFFF>\nendcodespacerange\n")
	fmt.Fprintf(&toUnicode, "%d beginbfchar\n", len(runes))
	for i, r := range runes {
		var code strings.Builder
		for _, u := range utf16.Encode([]rune{r}) {
			fmt.Fprintf(&code, "%04X", u)
		}
		fmt.Fprintf(&toUnicode, "<%04X> <%s>\n", i+1, code.String())
	}
	toUnicode.WriteString("endbfchar\nendcmap\nCMapName currentdict /CMap defineresource pop\nend\nend\n")
	if toUnicode.Len() > 4096 {
		return nil, errors.New("ToUnicode map exceeds fixed pilot bound")
	}
	return makePDF(fontProgram, []byte(toUnicode.String()), widths, [4]int{int(xMin), int(yMin), int(xMax), int(yMax)}, len(input))
}

func makePDF(fontProgram, cmap []byte, widths []int, box [4]int, originalFontBytes int) ([]byte, error) {
	content := []byte("BT\n/F0 24 Tf\n70 700 Td\n<000100020003> Tj\nET\n")
	stream := func(payload []byte) []byte {
		var b bytes.Buffer
		fmt.Fprintf(&b, "<< /Length %d >>\nstream\n", len(payload))
		b.Write(payload)
		b.WriteString("\nendstream\n")
		return b.Bytes()
	}
	widthText := make([]string, len(widths))
	for i, width := range widths {
		widthText[i] = fmt.Sprint(width)
	}
	objects := [][]byte{
		[]byte("<< /Type /Catalog /Pages 2 0 R >>\n"),
		[]byte("<< /Type /Pages /Kids [3 0 R] /Count 1 >>\n"),
		[]byte("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F0 4 0 R >> >> /Contents 8 0 R >>\n"),
		[]byte("<< /Type /Font /Subtype /Type0 /BaseFont /LuciPilot+CJK /Encoding /Identity-H /DescendantFonts [5 0 R] /ToUnicode 7 0 R >>\n"),
		[]byte(fmt.Sprintf("<< /Type /Font /Subtype /CIDFontType0 /BaseFont /LuciPilot+CJK /CIDSystemInfo << /Registry (Adobe) /Ordering (Identity) /Supplement 0 >> /FontDescriptor 6 0 R /DW 1000 /W [1 [%s]] >>\n", strings.Join(widthText, " "))),
		[]byte(fmt.Sprintf("<< /Type /FontDescriptor /FontName /LuciPilot+CJK /Flags 4 /FontBBox [%d %d %d %d] /ItalicAngle 0 /Ascent %d /Descent %d /CapHeight %d /StemV 80 /FontFile3 9 0 R >>\n", box[0], box[1], box[2], box[3], box[3], box[1], box[3])),
		stream(cmap),
		stream(content),
	}
	var out bytes.Buffer
	out.WriteString("%PDF-1.7\n%\xE2\xE3\xCF\xD3\n")
	offsets := make([]int, 10)
	for i, obj := range objects {
		offsets[i+1] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n", i+1)
		out.Write(obj)
		out.WriteString("endobj\n")
		if out.Len() > maxPDFBytes {
			return nil, errors.New("PDF object stream exceeds 16 MiB")
		}
	}
	// Keep a single subset buffer and write it straight into the bounded PDF
	// buffer; a second full font-stream copy would defeat the intermediate cap.
	offsets[9] = out.Len()
	fmt.Fprintf(&out, "9 0 obj\n<< /Length %d /Subtype /OpenType >>\nstream\n", len(fontProgram))
	out.Write(fontProgram)
	out.WriteString("\nendstream\nendobj\n")
	if out.Len() > maxPDFBytes || originalFontBytes+len(fontProgram)+out.Len() > maxCombinedBytes {
		return nil, errors.New("font input, CFF subset and PDF bytes exceed 32 MiB aggregate bound")
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&out, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xref)
	if out.Len() > maxPDFBytes || originalFontBytes+len(fontProgram)+out.Len() > maxCombinedBytes {
		return nil, errors.New("completed PDF or font/subset/PDF aggregate exceeds byte bound")
	}
	return out.Bytes(), nil
}
