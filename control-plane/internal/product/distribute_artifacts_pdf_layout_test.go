package product

import (
	"bytes"
	"context"
	"encoding/binary"
	"strings"
	"testing"

	subsetfont "github.com/tdewolff/font"
)

func TestDistributePDFCIDCharsetAndFDSelectionAreExplicit(t *testing.T) {
	data:=[]byte{0,0,0,0,0x07,0xd0,0x00,0x19}
	cids,err:=cffCIDCharset(data,3,3)
	if err!=nil||cids[1]!=2000||cids[2]!=25{t.Fatal("original CIDs were confused with subset GIDs",cids,err)}
	if _,err=cffCIDCharset([]byte{0,0,0,0,0,1,0,1},3,3);err==nil{t.Fatal("duplicate CIDs accepted")}
	selection,err:=cffFDSelect([]byte{3,0,2,0,0,1,0,2,0,0,4},0,4,2)
	if err!=nil||string(selection)!=string([]byte{1,1,0,0}){t.Fatal("private dictionary selection changed",selection,err)}
	if _,err=cffFDSelect([]byte{0,2},0,1,2);err==nil{t.Fatal("unknown private dictionary accepted")}
}

func TestDistributePDFSubsetAndClusterCodesKeepSeparateIdentities(t *testing.T) {
	fonts,err:=openPDFRenderFonts(context.Background());if err!=nil{t.Fatal(err)}
	budget:=&pdfBudget{}
	gid:=uint32(fonts[0].sfnt.GlyphIndex('f'))
	first,err:=fonts[0].code(gid,"f",budget);if err!=nil{t.Fatal(err)}
	second,err:=fonts[0].code(gid,"different original cluster",budget);if err!=nil||first==second{t.Fatal("different clusters reused a character code",err)}
	if err=fonts[0].subset(context.Background(),budget);err!=nil{t.Fatal(err)}
	ttf,err:=subsetfont.ParseEmbeddedSFNT(fonts[0].program,0);if err!=nil||!bytes.Equal(ttf.Tables["name"],pdfSubsetNameTable(fonts[0].subsetName)){t.Fatal("TrueType subset name differs from PDF font name",err)}
	if fonts[0].codeToSubset[first]!=fonts[0].codeToSubset[second]{t.Fatal("same glyph duplicated by Unicode mapping")}
	encoding,unicodeMap,err:=pdfCharacterMaps(context.Background(),fonts[0]);if err!=nil||!strings.Contains(string(encoding),"begincidchar")||!strings.Contains(string(unicodeMap),pdfUTF16Hex("different original cluster",false)){t.Fatal("original Unicode mapping missing",err)}
	cjk:=fonts[2]
	for _,r:=range "日本語"{if _,err=cjk.code(uint32(cjk.sfnt.GlyphIndex(r)),string(r),budget);err!=nil{t.Fatal(err)}}
	if err=cjk.subset(context.Background(),budget);err!=nil{t.Fatal(err)}
	if string(cjk.program[:4])!="OTTO"||binary.BigEndian.Uint16(cjk.program[4:])<6{t.Fatal("CFF subset is not a standalone SFNT")}
	otf,err:=subsetfont.ParseSFNT(cjk.program,0);if err!=nil{t.Fatal(err)}
	names,_,err:=cffIndex(otf.Tables["CFF "],4)
	if err!=nil||len(names)!=1||string(names[0])!=cjk.subsetName||!bytes.Equal(otf.Tables["name"],pdfSubsetNameTable(cjk.subsetName)){t.Fatal("CFF subset name differs from PDF font name",err)}
}

func TestDistributePDFLayoutContinuesWithoutDroppingRows(t *testing.T) {
	fonts,err:=openPDFRenderFonts(context.Background());if err!=nil{t.Fatal(err)}
	layout:=&pdfLayout{ctx:context.Background(),fonts:fonts,budget:&pdfBudget{}}
	value:=strings.Repeat("Saved evidence reference with café and 日本語. ",130)+"brief:4:"+strings.Repeat("a",64)
	if err=layout.row("Source reference",value,false);err!=nil{t.Fatal(err)}
	if len(layout.pages)<2{t.Fatal("long row did not continue onto another page")}
	var content strings.Builder;for _,page:=range layout.pages{content.WriteString(page.content.String())}
	if !strings.Contains(pdfLogicalText(t,[]byte(content.String())),value){t.Fatal("continued row lost source text")}
	if layout.y<40{t.Fatal("layout crossed printable page boundary")}
	if err=layout.budget.take(pdfIntermediateLimit);err==nil{t.Fatal("intermediate bound ignored")}
}
