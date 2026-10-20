package product

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"testing"
	"unicode/utf16"
)

// This checks the written logical text spans, not an independent PDF reader or
// visual font conformance. The later acceptance phase must exercise both.
func pdfLogicalText(t *testing.T,data []byte) string {
	t.Helper()
	pattern:=regexp.MustCompile(`/Span << /ActualText <([A-F0-9]+)> >> BDC`)
	var text strings.Builder
	for _,match:=range pattern.FindAllSubmatch(data,-1) {
		decoded,err:=hex.DecodeString(string(match[1]));if err!=nil||len(decoded)%2!=0{t.Fatal("invalid UTF16 logical text",err)}
		units:=make([]uint16,len(decoded)/2);for i:=range units{units[i]=binary.BigEndian.Uint16(decoded[i*2:])}
		if len(units)==0||units[0]!=0xfeff{t.Fatal("logical text lacks BOM")}
		text.WriteString(string(utf16.Decode(units[1:])))
	}
	return text.String()
}

func TestDistributePDFPreservesShapedBusinessTextAndSourceEvidence(t *testing.T) {
	lines:=[]artifactExportLine{
		{"Title","Proposal café 日本語"},
		{"Review state","draft"},{"Source freshness","outdated"},
		{"Section: Saved product purpose","office affinity e\u0301 العربية 日本語\nSecond paragraph stays editable in the saved source."},
		{"Key point","First saved point"},{"Key point","Second saved point"},
		{"Source reference","brief:4:"+strings.Repeat("a",64)},
		{"Evidence gap","No implementation check selected."},
		{"Assumption","Commercial figures require review."},
	}
	data,err:=artifactPDF(context.Background(),lines)
	if err!=nil{t.Fatal(err)}
	if !bytes.HasPrefix(data,[]byte("%PDF-"))||len(data)>distributeExportLimit{t.Fatal("not a bounded PDF")}
	logical:=pdfLogicalText(t,data)
	for _,line:=range lines {if !strings.Contains(logical,line.Label)||!strings.Contains(logical,line.Value){t.Fatalf("PDF lost saved field %q",line.Label)}}
	for _,required:=range []string{"/ToUnicode","/ActualText","/FontFile2","/FontFile3","begincidchar","CIDFontType0","CIDFontType2"}{if !bytes.Contains(data,[]byte(required)){t.Fatal("font/code mapping missing",required)}}
	for _,active:=range []string{"/JavaScript","/OpenAction","/EmbeddedFiles","/URI"}{if bytes.Contains(data,[]byte(active)){t.Fatal("unexpected active PDF feature",active)}}
}

func TestDistributePDFExactSavedDigestAndDedicatedPermit(t *testing.T) {
	s,h,id:=controlTestHandler(t)
	base:="/v1/product/projects/"+id+"/distribute/artifacts"
	var proposal struct {PrepareDigest string `json:"prepareDigest"`}
	if err:=json.Unmarshal(controlTestRequest(t,h,"POST",base+"/prepare",map[string]string{"kind":"risk"},200).Body.Bytes(),&proposal);err!=nil{t.Fatal(err)}
	var saved struct {Artifact DistributeArtifact `json:"artifact"`}
	if err:=json.Unmarshal(controlTestRequest(t,h,"POST",base,map[string]string{"kind":"risk","prepareDigest":proposal.PrepareDigest,"idempotencyKey":"pdf-saved-artifact-request-123456"},201).Body.Bytes(),&saved);err!=nil{t.Fatal(err)}
	url:=base+"/"+saved.Artifact.ID+"/exports/pdf?digest="+saved.Artifact.Digest
	// Office's standalone fixture exception is not a PDF permission.
	controlTestRequest(t,h,"GET",url,nil,503)
	acquired,released:=0,0
	s.SetDistributeWorkAdmission(func(context.Context)(func(),error){t.Fatal("PDF used the smaller Office permit");return nil,errors.New("wrong permit")})
	s.SetDistributePDFWorkAdmission(func(context.Context)(func(),error){acquired++;return func(){released++},nil})
	wrong:=controlTestRequest(t,h,"GET",base+"/"+saved.Artifact.ID+"/exports/pdf?digest="+strings.Repeat("0",64),nil,409)
	if bytes.HasPrefix(wrong.Body.Bytes(),[]byte("%PDF")){t.Fatal("wrong digest returned PDF bytes")}
	response:=controlTestRequest(t,h,"GET",url,nil,200)
	if response.Header().Get("Content-Type")!="application/pdf"||!strings.Contains(pdfLogicalText(t,response.Body.Bytes()),saved.Artifact.Title){t.Fatal("saved PDF response incomplete")}
	if acquired!=2||released!=2{t.Fatal("permit not retained through actual return",acquired,released)}
	p,err:=s.resolveProject(id);if err!=nil{t.Fatal(err)}
	_,ledger,err:=s.readArtifactLedger(p);if err!=nil||len(ledger.Records)!=1||len(ledger.Records[0].Revisions)!=1||ledger.Records[0].Revisions[0].Digest!=saved.Artifact.Digest{t.Fatal("export changed private artifact",err)}
}

func TestDistributePDFRefusesUnsupportedTextCancellationAndOutputOverflow(t *testing.T) {
	for _,input:=range []string{"missing \U0010ffff","bad\x01text"} {
		data,err:=artifactPDF(context.Background(),[]artifactExportLine{{"Title",input}})
		if err==nil||data!=nil{t.Fatal("unsupported text returned success")}
	}
	ctx,cancel:=context.WithCancel(context.Background());cancel()
	if data,err:=artifactPDF(ctx,[]artifactExportLine{{"Title","saved"}});!errors.Is(err,context.Canceled)||data!=nil{t.Fatal("cancellation returned success",err)}
	out:=&pdfOutput{ctx:context.Background(),data:make([]byte,distributeExportLimit-1,distributeExportLimit)}
	if n,err:=out.Write([]byte("overflow"));n!=0||err==nil||len(out.data)!=distributeExportLimit-1{t.Fatal("output bound admitted partial write")}
	if n,err:=out.Write([]byte("x"));n!=0||err==nil{t.Fatal("output error was not sticky")}
}
