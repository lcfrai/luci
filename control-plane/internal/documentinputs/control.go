package documentinputs

import (
	"bytes"
	"strings"
)

func validFieldName(key string) bool {
	if len(key)==0||len(key)>80{return false}
	for _,c:=range key {
		if !(c>='A'&&c<='Z'||c>='a'&&c<='z'||c>='0'&&c<='9'||c=='-'){return false}
	}
	return true
}

// paragraphFields parses the exact Debian control/RFC822 field structure. It
// retains relation expressions rather than interpreting package satisfaction.
func paragraphFields(raw []byte, max int) ([]map[string]string,error) {
	if len(raw)==0||len(raw)>max{return nil,ErrInvalid}
	rows:=[]map[string]string{};current:=map[string]string{};last:=""
	for len(raw)>0 {
		end:=bytes.IndexByte(raw,'\n');var line []byte
		if end<0 {line=raw;raw=nil} else {line=raw[:end];raw=raw[end+1:]}
		if len(line)>1<<20{return nil,ErrInvalid}
		line=bytes.TrimSuffix(line,[]byte{'\r'})
		if len(line)==0 {
			if len(current)>0 {rows=append(rows,current);if len(rows)>20000{return nil,ErrInvalid};current=map[string]string{};last=""}
			continue
		}
		if line[0]==' '||line[0]=='\t' {
			if last==""{return nil,ErrInvalid}
			current[last]+="\n"+string(bytes.TrimSpace(line));continue
		}
		at:=bytes.IndexByte(line,':');if at<1||at>80{return nil,ErrInvalid}
		key:=string(line[:at]);if !validFieldName(key){return nil,ErrInvalid}
		if _,exists:=current[key];exists{return nil,ErrInvalid}
		value:=bytes.TrimSpace(line[at+1:]);if bytes.IndexByte(value,0)>=0{return nil,ErrInvalid}
		current[key]=string(value);last=key
	}
	if len(current)>0{rows=append(rows,current)}
	if len(rows)==0{return nil,ErrInvalid}
	return rows,nil
}

func relations(raw string) []string {
	out:=[]string{}
	for _,item:=range strings.Split(raw,",") {if value:=strings.TrimSpace(item);value!=""{out=append(out,value)}}
	return out
}

func controlPackage(fields map[string]string) (Package,error) {
	name,version,architecture:=fields["Package"],fields["Version"],fields["Architecture"]
	if name==""||version==""||architecture==""||len(name)>200||len(version)>200||len(architecture)>100||strings.ContainsAny(name+version+architecture,"\x00\r\n/\\") {return Package{},ErrInvalid}
	return Package{Name:name,Version:version,Architecture:architecture,MultiArch:fields["Multi-Arch"],Source:fields["Source"],
		Depends:relations(fields["Depends"]),PreDepends:relations(fields["Pre-Depends"]),Provides:relations(fields["Provides"]),
		Breaks:relations(fields["Breaks"]),Conflicts:relations(fields["Conflicts"]),
		BuiltUsing:relations(fields["Built-Using"]),StaticBuiltUsing:relations(fields["Static-Built-Using"])},nil
}
