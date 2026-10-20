package documentinputs

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

// InspectWheel verifies the complete pinned ZIP and every installed RECORD
// member. It observes exact owner, metadata, native dependencies, and retained
// notice bytes; it does not install or infer legal closure.
func InspectWheel(parent context.Context, input io.ReaderAt, size int64, pin Pin, owner WheelOwner) (Wheel,error) {
	var result Wheel
	ctx,cancel,err:=checkedContext(parent,90*time.Second); if err!=nil{return result,err};defer cancel()
	if !validPin(pin,maxWheelBytes)||size!=pin.Bytes||input==nil||owner.ID==""||owner.Version==""||len(owner.ID)>128||len(owner.Version)>128||strings.ContainsAny(owner.ID+owner.Version,"/\\\x00\r\n") { return result,ErrInvalid }
	if name,ok:=cleanName(owner.InstalledOwner,false);!ok||name!=owner.InstalledOwner||strings.Contains(name,"/")||!strings.HasSuffix(name,".dist-info") {return result,ErrInvalid}
	budget:=&boundedReaderAt{ctx:ctx,reader:input,maxBytes:maxWheelBytes+2*maxWheelExpanded}
	actual,err:=hashReaderAt(ctx,input,size,budget);if err!=nil||actual!=pin.SHA256{return result,ErrInvalid}
	archive,err:=zip.NewReader(budget,size);if err!=nil||len(archive.File)<1||len(archive.File)>maxMembers{return result,ErrInvalid}
	result=Wheel{ID:owner.ID,Version:owner.Version,Components:[]Component{},Files:[]File{},Notices:[]Notice{}}
	seen:=map[string]bool{};seenFolded:=map[string]bool{}
	var record,metadata []byte
	var total,noticeTotal int64
	for _,entry:=range archive.File {
		if ctx.Err()!=nil{return Wheel{},ErrInvalid}
		name,ok:=cleanName(entry.Name,false)
		if !ok||seen[name]||seenFolded[strings.ToLower(name)]||entry.Mode()&os.ModeSymlink!=0||entry.Flags&0x1!=0 {return Wheel{},ErrInvalid}
		seen[name]=true;seenFolded[strings.ToLower(name)]=true
		if entry.FileInfo().IsDir() {
			if !strings.HasSuffix(entry.Name,"/")||entry.UncompressedSize64!=0{return Wheel{},ErrInvalid}
			continue
		}
		if !entry.Mode().IsRegular()||entry.UncompressedSize64>uint64(maxWheelExpanded-total){return Wheel{},ErrInvalid}
		total+=int64(entry.UncompressedSize64)
		stream,e:=entry.Open();if e!=nil{return Wheel{},ErrInvalid}
		h:=sha256.New();var n int64
		keepMeta:=name==owner.InstalledOwner+"/METADATA"||name==owner.InstalledOwner+"/RECORD"
		keepNotice:=wheelNoticeName(name)
		keepNative:=strings.HasSuffix(name,".so")||strings.Contains(name,".so.")
		if keepMeta&&entry.UncompressedSize64>1<<20||keepNotice&&entry.UncompressedSize64>1<<20||keepNative&&entry.UncompressedSize64>uint64(maxELFBytes){_ =stream.Close();return Wheel{},ErrInvalid}
		var retained bytes.Buffer
		var block [readBlock]byte
		for {
			if ctx.Err()!=nil{_ =stream.Close();return Wheel{},ErrInvalid}
			read,readErr:=stream.Read(block[:]);if read<0||read>len(block){_ =stream.Close();return Wheel{},ErrInvalid}
			if read>0 {n+=int64(read);if n>int64(entry.UncompressedSize64){_ =stream.Close();return Wheel{},ErrInvalid};_,_=h.Write(block[:read]);if keepMeta||keepNotice||keepNative{_,_=retained.Write(block[:read])}}
			if readErr==io.EOF{break};if readErr!=nil||read==0{_ =stream.Close();return Wheel{},ErrInvalid}
		}
		if stream.Close()!=nil||n!=int64(entry.UncompressedSize64){return Wheel{},ErrInvalid}
		file:=File{Path:name,Owner:owner.InstalledOwner,Kind:"regular",Bytes:n,SHA256:hex.EncodeToString(h.Sum(nil))}
		if keepNative {
			arch,needed,e:=nativeComponent(retained.Bytes());if e!=nil{return Wheel{},ErrInvalid}
			file.Architecture,file.Needed=arch,needed
			result.Components=append(result.Components,Component{Path:name,Architecture:arch,Needed:needed})
		}
		if keepNotice {
			noticeTotal+=n;if noticeTotal>maxNoticeBytes{return Wheel{},ErrInvalid}
			result.Notices=append(result.Notices,Notice{Path:name,Owner:owner.InstalledOwner,Bytes:n,SHA256:file.SHA256,Contents:append([]byte(nil),retained.Bytes()...)})
		}
		if name==owner.InstalledOwner+"/RECORD" {record=append([]byte(nil),retained.Bytes()...)}
		if name==owner.InstalledOwner+"/METADATA" {metadata=append([]byte(nil),retained.Bytes()...)}
		result.Files=append(result.Files,file)
	}
	if len(record)==0||len(metadata)==0{return Wheel{},ErrInvalid}
	name,version,err:=wheelMetadataIdentity(metadata)
	if err!=nil||name!=owner.ID||version!=owner.Version{return Wheel{},ErrInvalid}
	if !validWheelRecord(record,owner.InstalledOwner+"/RECORD",result.Files){return Wheel{},ErrInvalid}
	return result,nil
}

func wheelNoticeName(name string) bool {
	base:=strings.ToUpper(path.Base(name))
	return strings.HasPrefix(base,"LICENSE")||strings.HasPrefix(base,"LICENCE")||strings.HasPrefix(base,"COPYING")||strings.HasPrefix(base,"NOTICE")
}

func validWheelRecord(raw []byte,recordName string,files []File) bool {
	reader:=csv.NewReader(bytes.NewReader(raw));reader.FieldsPerRecord=3;reader.ReuseRecord=false
	rows:=map[string][2]string{}
	for {
		row,err:=reader.Read();if err==io.EOF{break};if err!=nil||len(row)!=3{return false}
		name,ok:=cleanName(row[0],false);if !ok||name!=row[0]{return false}
		if _,duplicate:=rows[name];duplicate{return false}
		rows[name]=[2]string{row[1],row[2]}
		if len(rows)>maxMembers{return false}
	}
	if len(rows)!=len(files){return false}
	for _,file:=range files{
		row,ok:=rows[file.Path];if !ok{return false}
		if file.Path==recordName {if row[0]!=""||row[1]!=""{return false};continue}
		if !strings.HasPrefix(row[0],"sha256="){return false}
		decoded,err:=base64.RawURLEncoding.DecodeString(strings.TrimPrefix(row[0],"sha256="))
		if err!=nil||len(decoded)!=sha256.Size||hex.EncodeToString(decoded)!=file.SHA256{return false}
		amount,err:=strconv.ParseInt(row[1],10,64);if err!=nil||amount!=file.Bytes||row[1]!=strconv.FormatInt(amount,10){return false}
	}
	return true
}
