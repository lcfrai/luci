package documentinputs

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/ulikunitz/xz"
)

type DebExpectation struct {
	Name string `json:"name"`
	Version string `json:"version"`
	Architecture string `json:"architecture"`
}

type Deb struct {
	Package Package `json:"package"`
	Files []File `json:"files"`
	Notices []Notice `json:"notices"`
	Conffiles []string `json:"conffiles"`
}

type arEntry struct { name string; offset,size int64 }

// InspectDeb verifies the whole pinned .deb, its control tuple, and every
// actual data.tar member. Maintainer scripts are never executed or accepted as
// installed-file evidence. Copyright notices retain their original bytes.
func InspectDeb(parent context.Context, input io.ReaderAt, size int64, pin Pin, expected DebExpectation)(Deb,error){
	var result Deb
	ctx,cancel,err:=checkedContext(parent,120*time.Second);if err!=nil{return result,err};defer cancel()
	if input==nil||size!=pin.Bytes||!validPin(pin,maxSourceArchiveBytes)||expected.Name==""||expected.Version==""||expected.Architecture=="" {return result,ErrInvalid}
	budget:=&boundedReaderAt{ctx:ctx,reader:input,maxBytes:3*pin.Bytes+readBlock}
	hash,e:=hashReaderAt(ctx,input,size,budget);if e!=nil||hash!=pin.SHA256{return result,ErrInvalid}
	var magic [8]byte
	if n,e:=budget.ReadAt(magic[:],0);n!=len(magic)||e!=nil||string(magic[:])!="!<arch>\n"{return result,ErrInvalid}
	entries:=[]arEntry{};offset:=int64(len(magic))
	for offset<size {
		if ctx.Err()!=nil||len(entries)>=3||size-offset<60{return result,ErrInvalid}
		var header [60]byte
		if n,e:=budget.ReadAt(header[:],offset);n!=60||e!=nil||string(header[58:60])!="`\n"{return result,ErrInvalid}
		name:=strings.TrimSuffix(strings.TrimSpace(string(header[:16])),"/")
		amount,e:=strconv.ParseInt(strings.TrimSpace(string(header[48:58])),10,64)
		if e!=nil||amount<1||amount>size-offset-60{return result,ErrInvalid}
		entries=append(entries,arEntry{name:name,offset:offset+60,size:amount})
		offset+=60+amount
		if amount%2==1{var padding [1]byte;if n,e:=budget.ReadAt(padding[:],offset);n!=1||e!=nil||padding[0]!='\n'{return result,ErrInvalid};offset++}
	}
	if offset!=size||len(entries)!=3||entries[0].name!="debian-binary"||entries[0].size!=4{return result,ErrInvalid}
	var format [4]byte
	if n,e:=budget.ReadAt(format[:],entries[0].offset);n!=4||e!=nil||string(format[:])!="2.0\n"{return result,ErrInvalid}
	if !debMemberName(entries[1].name,"control.tar")||!debMemberName(entries[2].name,"data.tar"){return result,ErrInvalid}
	control,e:=decodeDebMember(ctx,budget,entries[1],8<<20);if e!=nil{return result,ErrInvalid}
	rows,conffiles,e:=readDebControlTar(ctx,control);if e!=nil{return result,ErrInvalid}
	result.Package,e=controlPackage(rows);if e!=nil||result.Package.Name!=expected.Name||result.Package.Version!=expected.Version||result.Package.Architecture!=expected.Architecture{return Deb{},ErrInvalid}
	result.Conffiles=conffiles
	result.Files,result.Notices,e=readDebDataTar(ctx,budget,entries[2],result.Package.Name)
	if e!=nil||len(result.Files)==0{return Deb{},ErrInvalid}
	actualFiles:=make(map[string]bool,len(result.Files))
	for _,file:=range result.Files { if file.Kind=="regular" { actualFiles[file.Path]=true } }
	for _,name:=range result.Conffiles { if !actualFiles[strings.TrimPrefix(name,"/")] { return Deb{},ErrInvalid } }
	return result,nil
}

func debMemberName(name,prefix string) bool {return name==prefix+".gz"||name==prefix+".xz"}

func debDecoder(source io.Reader,name string)(io.Reader,io.Closer,error){
	if strings.HasSuffix(name,".gz"){reader,e:=gzip.NewReader(source);if e!=nil{return nil,nil,ErrInvalid};return reader,reader,nil}
	if strings.HasSuffix(name,".xz"){
		reader,e:=(xz.ReaderConfig{DictCap:64<<20}).NewReader(source);if e!=nil{return nil,nil,ErrInvalid}
		return reader,nil,nil
	}
	return nil,nil,ErrInvalid
}

type countedInput struct { source io.Reader; bytes int64 }
func (r *countedInput) Read(p []byte)(int,error){n,e:=r.source.Read(p);r.bytes+=int64(n);return n,e}

func decodeDebMember(ctx context.Context,reader io.ReaderAt,entry arEntry,limit int64)([]byte,error){
	section:=&countedInput{source:io.NewSectionReader(reader,entry.offset,entry.size)}
	decoded,closer,e:=debDecoder(section,entry.name);if e!=nil{return nil,e};if closer!=nil{defer closer.Close()}
	bounded:=&meteredReader{ctx:ctx,source:decoded,maxBytes:limit+1}
	raw,e:=io.ReadAll(io.LimitReader(bounded,limit+1))
	if e!=nil||int64(len(raw))>limit||section.bytes!=entry.size{return nil,ErrInvalid}
	var extra [1]byte;if n,e:=bounded.Read(extra[:]);n!=0||e!=io.EOF{return nil,ErrInvalid}
	return raw,nil
}

func readDebControlTar(ctx context.Context,raw []byte)(map[string]string,[]string,error){
	archive:=tar.NewReader(bytes.NewReader(raw));seen:=map[string]bool{};var control,conffileBytes []byte
	for {
		if ctx.Err()!=nil{return nil,nil,ErrInvalid}
		entry,e:=archive.Next();if e==io.EOF{break};if e!=nil||entry==nil||entry.Size<0||len(seen)>256{return nil,nil,ErrInvalid}
		if entry.Name=="./"&&entry.Typeflag==tar.TypeDir{continue}
		name,ok:=cleanName(entry.Name,true);if !ok||seen[name]{return nil,nil,ErrInvalid};seen[name]=true
		if name=="control"||name=="conffiles" {
			if entry.Typeflag!=tar.TypeReg||entry.Size<0||entry.Size>1<<20||name=="control"&&entry.Size==0{return nil,nil,ErrInvalid}
			retained:=make([]byte,entry.Size);if _,e=io.ReadFull(archive,retained);e!=nil{return nil,nil,ErrInvalid}
			if name=="control" { control=retained } else { conffileBytes=retained }
		}
	}
	if len(control)==0{return nil,nil,ErrInvalid}
	rows,e:=paragraphFields(control,1<<20);if e!=nil||len(rows)!=1{return nil,nil,ErrInvalid}
	conffiles:=[]string{};listed:=map[string]bool{}
	for _,line:=range bytes.Split(conffileBytes,[]byte{'\n'}) {
		if len(line)==0 {continue}
		if len(line)>1025||line[0]!='/' {return nil,nil,ErrInvalid}
		name,ok:=cleanName(string(line[1:]),false)
		if !ok||listed[name]||name!=string(line[1:]) {return nil,nil,ErrInvalid}
		listed[name]=true;conffiles=append(conffiles,"/"+name)
		if len(conffiles)>4096{return nil,nil,ErrInvalid}
	}
	return rows[0],conffiles,nil
}

func readDebDataTar(ctx context.Context,reader io.ReaderAt,entry arEntry,owner string)([]File,[]Notice,error){
	section:=&countedInput{source:io.NewSectionReader(reader,entry.offset,entry.size)}
	decoded,closer,e:=debDecoder(section,entry.name);if e!=nil{return nil,nil,e};if closer!=nil{defer closer.Close()}
	bounded:=&meteredReader{ctx:ctx,source:decoded,maxBytes:maxBaseExpanded}
	archive:=tar.NewReader(bounded)
	files:=[]File{};notices:=[]Notice{};seen:=map[string]bool{};var noticeTotal,namesBytes int64
	for {
		if ctx.Err()!=nil{return nil,nil,ErrInvalid}
		item,e:=archive.Next();if e==io.EOF{break};if e!=nil||item==nil||item.Size<0{return nil,nil,ErrInvalid}
		if item.Name=="./"&&item.Typeflag==tar.TypeDir{continue}
		name,ok:=cleanName(item.Name,true);if !ok||seen[name]||len(seen)>=maxMembers{return nil,nil,ErrInvalid}
		seen[name]=true;namesBytes+=int64(len(item.Name));if namesBytes>16<<20{return nil,nil,ErrInvalid}
		file:=File{Path:name,Owner:owner,Bytes:item.Size}
		switch item.Typeflag{
		case tar.TypeDir:
			if item.Size!=0{return nil,nil,ErrInvalid};file.Kind="directory"
		case tar.TypeSymlink,tar.TypeLink:
			if item.Size!=0||item.Linkname==""||len(item.Linkname)>1024||strings.ContainsAny(item.Linkname,"\x00\r\n"){return nil,nil,ErrInvalid}
			file.Target=item.Linkname;if item.Typeflag==tar.TypeSymlink{file.Kind="symlink"}else{file.Kind="hardlink"}
		case tar.TypeReg,tar.TypeRegA:
			if item.Size>128<<20{return nil,nil,ErrInvalid};file.Kind="regular"
			keepNotice:=strings.HasPrefix(name,"usr/share/doc/")&&strings.HasSuffix(name,"/copyright")
			if keepNotice&&item.Size>1<<20{return nil,nil,ErrInvalid}
			h:=sha256.New();var kept bytes.Buffer;var first,isELF bool;var readTotal int64
			var block [readBlock]byte
			for readTotal<item.Size{
				if ctx.Err()!=nil{return nil,nil,ErrInvalid}
				need:=int64(len(block));if item.Size-readTotal<need{need=item.Size-readTotal}
				n,e:=archive.Read(block[:int(need)]);if n<1||e!=nil&&e!=io.EOF{return nil,nil,ErrInvalid}
				if !first{first=true;isELF=n>=4&&string(block[:4])=="\x7fELF"}
				readTotal+=int64(n);_,_=h.Write(block[:n])
				if keepNotice||isELF{if isELF&&readTotal>maxELFBytes{return nil,nil,ErrInvalid};_,_=kept.Write(block[:n])}
			}
			file.SHA256=hex.EncodeToString(h.Sum(nil))
			if isELF{arch,needed,e:=nativeComponent(kept.Bytes());if e!=nil{return nil,nil,ErrInvalid};file.Architecture,file.Needed=arch,needed}
			if keepNotice{noticeTotal+=item.Size;if noticeTotal>maxNoticeBytes{return nil,nil,ErrInvalid};notices=append(notices,Notice{Path:name,Owner:owner,Bytes:item.Size,SHA256:file.SHA256,Contents:append([]byte(nil),kept.Bytes()...)})}
		default:return nil,nil,ErrInvalid
		}
		files=append(files,file)
	}
	var tail [readBlock]byte
	for {n,e:=bounded.Read(tail[:]);for _,v:=range tail[:n]{if v!=0{return nil,nil,ErrInvalid}};if e==io.EOF{break};if e!=nil||n==0{return nil,nil,ErrInvalid}}
	if section.bytes!=entry.size{return nil,nil,ErrInvalid}
	return files,notices,nil
}
