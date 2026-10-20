package documentinputs

import (
	"bytes"
	"strings"
	"unicode/utf8"
)

func entryPointName(value string) bool {
	if value==""||len(value)>128{return false}
	for _,c:=range value {if !(c>='A'&&c<='Z'||c>='a'&&c<='z'||c>='0'&&c<='9'||c=='_'||c=='-'||c=='.'){return false}}
	return true
}

func pythonIdentifierPath(value string) bool {
	if value==""||len(value)>256{return false}
	for _,part:=range strings.Split(value,".") {
		if part==""{return false}
		for i,c:=range part {
			if c>='A'&&c<='Z'||c>='a'&&c<='z'||c=='_'{continue}
			if i>0&&c>='0'&&c<='9'{continue}
			return false
		}
	}
	return true
}

// Only the two installer-generated executable groups confer a script name.
// Other well-formed groups may exist, but are not reported as commands.
func parseWheelEntryPoints(raw []byte)([]EntryPoint,error){
	if len(raw)==0||len(raw)>1<<20||!utf8.Valid(raw)||bytes.IndexByte(raw,0)>=0{return nil,ErrInvalid}
	result:=[]EntryPoint{};section:="";sections:=map[string]bool{};scripts:=map[string]bool{};lines:=0
	for len(raw)>0 {
		lines++;if lines>20000{return nil,ErrInvalid}
		end:=bytes.IndexByte(raw,'\n');var line []byte
		if end<0{line=raw;raw=nil}else{line=raw[:end];raw=raw[end+1:]}
		line=bytes.TrimSuffix(line,[]byte{'\r'})
		if len(line)>4096{return nil,ErrInvalid}
		value:=strings.TrimSpace(string(line))
		if value==""||strings.HasPrefix(value,"#")||strings.HasPrefix(value,";"){continue}
		if strings.HasPrefix(value,"[") {
			if !strings.HasSuffix(value,"]"){return nil,ErrInvalid}
			section=value[1:len(value)-1]
			if !entryPointName(section)||sections[section]{return nil,ErrInvalid}
			sections[section]=true;continue
		}
		if section==""{return nil,ErrInvalid}
		at:=strings.IndexByte(value,'=')
		if at<1{return nil,ErrInvalid}
		name:=strings.TrimSpace(value[:at]);target:=strings.TrimSpace(value[at+1:])
		if !entryPointName(name)||target==""||len(target)>2048{return nil,ErrInvalid}
		if section!="console_scripts"&&section!="gui_scripts"{continue}
		if scripts[strings.ToLower(name)]||len(result)>=256{return nil,ErrInvalid}
		parts:=strings.Split(target,":")
		if len(parts)!=2||!pythonIdentifierPath(parts[0])||!pythonIdentifierPath(parts[1]){return nil,ErrInvalid}
		scripts[strings.ToLower(name)]=true
		result=append(result,EntryPoint{Group:section,Name:name,Value:target})
	}
	return result,nil
}
