package product

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/benoitkugler/pdf/model"
)

// Only a fully completed in-memory document reaches the HTTP response. The
// writer owns xref/object/stream serialization; this package supplies bounded
// shapes, explicit code maps and font programs, with no actions or references.
func artifactPDF(ctx context.Context, lines []artifactExportLine) ([]byte,error) {
	if err:=ctx.Err();err!=nil{return nil,err}
	if len(lines)==0||len(lines)>16384{return nil,errors.New("invalid PDF source line count")}
	textBytes:=0
	for _,line:=range lines {
		if !validArtifactXMLText(line.Label)||!validArtifactXMLText(line.Value)||len(line.Label)>8192||len(line.Value)>256<<10{return nil,errors.New("invalid PDF source text")}
		textBytes+=len(line.Label)+len(line.Value)
		if textBytes>256<<10{return nil,errors.New("PDF source exceeds limit")}
	}
	fonts,err:=openPDFRenderFonts(ctx);if err!=nil{return nil,err}
	budget:=&pdfBudget{}
	layout:=&pdfLayout{ctx:ctx,fonts:fonts,budget:budget}
	for i,line:=range lines {if err=layout.row(line.Label,line.Value,i==0&&line.Label=="Title");err!=nil{return nil,err}}
	if len(layout.pages)==0{return nil,errors.New("PDF has no pages")}
	// Footer text uses the same checked shaping/coverage path. No drawing
	// operation is added after the font subset and its maps have been frozen.
	for i,page:=range layout.pages {
		footer,e:=layout.shape(fmt.Sprintf("Page %d of %d",i+1,len(layout.pages)),8,515)
		if e!=nil||len(footer)!=1{return nil,errors.New("PDF footer cannot fit")}
		saved:=layout.pages;layout.pages=[]*pdfPage{page}
		e=layout.paintRole(footer[0],40,20,true)
		layout.pages=saved
		if e!=nil{return nil,e}
	}
	resources:=model.NewResourcesDict()
	for i,font:=range fonts {
		if err=font.subset(ctx,budget);err!=nil{return nil,err}
		if len(font.codes)==1{continue}
		dictionary,e:=pdfFontDictionary(ctx,font,budget)
		if e!=nil{return nil,e}
		resources.Font[model.Name(fmt.Sprintf("F%d",i))]=dictionary
	}
	document:=model.Document{}
	document.Trailer.Info.Producer="Luci private artifact export"
	if lines[0].Label=="Title"{document.Trailer.Info.Title=lines[0].Value}
	document.Catalog.Pages.Resources=&resources
	box:=model.Rectangle{Llx:0,Lly:0,Urx:595,Ury:842}
	document.Catalog.Pages.MediaBox=&box
	for _,page:=range layout.pages {
		if err=ctx.Err();err!=nil{return nil,err}
		if err=budget.take(page.content.Len());err!=nil{return nil,err}
		stream:=model.ContentStream{Stream:model.Stream{Content:[]byte(page.content.String())}}
		document.Catalog.Pages.Kids=append(document.Catalog.Pages.Kids,&model.PageObject{Contents:[]model.ContentStream{stream}})
	}
	output:=&pdfOutput{ctx:ctx,data:make([]byte,0,distributeExportLimit)}
	if err=document.Write(output,nil);err!=nil{return nil,err}
	if err=ctx.Err();err!=nil{return nil,err}
	if output.err!=nil{return nil,output.err}
	return output.data,nil
}

type pdfOutput struct {ctx context.Context;data []byte;err error}
func (w *pdfOutput) Write(p []byte)(int,error){
	if w.err!=nil{return 0,w.err}
	if err:=w.ctx.Err();err!=nil{w.err=err;return 0,err}
	if len(p)>distributeExportLimit-len(w.data){w.err=errors.New("PDF output exceeds 16 MiB");return 0,w.err}
	w.data=append(w.data,p...)
	return len(p),nil
}

func pdfFontDictionary(ctx context.Context,font *pdfRenderFont,budget *pdfBudget)(*model.FontDict,error){
	if font.subsetName==""{return nil,errors.New("PDF subset name unavailable")}
	name:=model.Name(font.subsetName)
	units:=float64(font.sfnt.UnitsPerEm())
	widths:=map[uint16]model.Fl{}
	var xMin,yMin,xMax,yMax int16
	first:=true
	for code,glyph:=range font.codes[1:] {
		if err:=ctx.Err();err!=nil{return nil,err}
		cid:=font.codeToSubset[code+1]
		widths[cid]=model.Fl(float64(font.sfnt.GlyphAdvance(glyph.gid))*1000/units)
		a,b,c,d:=font.sfnt.GlyphBounds(glyph.gid)
		if first||a<xMin{xMin=a};if first||b<yMin{yMin=b};if first||c>xMax{xMax=c};if first||d>yMax{yMax=d};first=false
	}
	cidWidths:=make([]model.CIDWidth,0,len(widths))
	for cid:=1;cid<65536;cid++ {if width,ok:=widths[uint16(cid)];ok{cidWidths=append(cidWidths,model.CIDWidthRange{First:model.CID(cid),Last:model.CID(cid),Width:width})}}
	fontFile:=&model.FontFile{Stream:model.Stream{Content:font.program},Length1:len(font.program)}
	subtype:=model.Name("CIDFontType2")
	var gidMap model.CIDToGIDMap=model.CIDToGIDMapIdentity{}
	if font.sfnt.IsCFF {fontFile.Subtype="OpenType";subtype="CIDFontType0";gidMap=nil}
	scale:=func(n int16)model.Fl{return model.Fl(float64(n)*1000/units)}
	descendant:=model.CIDFontDictionary{
		Subtype:subtype,BaseFont:name,CIDSystemInfo:model.CIDSystemInfo{Registry:"Adobe",Ordering:"Identity",Supplement:0},
		FontDescriptor:model.FontDescriptor{FontName:name,Flags:4,FontBBox:model.Rectangle{Llx:scale(xMin),Lly:scale(yMin),Urx:scale(xMax),Ury:scale(yMax)},Ascent:scale(yMax),Descent:scale(yMin),CapHeight:scale(yMax),StemV:80,FontFile:fontFile},
		W:cidWidths,CIDToGIDMap:gidMap,
	}
	encoding,unicodeMap,err:=pdfCharacterMaps(ctx,font)
	if err!=nil{return nil,err}
	if err=budget.take(len(encoding)+len(unicodeMap)+len(cidWidths)*32);err!=nil{return nil,err}
	return &model.FontDict{
		Subtype:model.FontType0{BaseFont:name,Encoding:model.CMapEncodingEmbedded{Stream:model.Stream{Content:encoding},CMapName:model.Name("LuciEncoding"+font.subsetName),CIDSystemInfo:descendant.CIDSystemInfo},DescendantFonts:descendant},
		ToUnicode:&model.UnicodeCMap{Stream:model.Stream{Content:unicodeMap}},
	},nil
}

func pdfCharacterMaps(ctx context.Context,font *pdfRenderFont)([]byte,[]byte,error){
	if font.subsetName==""||len(font.codes)<2||len(font.codes)!=len(font.codeToSubset)||len(font.codes)>65536{return nil,nil,errors.New("invalid PDF code map")}
	var encoding,unicodeMap strings.Builder
	const begin="/CIDInit /ProcSet findresource begin\n12 dict begin\nbegincmap\n/CIDSystemInfo << /Registry (Adobe) /Ordering (Identity) /Supplement 0 >> def\n"
	encoding.WriteString(begin+"/CMapName /LuciEncoding"+font.subsetName+" def\n/CMapType 1 def\n1 begincodespacerange\n<0000> <FFFF>\nendcodespacerange\n")
	unicodeMap.WriteString(begin+"/CMapName /LuciUnicode"+font.subsetName+" def\n/CMapType 2 def\n1 begincodespacerange\n<0000> <FFFF>\nendcodespacerange\n")
	for start:=1;start<len(font.codes);start+=100 {
		if err:=ctx.Err();err!=nil{return nil,nil,err}
		end:=min(start+100,len(font.codes))
		fmt.Fprintf(&encoding,"%d begincidchar\n",end-start)
		mapped:=0;for i:=start;i<end;i++{if font.codes[i].text!=""{mapped++}}
		if mapped>0{fmt.Fprintf(&unicodeMap,"%d beginbfchar\n",mapped)}
		for i:=start;i<end;i++{
			fmt.Fprintf(&encoding,"<%04X> %d\n",i,font.codeToSubset[i])
			// A multi-glyph cluster is represented once. ActualText encloses
			// the complete logical line, including formatting controls; later
			// glyphs do not repeat the cluster's Unicode in ToUnicode.
			if font.codes[i].text!=""{fmt.Fprintf(&unicodeMap,"<%04X> <%s>\n",i,pdfUTF16Hex(font.codes[i].text,false))}
		}
		encoding.WriteString("endcidchar\n");if mapped>0{unicodeMap.WriteString("endbfchar\n")}
		if encoding.Len()+unicodeMap.Len()>pdfIntermediateLimit{return nil,nil,errors.New("PDF CMap exceeds limit")}
	}
	const end="endcmap\nCMapName currentdict /CMap defineresource pop\nend\nend\n"
	encoding.WriteString(end);unicodeMap.WriteString(end)
	return []byte(encoding.String()),[]byte(unicodeMap.String()),nil
}
