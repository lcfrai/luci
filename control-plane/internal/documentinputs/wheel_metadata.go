package documentinputs

import (
	"bytes"
	"strings"
	"unicode/utf8"
)

type wheelMetadata struct {
	name,version,requiresPython string
	requiresDist []string
}

// metadataHeaders accepts repeated Core Metadata fields and a free-form body.
// It calls visit only for header fields; the description body is not a second
// paragraph of metadata or a source of package identity.
func metadataHeaders(raw []byte,allowBody bool,visit func(string,string)error) error {
	if len(raw)==0||len(raw)>1<<20||!utf8.Valid(raw)||bytes.IndexByte(raw,0)>=0{return ErrInvalid}
	last:="";value:="";body:=false
	flush:=func()error{if last==""{return nil};if visit(last,value)!=nil{return ErrInvalid};last="";value="";return nil}
	for len(raw)>0 {
		end:=bytes.IndexByte(raw,'\n');var line []byte
		if end<0 {line=raw;raw=nil} else {line=raw[:end];raw=raw[end+1:]}
		line=bytes.TrimSuffix(line,[]byte{'\r'})
		if body {if !allowBody&&len(line)>0{return ErrInvalid};continue}
		if len(line)==0 {if flush()!=nil{return ErrInvalid};body=true;continue}
		if line[0]==' '||line[0]=='\t' {
			if last==""||last=="name"||last=="version"||last=="root-is-purelib"||last=="wheel-version"{return ErrInvalid}
			value+=" "+string(bytes.TrimSpace(line))
			continue
		}
		if flush()!=nil{return ErrInvalid}
		at:=bytes.IndexByte(line,':')
		if at<1||at>80||!validFieldName(string(line[:at])){return ErrInvalid}
		key:=strings.ToLower(string(line[:at]));part:=bytes.TrimSpace(line[at+1:])
		if bytes.IndexByte(part,'\r')>=0||bytes.IndexByte(part,'\n')>=0{return ErrInvalid}
		last=key;value=string(part)
	}
	return flush()
}

func parseWheelMetadata(raw []byte)(wheelMetadata,error){
	result:=wheelMetadata{requiresDist:[]string{}}
	err:=metadataHeaders(raw,true,func(key,value string)error{
		switch key {
		case "name":
			if result.name!=""||value==""||len(value)>128{return ErrInvalid};result.name=value
		case "version":
			if result.version!=""||value==""||len(value)>128{return ErrInvalid};result.version=value
		case "requires-python":
			if result.requiresPython!=""||value==""||len(value)>512{return ErrInvalid};result.requiresPython=value
		case "requires-dist":
			if value==""||len(value)>2048||len(result.requiresDist)>=256{return ErrInvalid}
			result.requiresDist=append(result.requiresDist,value)
		}
		return nil
	})
	if err!=nil||result.name==""||result.version==""{return wheelMetadata{},ErrInvalid}
	return result,nil
}

func parseWheelFile(raw []byte)([]string,bool,error){
	tags:=[]string{};version:="";purelib:="";seenTag:=map[string]bool{}
	err:=metadataHeaders(raw,false,func(key,value string)error{
		switch key {
		case "wheel-version":
			if version!=""||value!="1.0"{return ErrInvalid};version=value
		case "root-is-purelib":
			if purelib!=""||value!="true"&&value!="false"{return ErrInvalid};purelib=value
		case "tag":
			if len(value)==0||len(value)>200||len(tags)>=256||seenTag[value]{return ErrInvalid}
			for _,c:=range value{if !(c>='a'&&c<='z'||c>='A'&&c<='Z'||c>='0'&&c<='9'||c=='_'||c=='.'||c=='-'){return ErrInvalid}}
			if strings.Count(value,"-")!=2{return ErrInvalid}
			seenTag[value]=true;tags=append(tags,value)
		}
		return nil
	})
	if err!=nil||version==""||purelib==""||len(tags)==0{return nil,false,ErrInvalid}
	return tags,purelib=="true",nil
}
