package product

import (
	"context"
	"encoding/binary"
	"errors"
	"math"
	"sort"

	subsetfont "github.com/tdewolff/font"
)

// CID-keyed CFF charset entries are CIDs, not String INDEX SIDs. The pinned
// subsetter treats these as names. This adapter keeps original Type2 programs
// and their global/local subroutines, selects their exact FD/private dictionaries,
// and writes a new sequential CID charset and FDSelect. No GID/CID identity is
// inferred from an Adobe-Identity ROS or from the original cmap.
type pdfCFFEntry struct { op int; raw []byte; values []int; integers bool }

func cffIndex(data []byte, at int) ([][]byte,int,error) {
	bad:=errors.New("invalid bounded CFF INDEX")
	if at<0 || at>len(data)-2 { return nil,0,bad }
	n:=int(binary.BigEndian.Uint16(data[at:]));at+=2
	if n==0 { return nil,at,nil }
	if at>=len(data) {return nil,0,bad}
	size:=int(data[at]);at++
	if size<1 || size>4 || (n+1)>((len(data)-at)/size) {return nil,0,bad}
	offsets:=make([]int,n+1)
	for i:=range offsets {
		v:=uint32(0)
		for j:=0;j<size;j++ {v=v<<8|uint32(data[at+i*size+j])}
		if uint64(v)>uint64(len(data)) {return nil,0,bad};offsets[i]=int(v)
		if offsets[i]<1 || i>0 && offsets[i]<offsets[i-1] {return nil,0,bad}
	}
	if offsets[0]!=1 {return nil,0,bad}
	start:=at+(n+1)*size
	if offsets[n]-1>len(data)-start {return nil,0,bad}
	items:=make([][]byte,n)
	for i:=range items {items[i]=data[start+offsets[i]-1:start+offsets[i+1]-1]}
	return items,start+offsets[n]-1,nil
}

func cffMakeIndex(items [][]byte) ([]byte,error) {
	if len(items)>65535 {return nil,errors.New("CFF INDEX count exceeded")}
	if len(items)==0 {return []byte{0,0},nil}
	total:=1
	for _,item:=range items {if len(item)>pdfFontLimit-total{return nil,errors.New("CFF INDEX bytes exceeded")};total+=len(item)}
	// Four-byte offsets keep dictionary layout stable while offsets are patched.
	out:=make([]byte,3+4*(len(items)+1),3+4*(len(items)+1)+total-1)
	binary.BigEndian.PutUint16(out,uint16(len(items)));out[2]=4
	at:=1
	for i,item:=range items {binary.BigEndian.PutUint32(out[3+4*i:],uint32(at));at+=len(item)}
	binary.BigEndian.PutUint32(out[3+4*len(items):],uint32(at))
	for _,item:=range items {out=append(out,item...)}
	return out,nil
}

func cffDict(data []byte) ([]pdfCFFEntry,error) {
	var entries []pdfCFFEntry
	start:=0;values:=[]int{};integers:=true
	for at:=0;at<len(data); {
		b:=data[at];at++
		if b<=21 {
			op:=int(b)
			if b==12 {if at>=len(data){return nil,errors.New("short CFF operator")};op=0x0c00|int(data[at]);at++}
			entries=append(entries,pdfCFFEntry{op:op,raw:data[start:at],values:values,integers:integers})
			start=at;values=nil;integers=true
			continue
		}
		var v int
		switch {
		case b>=32&&b<=246: v=int(b)-139
		case b>=247&&b<=250: if at>=len(data){return nil,errors.New("short CFF number")};v=(int(b)-247)*256+int(data[at])+108;at++
		case b>=251&&b<=254: if at>=len(data){return nil,errors.New("short CFF number")};v=-(int(b)-251)*256-int(data[at])-108;at++
		case b==28: if at>len(data)-2{return nil,errors.New("short CFF number")};v=int(int16(binary.BigEndian.Uint16(data[at:])));at+=2
		case b==29: if at>len(data)-4{return nil,errors.New("short CFF number")};v=int(int32(binary.BigEndian.Uint32(data[at:])));at+=4
		case b==30:
			integers=false;ended:=false
			for at<len(data) {x:=data[at];at++;if x>>4==15||x&15==15{ended=true;break}}
			if !ended{return nil,errors.New("short CFF real")}
		default:return nil,errors.New("unsupported CFF DICT number")
		}
		values=append(values,v)
		if len(values)>48{return nil,errors.New("CFF operand stack exceeded")}
	}
	if start!=len(data) {return nil,errors.New("unfinished CFF DICT")}
	return entries,nil
}

func cffValues(entries []pdfCFFEntry,op,n int) ([]int,error) {
	var result []int
	for _,entry:=range entries {if entry.op==op {if result!=nil||!entry.integers||len(entry.values)!=n{return nil,errors.New("invalid CFF dictionary field")};result=entry.values}}
	if result==nil{return nil,errors.New("missing CFF dictionary field")}
	return result,nil
}

func cffPatch(entries []pdfCFFEntry,replacements map[int][]int) ([]byte,error) {
	var out []byte
	written:=map[int]bool{}
	write:=func(op int,values []int)error{
		if written[op]{return errors.New("duplicate CFF dictionary field")};written[op]=true
		for _,v:=range values {if int64(v)<math.MinInt32||int64(v)>math.MaxInt32{return errors.New("CFF offset exceeds range")};out=append(out,29,byte(uint32(v)>>24),byte(uint32(v)>>16),byte(uint32(v)>>8),byte(v))}
		if op>255 {out=append(out,12,byte(op))}else{out=append(out,byte(op))}
		return nil
	}
	for _,entry:=range entries {if value,ok:=replacements[entry.op];ok {if err:=write(entry.op,value);err!=nil{return nil,err}}else{out=append(out,entry.raw...)}}
	keys:=make([]int,0,len(replacements));for op:=range replacements{keys=append(keys,op)};sort.Ints(keys)
	for _,op:=range keys {if !written[op]{if err:=write(op,replacements[op]);err!=nil{return nil,err}}}
	return out,nil
}

func cffCIDCharset(data []byte,at,n int) ([]uint16,error) {
	if n<1||n>65535||at<3||at>=len(data){return nil,errors.New("invalid CID charset")}
	format:=data[at];at++
	cids:=make([]uint16,n);seen:=map[uint16]bool{0:true}
	for i:=1;i<n; {
		if at>len(data)-2{return nil,errors.New("short CID charset")}
		first:=int(binary.BigEndian.Uint16(data[at:]));at+=2;count:=1
		if format==1 {if at>=len(data){return nil,errors.New("short CID range")};count+=int(data[at]);at++} else if format==2 {if at>len(data)-2{return nil,errors.New("short CID range")};count+=int(binary.BigEndian.Uint16(data[at:]));at+=2} else if format!=0 {return nil,errors.New("unsupported CID charset")}
		if count>n-i||first+count>65536{return nil,errors.New("CID range overflow")}
		for j:=0;j<count;j++ {cid:=uint16(first+j);if seen[cid]{return nil,errors.New("duplicate original CID")};seen[cid]=true;cids[i]=cid;i++}
	}
	return cids,nil
}

func cffFDSelect(data []byte,at,n,fdCount int) ([]byte,error) {
	if at<0||at>=len(data)||n<1||fdCount<1||fdCount>256{return nil,errors.New("invalid CFF FDSelect")}
	format:=data[at];at++
	result:=make([]byte,n)
	if format==0 {if n>len(data)-at{return nil,errors.New("short FDSelect")};copy(result,data[at:at+n])} else if format==3 {
		if at>len(data)-2{return nil,errors.New("short FDSelect ranges")};count:=int(binary.BigEndian.Uint16(data[at:]));at+=2
		if count<1||count>n||count>(len(data)-at-2)/3{return nil,errors.New("bad FDSelect range count")}
		previous:=0
		for i:=0;i<count;i++ {
			first:=int(binary.BigEndian.Uint16(data[at+3*i:]));fd:=data[at+3*i+2]
			next:=n
			if i+1<count{next=int(binary.BigEndian.Uint16(data[at+3*(i+1):]))}else{next=int(binary.BigEndian.Uint16(data[at+3*count:]))}
			if first!=previous||next<=first||next>n{return nil,errors.New("invalid FDSelect range")}
			for j:=first;j<next;j++{result[j]=fd};previous=next
		}
		if previous!=n{return nil,errors.New("incomplete FDSelect")}
	} else {return nil,errors.New("unsupported CFF FDSelect format")}
	for _,fd:=range result {if int(fd)>=fdCount{return nil,errors.New("unknown CFF private dictionary")}}
	return result,nil
}

func cffPrivate(data []byte,dict []pdfCFFEntry) ([]byte,int,error) {
	v,err:=cffValues(dict,18,2);if err!=nil{return nil,0,err}
	size,offset:=v[0],v[1]
	if size<0||offset<0||offset>len(data)||size>len(data)-offset{return nil,0,errors.New("invalid CFF private range")}
	entries,err:=cffDict(data[offset:offset+size]);if err!=nil{return nil,0,err}
	end:=offset+size
	for _,entry:=range entries {if entry.op==19 {
		if !entry.integers||len(entry.values)!=1||entry.values[0]<size||entry.values[0]>len(data)-offset{return nil,0,errors.New("invalid CFF local subroutine offset")}
		_,end,err=cffIndex(data,offset+entry.values[0]);if err!=nil{return nil,0,err}
	}}
	if end-offset>pdfFontLimit{return nil,0,errors.New("CFF private range exceeded")}
	return data[offset:end],size,nil
}

func subsetPDFCIDFont(ctx context.Context,sfnt *subsetfont.SFNT,gids []uint16,subsetName string,budget *pdfBudget) ([]byte,error) {
	data:=sfnt.Tables["CFF "]
	if len(data)<4||len(data)>pdfFontLimit||data[0]!=1||data[2]<4||int(data[2])>len(data)||len(gids)<1||len(gids)>65535||gids[0]!=0{return nil,errors.New("invalid CFF1 subset input")}
	names,at,err:=cffIndex(data,int(data[2]));if err!=nil||len(names)!=1{return nil,errors.New("CFF name unavailable")}
	tops,at,err:=cffIndex(data,at);if err!=nil||len(tops)!=1{return nil,errors.New("CFF top dictionary unavailable")}
	stringItems,at,err:=cffIndex(data,at);if err!=nil{return nil,err}
	// Rewritten CIDs belong to our explicit Adobe-Identity collection. Neither
	// the original ROS nor SID-based names are authority for their new values.
	if len(stringItems)>65140||len(subsetName)<8||len(subsetName)>100{return nil,errors.New("CFF subset name unavailable")}
	nameSID:=391+len(stringItems);stringItems=append(stringItems,[]byte(subsetName))
	registrySID:=391+len(stringItems);stringItems=append(stringItems,[]byte("Adobe"))
	orderingSID:=391+len(stringItems);stringItems=append(stringItems,[]byte("Identity"))
	stringsIndex,err:=cffMakeIndex(stringItems);if err!=nil{return nil,err}
	globalStart:=at;_,at,err=cffIndex(data,at);if err!=nil{return nil,err};globals:=data[globalStart:at]
	top,err:=cffDict(tops[0]);if err!=nil{return nil,err}
	if _,err=cffValues(top,0x0c1e,3);err!=nil{return nil,errors.New("PDF CFF font must have an explicit CID ROS")}
	charAt,err:=cffValues(top,17,1);if err!=nil{return nil,err}
	chars,_,err:=cffIndex(data,charAt[0]);if err!=nil||len(chars)!=int(sfnt.NumGlyphs()){return nil,errors.New("CFF glyph count mismatch")}
	charsetAt,err:=cffValues(top,15,1);if err!=nil{return nil,err}
	if _,err=cffCIDCharset(data,charsetAt[0],len(chars));err!=nil{return nil,err}
	fdAt,err:=cffValues(top,0x0c24,1);if err!=nil{return nil,err}
	fdItems,_,err:=cffIndex(data,fdAt[0]);if err!=nil{return nil,err}
	selectAt,err:=cffValues(top,0x0c25,1);if err!=nil{return nil,err}
	fdSelection,err:=cffFDSelect(data,selectAt[0],len(chars),len(fdItems));if err!=nil{return nil,err}
	selected:=make([][]byte,len(gids));newFD:=make([]byte,len(gids)+1);newFD[0]=0
	seen:=map[uint16]bool{}
	charBytes:=3+4*(len(gids)+1)
	for i,gid:=range gids {if err=ctx.Err();err!=nil{return nil,err};if int(gid)>=len(chars)||seen[gid]{return nil,errors.New("invalid subset GID")};seen[gid]=true;selected[i]=chars[gid];newFD[i+1]=fdSelection[gid];if len(chars[gid])>distributeExportLimit-charBytes{return nil,errors.New("CFF charstring subset exceeds limit")};charBytes+=len(chars[gid])}
	if err=budget.take(charBytes+len(gids)*64+len(chars)*8);err!=nil{return nil,err}
	charIndex,err:=cffMakeIndex(selected);if err!=nil{return nil,err}
	charset:=make([]byte,1+2*(len(gids)-1));for i:=1;i<len(gids);i++{binary.BigEndian.PutUint16(charset[1+2*(i-1):],uint16(i))}
	privateData:=make([][]byte,len(fdItems));privateSizes:=make([]int,len(fdItems));fdDicts:=make([][]pdfCFFEntry,len(fdItems));fdDummy:=make([][]byte,len(fdItems))
	privateTotal:=0
	for i,item:=range fdItems {
		if err=ctx.Err();err!=nil{return nil,err}
		fdDicts[i],err=cffDict(item);if err!=nil{return nil,err}
		privateData[i],privateSizes[i],err=cffPrivate(data,fdDicts[i]);if err!=nil{return nil,err}
		privateTotal+=len(privateData[i]);if privateTotal>pdfFontLimit{return nil,errors.New("CFF private data exceeds limit")}
		fdDummy[i],err=cffPatch(fdDicts[i],map[int][]int{18:{privateSizes[i],0},0x0c26:{nameSID}});if err!=nil{return nil,err}
	}
	fdIndex,err:=cffMakeIndex(fdDummy);if err!=nil{return nil,err}
	nameIndex,err:=cffMakeIndex([][]byte{[]byte(subsetName)});if err!=nil{return nil,err}
	changes:=map[int][]int{2:{nameSID},3:{nameSID},15:{0},17:{0},0x0c1e:{registrySID,orderingSID,0},0x0c24:{0},0x0c25:{0},0x0c22:{len(gids)}}
	topBytes,err:=cffPatch(top,changes);if err!=nil{return nil,err}
	topIndex,err:=cffMakeIndex([][]byte{topBytes});if err!=nil{return nil,err}
	base:=4+len(nameIndex)+len(topIndex)+len(stringsIndex)+len(globals)
	changes[15]=[]int{base};changes[17]=[]int{base+len(charset)};changes[0x0c25]=[]int{base+len(charset)+len(charIndex)};changes[0x0c24]=[]int{base+len(charset)+len(charIndex)+len(newFD)}
	privateAt:=changes[0x0c24][0]+len(fdIndex)
	for i:=range fdDummy {fdDummy[i],err=cffPatch(fdDicts[i],map[int][]int{18:{privateSizes[i],privateAt},0x0c26:{nameSID}});if err!=nil{return nil,err};privateAt+=len(privateData[i])}
	if privateAt>distributeExportLimit{return nil,errors.New("CFF subset bytes exceeded")}
	if err=budget.take(privateAt);err!=nil{return nil,err}
	fdIndex,err=cffMakeIndex(fdDummy);if err!=nil{return nil,err}
	topBytes,err=cffPatch(top,changes);if err!=nil{return nil,err};topIndex,err=cffMakeIndex([][]byte{topBytes});if err!=nil{return nil,err}
	out:=make([]byte,0,privateAt);out=append(out,1,0,4,4)
	for _,part:=range [][]byte{nameIndex,topIndex,stringsIndex,globals,charset,charIndex,newFD,fdIndex}{out=append(out,part...)}
	for _,part:=range privateData{out=append(out,part...)}
	if len(out)!=privateAt{return nil,errors.New("CFF relocated offset mismatch")}
	actualCIDs,err:=cffCIDCharset(out,changes[15][0],len(gids));if err!=nil{return nil,err}
	for i,cid:=range actualCIDs{if int(cid)!=i{return nil,errors.New("CFF CID rewrite failed")}}
	return pdfCFFSFNT(ctx,sfnt,gids,out,subsetName,budget)
}

func pdfCFFSFNT(ctx context.Context,original *subsetfont.SFNT,gids []uint16,cff []byte,subsetName string,budget *pdfBudget) ([]byte,error) {
	tables:=map[string][]byte{"CFF ":cff}
	for _,tag:=range []string{"head","hhea","maxp","OS/2"} {if data:=original.Tables[tag];data!=nil{if err:=budget.take(len(data));err!=nil{return nil,err};tables[tag]=append([]byte(nil),data...)}}
	tables["name"]=pdfSubsetNameTable(subsetName)
	if len(tables["head"])<54||len(tables["hhea"])<36||len(tables["maxp"])<6{return nil,errors.New("CFF SFNT metrics unavailable")}
	clear(tables["head"][8:12]);binary.BigEndian.PutUint16(tables["hhea"][34:],uint16(len(gids)));binary.BigEndian.PutUint16(tables["maxp"][4:],uint16(len(gids)))
	if err:=budget.take(len(gids)*4);err!=nil{return nil,err}
	hmtx:=make([]byte,len(gids)*4)
	mapping:=make(map[rune]uint32)
	for i,gid:=range gids {
		if err:=ctx.Err();err!=nil{return nil,err}
		binary.BigEndian.PutUint16(hmtx[4*i:],original.GlyphAdvance(gid));binary.BigEndian.PutUint16(hmtx[4*i+2:],uint16(original.Hmtx.LeftSideBearing(gid)))
		for _,r:=range original.Cmap.ToUnicode(gid){if r>=0&&r<=0x10ffff{if _,exists:=mapping[r];!exists{if err:=budget.take(64);err!=nil{return nil,err}};mapping[r]=uint32(i)}}
	}
	tables["hmtx"]=hmtx
	runes:=make([]rune,0,len(mapping));for r:=range mapping{runes=append(runes,r)};sort.Slice(runes,func(i,j int)bool{return runes[i]<runes[j]})
	if len(runes)>1<<20{return nil,errors.New("subset cmap exceeded")}
	if err:=budget.take(28+16*len(runes));err!=nil{return nil,err}
	cmap:=make([]byte,12+16+12*len(runes));binary.BigEndian.PutUint16(cmap[2:],1);binary.BigEndian.PutUint16(cmap[4:],3);binary.BigEndian.PutUint16(cmap[6:],10);binary.BigEndian.PutUint32(cmap[8:],12)
	binary.BigEndian.PutUint16(cmap[12:],12);binary.BigEndian.PutUint32(cmap[16:],uint32(len(cmap)-12));binary.BigEndian.PutUint32(cmap[24:],uint32(len(runes)))
	for i,r:=range runes{at:=28+12*i;binary.BigEndian.PutUint32(cmap[at:],uint32(r));binary.BigEndian.PutUint32(cmap[at+4:],uint32(r));binary.BigEndian.PutUint32(cmap[at+8:],mapping[r])}
	tables["cmap"]=cmap
	post:=make([]byte,32);binary.BigEndian.PutUint32(post,0x00030000);tables["post"]=post
	tags:=make([]string,0,len(tables));for tag:=range tables{tags=append(tags,tag)};sort.Strings(tags)
	total:=12+16*len(tags);for _,tag:=range tags{n:=len(tables[tag]);if n>distributeExportLimit-total-3{return nil,errors.New("standalone CFF font exceeded")};total+=(n+3)&^3}
	if err:=budget.take(total);err!=nil{return nil,err}
	out:=make([]byte,12+16*len(tags),total);copy(out,"OTTO");binary.BigEndian.PutUint16(out[4:],uint16(len(tags)))
	power,log:=1,0;for power*2<=len(tags){power*=2;log++};binary.BigEndian.PutUint16(out[6:],uint16(power*16));binary.BigEndian.PutUint16(out[8:],uint16(log));binary.BigEndian.PutUint16(out[10:],uint16(len(tags)*16-power*16))
	headAt:=0
	for i,tag:=range tags {
		body:=tables[tag];at:=len(out);if tag=="head"{headAt=at}
		if len(body)>pdfFontLimit-at-3{return nil,errors.New("standalone CFF font exceeded")}
		entry:=12+16*i;copy(out[entry:],tag);binary.BigEndian.PutUint32(out[entry+4:],pdfSFNTChecksum(body));binary.BigEndian.PutUint32(out[entry+8:],uint32(at));binary.BigEndian.PutUint32(out[entry+12:],uint32(len(body)))
		out=append(out,body...);for len(out)%4!=0{out=append(out,0)}
	}
	binary.BigEndian.PutUint32(out[headAt+8:],uint32(0xb1b0afba)-pdfSFNTChecksum(out))
	return out,nil
}

func pdfSFNTChecksum(data []byte) uint32 {
	var sum uint32
	for at:=0;at<len(data);at+=4{var block [4]byte;copy(block[:],data[at:min(at+4,len(data))]);sum+=binary.BigEndian.Uint32(block[:])}
	return sum
}
