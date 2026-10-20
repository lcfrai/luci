package documentinputs

import (
	"bytes"
	"strings"
	"unicode/utf8"
)

// wheelMetadataIdentity reads only the singleton identity headers from Core
// Metadata. Other headers may repeat (notably Requires-Dist and Classifier),
// and content after the header/body separator is an uninterpreted description.
func wheelMetadataIdentity(raw []byte) (string,string,error) {
	if len(raw)==0||len(raw)>1<<20||!utf8.Valid(raw)||bytes.IndexByte(raw,0)>=0{return "","",ErrInvalid}
	var name,version,last string
	body:=false
	for _,line:=range bytes.Split(raw,[]byte{'\n'}) {
		line=bytes.TrimSuffix(line,[]byte{'\r'})
		if body {continue}
		if len(line)==0 {body=true;continue}
		if line[0]==' '||line[0]=='\t' {
			if last==""||last=="name"||last=="version" {return "","",ErrInvalid}
			continue
		}
		at:=bytes.IndexByte(line,':')
		if at<1||at>80||!validFieldName(string(line[:at])){return "","",ErrInvalid}
		key:=strings.ToLower(string(line[:at]));value:=bytes.TrimSpace(line[at+1:])
		if bytes.IndexByte(value,'\r')>=0||bytes.IndexByte(value,'\n')>=0{return "","",ErrInvalid}
		switch key {
		case "name":
			if name!=""||len(value)==0||len(value)>128{return "","",ErrInvalid}
			name=string(value)
		case "version":
			if version!=""||len(value)==0||len(value)>128{return "","",ErrInvalid}
			version=string(value)
		}
		last=key
	}
	if name==""||version=="" {return "","",ErrInvalid}
	return name,version,nil
}
