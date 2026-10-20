package documentinputs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/ulikunitz/xz"
)

// InspectSources requires the compressed Sources.xz retained under an
// independently authenticated release/index binding. The caller proves that
// relation; this parser verifies exact compressed and expanded bytes, selected
// paragraphs, and each original source archive. It does not verify signatures.
func InspectSources(parent context.Context, compressedInput io.Reader, binding SourcesBinding, archives []SourceArchiveInput) ([]Source,error) {
	ctx,cancel,err:=checkedContext(parent,120*time.Second);if err!=nil{return nil,err};defer cancel()
	if compressedInput==nil||!validHex(binding.SignedReleaseSHA256)||binding.IndexPath!="main/source/Sources.xz"||
		!validPin(binding.CompressedIndex,maxSourceIndexBytes)||!validPin(binding.Cleartext,maxSourceCleartextBytes)||
		len(archives)<1||len(archives)>256{return nil,ErrInvalid}
	wanted:=map[string]SourceArchiveInput{}
	var archiveBytes int64
	for _,archive:=range archives{
		name,ok:=cleanName(archive.FileName,false)
		if !ok||name!=archive.FileName||archive.SourceName==""||archive.SourceVersion==""||
			len(archive.SourceName)>200||len(archive.SourceVersion)>200||archive.Reader==nil||
			!validPin(archive.Pin,maxSourceArchiveBytes)||wanted[name].Reader!=nil{return nil,ErrInvalid}
		archiveBytes+=archive.Pin.Bytes
		if archiveBytes>maxSelectedSourceArchiveBytes{return nil,ErrInvalid}
		wanted[name]=archive
	}
	compressedHash:=sha256.New()
	bounded:=&meteredReader{ctx:ctx,source:compressedInput,maxBytes:binding.CompressedIndex.Bytes+1,hash:compressedHash}
	// The selected pure-Go decoder must be pinned and reviewed before source
	// checks. Its 64 MiB dictionary cap prevents a hostile XZ header from
	// allocating memory outside the caller's admission budget.
	decoder,err:=(xz.ReaderConfig{DictCap:64<<20}).NewReader(bounded)
	if err!=nil{return nil,ErrInvalid}
	clear,err:=io.ReadAll(io.LimitReader(decoder,binding.Cleartext.Bytes+1))
	if err!=nil||int64(len(clear))!=binding.Cleartext.Bytes||hashBytes(clear)!=binding.Cleartext.SHA256{return nil,ErrInvalid}
	var extra [1]byte
	if n,e:=decoder.Read(extra[:]);n!=0||e!=io.EOF{return nil,ErrInvalid}
	if bounded.bytes!=binding.CompressedIndex.Bytes||hex.EncodeToString(compressedHash.Sum(nil))!=binding.CompressedIndex.SHA256{return nil,ErrInvalid}
	if n,e:=bounded.Read(extra[:]);n!=0||e!=io.EOF{return nil,ErrInvalid}
	selected:=map[string]*Source{};matched:=map[string]bool{}
	paragraphs:=0
	for _,block:=range bytes.Split(clear,[]byte("\n\n")) {
		if ctx.Err()!=nil{return nil,ErrInvalid}
		if len(bytes.TrimSpace(block))==0{continue}
		paragraphs++;if paragraphs>100000||len(block)>1<<20{return nil,ErrInvalid}
		rows,e:=paragraphFields(block,1<<20);if e!=nil||len(rows)!=1{return nil,ErrInvalid}
		row:=rows[0];name,version:=row["Package"],row["Version"]
		if name==""||version==""||len(name)>200||len(version)>200{return nil,ErrInvalid}
		key:=name+"\x00"+version
		needed:=false;for _,candidate:=range archives{if candidate.SourceName==name&&candidate.SourceVersion==version{needed=true;break}}
		if !needed{continue}
		if selected[key]!=nil{return nil,ErrInvalid}
		directory,ok:=cleanName(row["Directory"],false);if !ok||directory!=row["Directory"]{return nil,ErrInvalid}
		rows:=strings.Split(row["Checksums-Sha256"],"\n")
		if len(rows)==0||len(rows)>256{return nil,ErrInvalid}
		candidate:=&Source{Name:name,Version:version,Archives:[]SourceArchive{}}
		seen:=map[string]bool{}
		for _,line:=range rows{
			if strings.TrimSpace(line)==""{continue}
			fields:=strings.Fields(line);if len(fields)!=3||!validHex(fields[0]){return nil,ErrInvalid}
			amount,e:=strconv.ParseInt(fields[1],10,64);if e!=nil||amount<1||amount>maxSourceArchiveBytes{return nil,ErrInvalid}
			base,ok:=cleanName(fields[2],false);if !ok||strings.Contains(base,"/"){return nil,ErrInvalid}
			full:=directory+"/"+base;if seen[full]{return nil,ErrInvalid};seen[full]=true
			input,exists:=wanted[full];if !exists{continue}
			if input.SourceName!=name||input.SourceVersion!=version||input.Pin.Bytes!=amount||input.Pin.SHA256!=fields[0]||matched[full]{return nil,ErrInvalid}
			budget:=&boundedReaderAt{ctx:ctx,reader:input.Reader,maxBytes:input.Pin.Bytes+readBlock}
			actual,e:=hashReaderAt(ctx,input.Reader,input.Pin.Bytes,budget);if e!=nil||actual!=input.Pin.SHA256{return nil,ErrInvalid}
			matched[full]=true
			candidate.Archives=append(candidate.Archives,SourceArchive{FileName:full,Bytes:amount,SHA256:actual})
		}
		if len(candidate.Archives)==0{return nil,ErrInvalid}
		selected[key]=candidate
	}
	if len(matched)!=len(wanted){return nil,ErrInvalid}
	out:=make([]Source,0,len(selected))
	for _,candidate:=range selected{out=append(out,*candidate)}
	// Stable order is part of the observation contract and not proof of grants.
	sortSources(out)
	return out,nil
}

func sortSources(rows []Source) {
	for i:=1;i<len(rows);i++{for j:=i;j>0&&rows[j].Name+"\x00"+rows[j].Version<rows[j-1].Name+"\x00"+rows[j-1].Version;j--{rows[j],rows[j-1]=rows[j-1],rows[j]}}
}
