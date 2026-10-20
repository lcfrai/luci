package documentinputs

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"strings"
	"time"
)

type meteredReader struct {
	ctx context.Context
	source io.Reader
	bytes,maxBytes int64
	calls int
	hash io.Writer
}

func (r *meteredReader) Read(p []byte)(int,error){
	if len(p)==0{return 0,nil}
	if r.ctx.Err()!=nil||r.calls>=maxCalls||r.bytes>=r.maxBytes{return 0,ErrInvalid}
	r.calls++
	if int64(len(p))>r.maxBytes-r.bytes{p=p[:int(r.maxBytes-r.bytes)]}
	n,err:=r.source.Read(p);if n<0||n>len(p){return 0,ErrInvalid}
	r.bytes+=int64(n);if r.hash!=nil&&n>0{_,_=r.hash.Write(p[:n])}
	if r.ctx.Err()!=nil||n==0&&err==nil{return 0,ErrInvalid}
	return n,err
}

// InspectBase hashes the exact compressed layer and inventories actual dpkg
// status, .list ownership, files, ELF imports, and original copyright bytes.
// Non-packaged files retain an empty owner rather than acquiring a guessed one.
func InspectBase(parent context.Context, input io.Reader, pin Pin)(Base,error){
	var result Base
	ctx,cancel,err:=checkedContext(parent,180*time.Second);if err!=nil{return result,err};defer cancel()
	if input==nil||!validPin(pin,maxBaseBytes){return result,ErrInvalid}
	compressedHash:=sha256.New()
	compressed:=&meteredReader{ctx:ctx,source:input,maxBytes:pin.Bytes+1,hash:compressedHash}
	gz,err:=gzip.NewReader(compressed);if err!=nil{return result,ErrInvalid}
	defer gz.Close()
	expanded:=&meteredReader{ctx:ctx,source:gz,maxBytes:maxBaseExpanded}
	archive:=tar.NewReader(expanded)
	result=Base{Packages:[]Package{},Files:[]File{},Notices:[]Notice{}}
	seen:=map[string]bool{};lists:=map[string][]byte{}
	var status []byte
	var noticesTotal,namesBytes,listBytes int64
	for {
		if ctx.Err()!=nil{return Base{},ErrInvalid}
		entry,e:=archive.Next();if e==io.EOF{break};if e!=nil||entry==nil||entry.Size<0{return Base{},ErrInvalid}
		if entry.Name=="./"&&entry.Typeflag==tar.TypeDir{continue}
		name,ok:=cleanName(entry.Name,true);if !ok||seen[name]||len(seen)>=maxMembers{return Base{},ErrInvalid}
		seen[name]=true;namesBytes+=int64(len(entry.Name));if namesBytes>16<<20{return Base{},ErrInvalid}
		file:=File{Path:name,Bytes:entry.Size}
		switch entry.Typeflag {
		case tar.TypeDir:
			if entry.Size!=0{return Base{},ErrInvalid};file.Kind="directory"
		case tar.TypeSymlink,tar.TypeLink:
			if entry.Size!=0||entry.Linkname==""||len(entry.Linkname)>1024||strings.ContainsAny(entry.Linkname,"\x00\r\n") {return Base{},ErrInvalid}
			file.Target=entry.Linkname;if entry.Typeflag==tar.TypeSymlink{file.Kind="symlink"}else{file.Kind="hardlink"}
		case tar.TypeReg,tar.TypeRegA:
			if entry.Size>128<<20{return Base{},ErrInvalid}
			file.Kind="regular"
			keepStatus:=name=="var/lib/dpkg/status"
			keepList:=strings.HasPrefix(name,"var/lib/dpkg/info/")&&strings.HasSuffix(name,".list")
			keepNotice:=strings.HasPrefix(name,"usr/share/doc/")&&strings.HasSuffix(name,"/copyright")
			limit:=int64(16<<20);if keepNotice{limit=1<<20}
			if (keepStatus||keepList||keepNotice)&&entry.Size>limit{return Base{},ErrInvalid}
			h:=sha256.New();var kept bytes.Buffer;var native elfCapture
			var block [readBlock]byte;var readTotal int64
			for readTotal<entry.Size{
				if ctx.Err()!=nil{return Base{},ErrInvalid}
				need:=int64(len(block));if entry.Size-readTotal<need{need=entry.Size-readTotal}
				n,e:=archive.Read(block[:int(need)]);if n<1||e!=nil&&e!=io.EOF{return Base{},ErrInvalid}
				readTotal+=int64(n);_,_=h.Write(block[:n])
				if native.write(block[:n])!=nil{return Base{},ErrInvalid}
				if keepStatus||keepList||keepNotice{_,_=kept.Write(block[:n])}
			}
			file.SHA256=hex.EncodeToString(h.Sum(nil))
			if keepStatus{status=append([]byte(nil),kept.Bytes()...)}
			if keepList{
				listBytes+=entry.Size
				if listBytes>32<<20||len(lists)>=10000{return Base{},ErrInvalid}
				lists[name]=append([]byte(nil),kept.Bytes()...)
			}
			if keepNotice{
				noticesTotal+=entry.Size;if noticesTotal>maxNoticeBytes{return Base{},ErrInvalid}
				result.Notices=append(result.Notices,Notice{Path:name,Bytes:entry.Size,SHA256:file.SHA256,Contents:append([]byte(nil),kept.Bytes()...)})
			}
			if native.isELF{arch,needed,e:=nativeComponent(native.raw.Bytes());if e!=nil{return Base{},ErrInvalid};file.Architecture,file.Needed=arch,needed}
		default:return Base{},ErrInvalid
		}
		result.Files=append(result.Files,file)
	}
	// Tar EOF can precede zero padding. Consume the decompressed tail and refuse
	// a concatenated rootfs or any nonzero data outside inventoried members.
	var trailing [readBlock]byte
	for {
		n,e:=expanded.Read(trailing[:]);for _,value:=range trailing[:n]{if value!=0{return Base{},ErrInvalid}}
		if e==io.EOF{break};if e!=nil||n==0{return Base{},ErrInvalid}
	}
	if gz.Close()!=nil{return Base{},ErrInvalid}
	var one [1]byte
	for compressed.bytes<pin.Bytes {
		n,e:=compressed.Read(one[:]);if n==0&&e!=nil{return Base{},ErrInvalid}
	}
	if compressed.bytes!=pin.Bytes||hex.EncodeToString(compressedHash.Sum(nil))!=pin.SHA256{return Base{},ErrInvalid}
	if n,e:=compressed.Read(one[:]);n!=0||e!=io.EOF{return Base{},ErrInvalid}
	if len(status)==0{return Base{},ErrInvalid}
	rows,e:=paragraphFields(status,16<<20);if e!=nil{return Base{},ErrInvalid}
	packages:=map[string]bool{}
	for _,fields:=range rows{
		if fields["Status"]!="install ok installed"{continue}
		pkg,e:=controlPackage(fields);if e!=nil||packages[pkg.Name+":"+pkg.Architecture]{return Base{},ErrInvalid}
		packages[pkg.Name]=true;packages[pkg.Name+":"+pkg.Architecture]=true;result.Packages=append(result.Packages,pkg)
	}
	if len(result.Packages)==0||len(result.Packages)>10000{return Base{},ErrInvalid}
	owners:=map[string]string{};multi:=map[string]bool{};listLines:=0
	for name,raw:=range lists{
		pkg:=strings.TrimSuffix(strings.TrimPrefix(name,"var/lib/dpkg/info/"),".list")
		if !packages[pkg]{continue}
		for len(raw)>0 {
			end:=bytes.IndexByte(raw,'\n');line:=raw
			if end>=0{line=raw[:end];raw=raw[end+1:]}else{raw=nil}
			listLines++;if listLines>maxMembers||len(line)>1025{return Base{},ErrInvalid}
			value:=string(bytes.TrimSpace(line));if value==""||value=="/."{continue}
			value=strings.TrimPrefix(value,"/");normalized,ok:=cleanName(value,true);if !ok{return Base{},ErrInvalid}
			if old:=owners[normalized];old!=""&&old!=pkg{multi[normalized]=true}
			owners[normalized]=pkg
		}
	}
	for i:=range result.Files{if !multi[result.Files[i].Path]{result.Files[i].Owner=owners[result.Files[i].Path]}}
	for i:=range result.Notices{if !multi[result.Notices[i].Path]{result.Notices[i].Owner=owners[result.Notices[i].Path]}}
	return result,nil
}
