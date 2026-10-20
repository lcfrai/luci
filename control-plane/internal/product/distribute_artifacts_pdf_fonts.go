package product

import (
	"bytes"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"sync"

	shapefont "github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/font/opentype"
	subsetfont "github.com/tdewolff/font"
)

// These inputs belong to the release, never to an export request or a host-font
// lookup. Root-owned originals and grants must be present before compilation.
//go:embed distribute_pdf_fonts/*
var artifactPDFFontFiles embed.FS

type pdfFontRecord struct {
	ID string `json:"id"`
	File string `json:"file"`
	Bytes int `json:"bytes"`
	SHA256 string `json:"sha256"`
	FaceIndex int `json:"faceIndex"`
	Outline string `json:"outline"`
	License string `json:"license"`
}

type pdfNoticeRecord struct {
	File string `json:"file"`
	Bytes int `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type pdfAssetManifest struct {
	Schema string `json:"schema"`
	Fonts []pdfFontRecord `json:"fonts"`
	Notices []pdfNoticeRecord `json:"notices"`
}

type pdfFontAsset struct {
	record pdfFontRecord
	data []byte
}

var pdfAssetOnce sync.Once
var pdfFontAssets []pdfFontAsset
var pdfAssetError error

func artifactPDFAssetsAvailable() bool {
	pdfAssetOnce.Do(func() { pdfFontAssets, pdfAssetError = loadPDFAssets() })
	return pdfAssetError == nil && len(pdfFontAssets) == 3
}

func pdfAssetName(s string) bool {
	return s != "" && s != "." && s != ".." && path.Base(s) == s && !strings.ContainsAny(s, "\\\x00\r\n")
}

func loadPDFAssets() ([]pdfFontAsset, error) {
	raw, err := artifactPDFFontFiles.ReadFile("distribute_pdf_fonts/manifest.json")
	if err != nil || len(raw) > 64<<10 { return nil, errors.New("PDF font manifest unavailable") }
	var manifest pdfAssetManifest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&manifest); err != nil { return nil, err }
	var extra any
	if decoder.Decode(&extra) != io.EOF || manifest.Schema != "luci.distribute.pdf-fonts.v1" || len(manifest.Fonts) != 3 || len(manifest.Notices) < 1 || len(manifest.Notices) > 8 { return nil, errors.New("invalid PDF font manifest") }
	seen := map[string]bool{"manifest.json":true}
	roles := map[string]bool{}
	total := 0
	assets := make(map[string]pdfFontAsset, 3)
	readExact := func(name string, size int, digest string, limit int) ([]byte, error) {
		if !pdfAssetName(name) || seen[name] || size < 1 || size > limit || !hexDigest(digest) { return nil, errors.New("invalid PDF asset record") }
		seen[name] = true
		data, e := artifactPDFFontFiles.ReadFile("distribute_pdf_fonts/"+name)
		if e != nil || len(data) != size || fmt.Sprintf("%x", sha256.Sum256(data)) != digest { return nil, errors.New("PDF asset identity mismatch") }
		return data, nil
	}
	for _, record := range manifest.Fonts {
		if record.ID != "latin" && record.ID != "arabic" && record.ID != "cjk" || roles[record.ID] || record.FaceIndex < 0 || record.FaceIndex > 9 || record.Outline != "truetype" && record.Outline != "cff1" || record.License != "OFL-1.1" { return nil, errors.New("invalid PDF font role") }
		roles[record.ID] = true
		if record.Bytes < 1 || record.Bytes > pdfFontLimit-total { return nil, errors.New("PDF font bundle exceeds limit") }
		data, e := readExact(record.File, record.Bytes, record.SHA256, pdfFontLimit)
		if e != nil { return nil, e }
		total += len(data)
		assets[record.ID] = pdfFontAsset{record:record, data:data}
	}
	for _, notice := range manifest.Notices {
		if _, e := readExact(notice.File, notice.Bytes, notice.SHA256, 256<<10); e != nil { return nil, e }
	}
	entries, err := artifactPDFFontFiles.ReadDir("distribute_pdf_fonts")
	if err != nil || len(entries) != len(seen) { return nil, errors.New("unexpected PDF bundle file") }
	for _, entry := range entries { if entry.IsDir() || !seen[entry.Name()] { return nil, errors.New("unexpected PDF bundle entry") } }
	return []pdfFontAsset{assets["latin"],assets["arabic"],assets["cjk"]}, nil
}

type pdfGlyphKey struct { gid uint16; text string }
type pdfCode struct { gid uint16; text string }
type pdfRenderFont struct {
	asset pdfFontAsset
	face *shapefont.Face
	sfnt *subsetfont.SFNT
	codes []pdfCode // code zero is reserved
	codeByGlyph map[pdfGlyphKey]uint16
	program []byte
	subsetName string
	codeToSubset []uint16
}

func openPDFRenderFonts(ctx context.Context) ([]*pdfRenderFont, error) {
	if !artifactPDFAssetsAvailable() { return nil, errors.New("PDF font assets are unavailable") }
	fonts := make([]*pdfRenderFont, 0, 3)
	for _, asset := range pdfFontAssets {
		if err := ctx.Err(); err != nil { return nil, err }
		loaders, err := opentype.NewLoaders(bytes.NewReader(asset.data))
		if err != nil || asset.record.FaceIndex >= len(loaders) { return nil, errors.New("PDF font face unavailable") }
		font, err := shapefont.NewFont(loaders[asset.record.FaceIndex])
		if err != nil { return nil, err }
		sfnt, err := subsetfont.ParseSFNT(asset.data, asset.record.FaceIndex)
		if err != nil || sfnt == nil || sfnt.UnitsPerEm() == 0 || sfnt.Tables["CFF2"] != nil || (asset.record.Outline == "truetype") != sfnt.IsTrueType || (asset.record.Outline == "cff1") != sfnt.IsCFF { return nil, errors.New("PDF font outline mismatch") }
		fonts = append(fonts, &pdfRenderFont{asset:asset,face:shapefont.NewFace(font),sfnt:sfnt,codes:[]pdfCode{{}},codeByGlyph:make(map[pdfGlyphKey]uint16)})
	}
	return fonts, nil
}

func (f *pdfRenderFont) code(gid uint32, text string, budget *pdfBudget) (uint16, error) {
	if gid == 0 || gid > 65535 || int(gid) >= int(f.sfnt.NumGlyphs()) { return 0, errors.New("PDF glyph unavailable") }
	key := pdfGlyphKey{uint16(gid),text}
	if code, ok := f.codeByGlyph[key]; ok { return code, nil }
	if len(f.codes) >= 65536 { return 0, errors.New("PDF character-code limit exceeded") }
	if err := budget.take(96+len(text)*2); err != nil { return 0, err }
	code := uint16(len(f.codes))
	f.codes = append(f.codes, pdfCode{uint16(gid),text})
	f.codeByGlyph[key] = code
	return code, nil
}

func (f *pdfRenderFont) subset(ctx context.Context, budget *pdfBudget) error {
	if len(f.codes) == 1 { return nil }
	gids := []uint16{0}
	indices := map[uint16]uint16{0:0}
	f.codeToSubset = make([]uint16,len(f.codes))
	for code, glyph := range f.codes[1:] {
		if err := ctx.Err(); err != nil { return err }
		index, ok := indices[glyph.gid]
		if !ok { index=uint16(len(gids)); indices[glyph.gid]=index; gids=append(gids,glyph.gid) }
		f.codeToSubset[code+1]=index
	}
	var err error
	// The PDF BaseFont, embedded name table and CFF Name INDEX use one name.
	// Name it before serialization, avoiding a hash/name circular dependency.
	identity:=append([]byte(f.asset.record.SHA256),0)
	for _,gid:=range gids {identity=append(identity,byte(gid>>8),byte(gid))}
	f.subsetName=pdfDigestPrefix(identity)+"+LuciPDF"+f.asset.record.ID
	if f.sfnt.IsCFF {
		f.program, err = subsetPDFCIDFont(ctx, f.sfnt, gids, f.subsetName, budget)
	} else {
		var subset *subsetfont.SFNT
		subset, err = f.sfnt.Subset(gids, subsetfont.SubsetOptions{Tables:subsetfont.KeepPDFTables})
		if err == nil && subset != nil { subset.Tables["name"]=pdfSubsetNameTable(f.subsetName);f.program=subset.Write() } else if err == nil { err=errors.New("PDF subset unavailable") }
	}
	if err != nil { return err }
	if err = ctx.Err(); err != nil { return err }
	if len(f.program) < 12 || len(f.program) > distributeExportLimit { return errors.New("PDF font subset exceeds limit") }
	if !f.sfnt.IsCFF { if err = budget.take(len(f.program)); err != nil { return err } }
	var check *subsetfont.SFNT
	// Embedded CFF parsing intentionally skips hmtx/maxp in this library; our
	// standalone OpenType CFF has the complete required tables, so parse those.
	if f.sfnt.IsCFF {check,err=subsetfont.ParseSFNT(f.program,0)} else {check,err=subsetfont.ParseEmbeddedSFNT(f.program,0)}
	if err != nil || check == nil || int(check.NumGlyphs()) < len(gids) || check.IsCFF != f.sfnt.IsCFF { return errors.New("PDF subset structure changed") }
	for old, index := range indices {
		if check.GlyphAdvance(index) != f.sfnt.GlyphAdvance(old) { return errors.New("PDF subset advance mismatch") }
	}
	return nil
}

// A format-zero name table with Windows Unicode family/full/PostScript names.
// Subset names are private fixed ASCII, never request text or a host font name.
func pdfSubsetNameTable(name string) []byte {
	encoded:=make([]byte,len(name)*2)
	for i:=range name {binary.BigEndian.PutUint16(encoded[2*i:],uint16(name[i]))}
	out:=make([]byte,42+len(encoded))
	binary.BigEndian.PutUint16(out[2:],3);binary.BigEndian.PutUint16(out[4:],42)
	for i,id:=range []uint16{1,4,6} {
		at:=6+12*i;binary.BigEndian.PutUint16(out[at:],3);binary.BigEndian.PutUint16(out[at+2:],1);binary.BigEndian.PutUint16(out[at+4:],0x409)
		binary.BigEndian.PutUint16(out[at+6:],id);binary.BigEndian.PutUint16(out[at+8:],uint16(len(encoded)))
	}
	copy(out[42:],encoded)
	return out
}

func pdfUTF16Hex(s string, bom bool) string {
	var b strings.Builder
	if bom { b.WriteString("FEFF") }
	for _, r := range s {
		if r <= 0xffff { fmt.Fprintf(&b,"%04X",r) } else { r-=0x10000;fmt.Fprintf(&b,"%04X%04X",0xd800+(r>>10),0xdc00+(r&1023)) }
	}
	return b.String()
}

func pdfDigestPrefix(data []byte) string {
	digest:=sha256.Sum256(data)
	var prefix [6]byte
	for i:=range prefix { prefix[i]='A'+digest[i]%26 }
	return string(prefix[:])
}
