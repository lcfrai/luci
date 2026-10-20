package product

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/go-text/typesetting/di"
	shapefont "github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/shaping"
	"golang.org/x/image/math/fixed"
)

const (
	pdfFontLimit = 32 << 20
	pdfIntermediateLimit = 32 << 20
	pdfPageLimit = 512
)

// This debit bounds retained plans and byte buffers; third-party internal
// allocations are covered by host admission, not claimed to be measured here.
type pdfBudget struct { used int }
func (b *pdfBudget) take(n int) error {
	if n < 0 || n > pdfIntermediateLimit-b.used { return errors.New("PDF intermediate limit exceeded") }
	b.used += n
	return nil
}

type pdfPlacedGlyph struct { font int; code uint16; x,y,size float64 }
type pdfTextLine struct { text string; glyphs []pdfPlacedGlyph; ascent,descent,gap,width float64 }
type pdfPage struct { content strings.Builder }

type pdfLayout struct {
	ctx context.Context
	fonts []*pdfRenderFont
	budget *pdfBudget
	segmenter shaping.Segmenter
	shaper shaping.HarfbuzzShaper
	pages []*pdfPage
	y float64
}

// Missing coverage is latched independently: the library's Fontmap contract
// requires a nonnil fallback even for an unsupported rune. That fallback is
// never allowed to become an accepted .notdef glyph or a host-font lookup.
type pdfFaceMap struct { fonts []*pdfRenderFont; missing bool }
func (m *pdfFaceMap) ResolveFace(r rune) *shapefont.Face {
	for _, font := range m.fonts { if _,ok:=font.face.NominalGlyph(r); ok { return font.face } }
	if !unicode.IsControl(r) && r != '\u200c' && r != '\u200d' && !(r >= '\u202a' && r <= '\u202e') && !(r >= '\u2066' && r <= '\u2069') { m.missing=true }
	return m.fonts[0].face
}

func (l *pdfLayout) shape(text string, size, width float64) ([]pdfTextLine,error) {
	if err:=l.ctx.Err();err!=nil { return nil,err }
	if !utf8.ValidString(text) || !validArtifactXMLText(text) || len(text)>256<<10 || size<6 || size>24 || width<20 || width>515 { return nil,errors.New("invalid PDF text block") }
	if text=="" { return []pdfTextLine{{ascent:size,descent:-size*.25,gap:3}},nil }
	runes:=[]rune(text)
	if err:=l.budget.take(len(runes)*4);err!=nil { return nil,err }
	faces:=&pdfFaceMap{fonts:l.fonts}
	input:=shaping.Input{Text:runes,RunStart:0,RunEnd:len(runes),Direction:di.DirectionLTR,Size:fixed.Int26_6(math.Round(size*64))}
	runs:=l.segmenter.Split(input,faces)
	if faces.missing { return nil,errors.New("PDF font coverage is incomplete") }
	outputs:=make([]shaping.Output,0,len(runs))
	for _,run:=range runs {
		if err:=l.ctx.Err();err!=nil { return nil,err }
		if run.RunStart<0 || run.RunEnd<=run.RunStart || run.RunEnd>len(runes) { return nil,errors.New("invalid PDF shaping run") }
		output:=l.shaper.Shape(run)
		for i,glyph:=range output.Glyphs {
			at,n:=glyph.TextIndex(),glyph.RunesCount()
			if at<0||n<1||at+n>len(runes){return nil,errors.New("invalid shaped PDF cluster")}
			if n==1 && runes[at]=='\t' {
				output.Glyphs[i].GlyphID=shapefont.EmptyGlyph
				output.Glyphs[i].Advance=fixed.Int26_6(math.Round(size*2*64))
				output.Glyphs[i].Width=0
				output.Glyphs[i].Height=0
			}
		}
		output.RecomputeAdvance()
		if err:=l.budget.take(len(output.Glyphs)*96+128);err!=nil { return nil,err }
		outputs=append(outputs,output)
	}
	var wrapper shaping.LineWrapper
	// Long saved digests and URLs must wrap at grapheme boundaries when no
	// word boundary fits; refusing them would make ordinary evidence unexportable.
	wrapper.Prepare(shaping.WrapConfig{Direction:di.DirectionLTR,BreakPolicy:shaping.WhenNecessary,DisableTrailingWhitespaceTrim:true},runes,shaping.NewSliceIterator(outputs))
	var result []pdfTextLine
	start:=0
	for count:=0;;count++ {
		if err:=l.ctx.Err();err!=nil { return nil,err }
		if count>=pdfPageLimit*80 { return nil,errors.New("PDF line count exceeded") }
		wrapped,done:=wrapper.WrapNextLineF(fixed.Int26_6(math.Round(width*64)))
		if wrapped.Truncated!=0 || wrapped.NextLine<=start || wrapped.NextLine>len(runes) { return nil,errors.New("PDF line lost source text") }
		line:=pdfTextLine{text:string(runes[start:wrapped.NextLine]),ascent:size,descent:-size*.25,gap:3}
		// Retain logical order for source extraction, but determine every run's
		// physical position from its separately computed visual index.
		visual:=append(shaping.Line(nil),wrapped.Line...)
		sort.Slice(visual,func(i,j int)bool{return visual[i].VisualIndex<visual[j].VisualIndex})
		xByVisual:=make(map[int32]float64,len(visual))
		pen:=0.0
		for index,run:=range visual {
			if run.VisualIndex!=int32(index) || run.Advance<0 { return nil,errors.New("invalid PDF visual run") }
			xByVisual[run.VisualIndex]=pen
			pen+=float64(run.Advance)/64
		}
		if pen>width+.01 { return nil,errors.New("PDF text cannot fit without clipping") }
		covered:=make([]bool,wrapped.NextLine-start)
		minX,maxX:=0.0,pen
		for _,run:=range wrapped.Line {
			fontIndex:=-1
			for index,font:=range l.fonts { if font.face==run.Face {fontIndex=index;break} }
			if fontIndex<0 || run.Runes.Offset<start || run.Runes.Offset+run.Runes.Count>wrapped.NextLine { return nil,errors.New("PDF run identity changed") }
			for i:=run.Runes.Offset;i<run.Runes.Offset+run.Runes.Count;i++ { if covered[i-start] {return nil,errors.New("overlapping PDF source runs")};covered[i-start]=true }
			line.ascent=math.Max(line.ascent,float64(run.LineBounds.Ascent)/64)
			line.descent=math.Min(line.descent,float64(run.LineBounds.Descent)/64)
			line.gap=math.Max(line.gap,float64(run.LineBounds.Gap)/64)
			x:=xByVisual[run.VisualIndex]
			seenClusters:=make(map[int]bool)
			for _,glyph:=range run.Glyphs {
				at,n:=glyph.TextIndex(),glyph.RunesCount()
				if at<start || n<1 || n>wrapped.NextLine-at { return nil,errors.New("invalid PDF Unicode cluster") }
				cluster:=string(runes[at:at+n])
				advance:=float64(glyph.Advance)/64
				if advance<0 { return nil,errors.New("invalid horizontal PDF advance") }
				if glyph.GlyphID==shapefont.EmptyGlyph { x+=advance;continue }
				if glyph.GlyphID==0 {
					// Formatting controls remain in ActualText but have no painted
					// glyph. Tabs get a fixed four-space layout advance.
					controls:=true
					for _,r:=range cluster { if r!='\t' && r!='\n' && r!='\r' && r!='\u200c' && r!='\u200d' && !(r>='\u202a'&&r<='\u202e') && !(r>='\u2066'&&r<='\u2069') {controls=false} }
					if !controls { return nil,errors.New("missing PDF glyph") }
					if strings.ContainsRune(cluster,'\t') { advance=math.Max(advance,size*2) }
					x+=advance;continue
				}
				original:=""
				if !seenClusters[at] { original=cluster;seenClusters[at]=true }
				code,err:=l.fonts[fontIndex].code(uint32(glyph.GlyphID),original,l.budget)
				if err!=nil { return nil,err }
				gx,gy:=x+float64(glyph.XOffset)/64,float64(glyph.YOffset)/64
				minX=math.Min(minX,gx+float64(glyph.XBearing)/64)
				maxX=math.Max(maxX,gx+float64(glyph.XBearing+glyph.Width)/64)
				line.ascent=math.Max(line.ascent,gy+float64(glyph.YBearing)/64)
				line.descent=math.Min(line.descent,gy+float64(glyph.YBearing+glyph.Height)/64)
				line.glyphs=append(line.glyphs,pdfPlacedGlyph{font:fontIndex,code:code,x:gx,y:gy,size:size})
				x+=advance
			}
			maxX=math.Max(maxX,x)
		}
		for _,present:=range covered { if !present {return nil,errors.New("PDF omitted source runes")} }
		if maxX-minX>width+.01 { return nil,errors.New("PDF ink exceeds available width") }
		for i:=range line.glyphs { line.glyphs[i].x-=minX }
		line.width=maxX-minX
		if err:=l.budget.take(len(line.glyphs)*48+len(line.text)+128);err!=nil { return nil,err }
		result=append(result,line)
		start=wrapped.NextLine
		if done { break }
	}
	if start!=len(runes) {return nil,errors.New("incomplete PDF paragraph")}
	return result,nil
}

func (l *pdfLayout) page() error {
	if len(l.pages)>=pdfPageLimit { return errors.New("PDF page limit exceeded") }
	if err:=l.budget.take(256);err!=nil { return err }
	l.pages=append(l.pages,&pdfPage{})
	l.y=802
	return nil
}

func (l *pdfLayout) paint(line pdfTextLine,x,baseline float64) error {
	return l.paintRole(line,x,baseline,false)
}

func (l *pdfLayout) paintRole(line pdfTextLine,x,baseline float64,decoration bool) error {
	if err:=l.ctx.Err();err!=nil { return err }
	var text strings.Builder
	if decoration {text.WriteString("/Artifact BMC\n")} else {fmt.Fprintf(&text,"/Span << /ActualText <%s> >> BDC\n",pdfUTF16Hex(line.text,true))}
	for _,g:=range line.glyphs { fmt.Fprintf(&text,"BT /F%d %.4f Tf 1 0 0 1 %.4f %.4f Tm <%04X> Tj ET\n",g.font,g.size,x+g.x,baseline+g.y,g.code) }
	text.WriteString("EMC\n")
	if err:=l.budget.take(text.Len());err!=nil { return err }
	l.pages[len(l.pages)-1].content.WriteString(text.String())
	return nil
}

func (l *pdfLayout) row(label,value string,title bool) error {
	size,leftWidth,rightX,rightWidth:=10.0,118.0,170.0,385.0
	if title {size=18;leftWidth=515;rightX=40;rightWidth=515}
	var left,right []pdfTextLine
	var err error
	if title { right,err=l.shape(label+": "+value,size,rightWidth) } else {
		left,err=l.shape(label+":",9,leftWidth)
		if err==nil { right,err=l.shape(value,size,rightWidth) }
	}
	if err!=nil { return err }
	if !title && (len(left)>1 || strings.HasPrefix(label,"Section: ")) {
		// Long section headings form their own full-width block; they must
		// not interleave their logical text with a multi-line table value.
		heading,e:=l.shape(label+":",12,515);if e!=nil{return e}
		body,e:=l.shape(value,size,515);if e!=nil{return e}
		left=nil;right=append(heading,body...);rightX=40
	}
	count:=max(len(left),len(right))
	for i:=0;i<count;i++ {
		ascent,descent,gap:=10.0,-3.0,4.0
		if i<len(left) {ascent=math.Max(ascent,left[i].ascent);descent=math.Min(descent,left[i].descent);gap=math.Max(gap,left[i].gap)}
		if i<len(right) {ascent=math.Max(ascent,right[i].ascent);descent=math.Min(descent,right[i].descent);gap=math.Max(gap,right[i].gap)}
		height:=ascent-descent+gap
		if height>762 {return errors.New("PDF row cannot fit on a page")}
		if len(l.pages)==0 || l.y-height<40 {
			if err=l.page();err!=nil{return err}
			if i>0 {
				continued,e:=l.shape("Continued",8,515);if e!=nil||len(continued)!=1{return errors.New("PDF continuation label unavailable")}
				if err=l.paintRole(continued[0],40,790,true);err!=nil{return err}
				l.y=776
			}
		}
		baseline:=l.y-ascent
		if i<len(left) {if err=l.paint(left[i],40,baseline);err!=nil{return err}}
		if i<len(right) {if err=l.paint(right[i],rightX,baseline);err!=nil{return err}}
		l.y-=height
	}
	l.y-=titleSpacing(title)
	return nil
}

func titleSpacing(title bool) float64 { if title { return 12 };return 6 }
