package documentinputs

import (
	"bytes"
	"context"
	"encoding/binary"
	"hash/crc32"
	"io"
)

const (
	maxXZIndexBytes int64 = 1 << 20
	maxXZBlocks = 4096
	maxXZStreams = 64
	maxXZChunks = 65536
maxXZPaddingWords = 1024
	maxXZDictBytes uint64 = 64 << 20
)

type xzRecord struct { unpadded,uncompressed uint64 }

func xzReadAt(ctx context.Context,reader io.ReaderAt,offset int64,out []byte) error {
	for len(out)>0 {
		if ctx.Err()!=nil{return ErrInvalid}
		part:=len(out);if part>readBlock{part=readBlock}
		n,e:=reader.ReadAt(out[:part],offset)
		if n!=part||e!=nil&&e!=io.EOF{return ErrInvalid}
		offset+=int64(n);out=out[n:]
	}
	return nil
}

func xzVLI(raw []byte,position *int)(uint64,bool){
	var value uint64
	for i:=0;i<9;i++{
		if *position>=len(raw){return 0,false}
		part:=raw[*position];*position=*position+1
		if i==8&&part>0x7f{return 0,false}
		value|=uint64(part&0x7f)<<uint(7*i)
		if part&0x80==0 {
			if i>0&&part==0{return 0,false}
			return value,true
		}
	}
	return 0,false
}

func xzIndex(raw []byte)([]xzRecord,error){
	if len(raw)<8||raw[0]!=0||crc32.ChecksumIEEE(raw[:len(raw)-4])!=binary.LittleEndian.Uint32(raw[len(raw)-4:]){return nil,ErrInvalid}
	end:=len(raw)-4;at:=1
	count,ok:=xzVLI(raw[:end],&at)
	if !ok||count<1||count>maxXZBlocks{return nil,ErrInvalid}
	records:=make([]xzRecord,0,int(count))
	for i:=uint64(0);i<count;i++{
		unpadded,ok:=xzVLI(raw[:end],&at);if !ok||unpadded<8{return nil,ErrInvalid}
		uncompressed,ok:=xzVLI(raw[:end],&at);if !ok||uncompressed<1{return nil,ErrInvalid}
		records=append(records,xzRecord{unpadded:unpadded,uncompressed:uncompressed})
	}
	for _,value:=range raw[at:end]{if value!=0{return nil,ErrInvalid}}
	return records,nil
}

func xzCheckSize(flag byte)(int64,bool){
	switch flag {case 0:return 0,true;case 1:return 4,true;case 4:return 8,true;case 10:return 32,true}
	return 0,false
}

func xzBlockHeader(ctx context.Context,reader io.ReaderAt,at int64,record xzRecord,checkSize int64)(int64,int64,error) {
	var first [1]byte
	if xzReadAt(ctx,reader,at,first[:])!=nil||first[0]==0{return 0,0,ErrInvalid}
	headerSize:=(int64(first[0])+1)*4
	if headerSize<8||uint64(headerSize)+uint64(checkSize)>=record.unpadded{return 0,0,ErrInvalid}
	header:=make([]byte,headerSize)
	if xzReadAt(ctx,reader,at,header)!=nil||crc32.ChecksumIEEE(header[:len(header)-4])!=binary.LittleEndian.Uint32(header[len(header)-4:]){return 0,0,ErrInvalid}
	flags:=header[1]
	if flags&0x3f!=0{return 0,0,ErrInvalid} // exactly one filter and no reserved flags
	body:=header[:len(header)-4];offset:=2
	compressed:=record.unpadded-uint64(headerSize)-uint64(checkSize)
	if flags&0x40!=0{amount,ok:=xzVLI(body,&offset);if !ok||amount!=compressed{return 0,0,ErrInvalid}}
	if flags&0x80!=0{amount,ok:=xzVLI(body,&offset);if !ok||amount!=record.uncompressed{return 0,0,ErrInvalid}}
	filterID,ok:=xzVLI(body,&offset);if !ok||filterID!=0x21{return 0,0,ErrInvalid}
	properties,ok:=xzVLI(body,&offset);if !ok||properties!=1||offset>=len(body){return 0,0,ErrInvalid}
	property:=body[offset];offset++
	if property>40{return 0,0,ErrInvalid}
	dict:=uint64(2|int(property&1))<<uint(property/2+11)
	if dict>maxXZDictBytes{return 0,0,ErrInvalid}
	for _,value:=range body[offset:]{if value!=0{return 0,0,ErrInvalid}}
	return headerSize,int64(compressed),nil
}

// The LZMA2 chunk lengths must consume the indexed compressed block exactly.
// Otherwise the decoder could encounter a second block header at an offset
// different from the one inspected above before its index mismatch is known.
func xzLZMA2Chunks(ctx context.Context,reader io.ReaderAt,at,length int64) error {
	end:=at+length
	for chunks:=0;at<end&&chunks<maxXZChunks;chunks++ {
		var control [1]byte
		if xzReadAt(ctx,reader,at,control[:])!=nil{return ErrInvalid}
		at++
		if control[0]==0 {if at!=end{return ErrInvalid};return nil}
		headerLen:=2
		if control[0]>=0x80 {headerLen=4;if control[0]>=0xc0{headerLen=5}} else if control[0]!=1&&control[0]!=2{return ErrInvalid}
		if at+int64(headerLen)>end{return ErrInvalid}
		var header [5]byte
		if xzReadAt(ctx,reader,at,header[:headerLen])!=nil{return ErrInvalid}
		var payload int64
		if control[0]>=0x80 {payload=int64(binary.BigEndian.Uint16(header[2:4]))+1} else {payload=int64(binary.BigEndian.Uint16(header[:2]))+1}
		at+=int64(headerLen)
		if payload>end-at{return ErrInvalid}
		at+=payload
	}
	return ErrInvalid
}

// preflightXZ walks every stream footer, index record and block header before
// constructing the upstream decoder. The selected upstream ReaderConfig.DictCap
// is a default, not a maximum: the decoder raises it to the declared LZMA2
// dictionary. Therefore no decoder may see this input until every block's
// CRC-validated header declares a dictionary at or below maxXZDictBytes.
func preflightXZ(ctx context.Context,reader io.ReaderAt,size,maxExpanded int64) error {
	if ctx==nil||ctx.Err()!=nil||reader==nil||size<36||size>maxDebBytes||maxExpanded<1{return ErrInvalid}
	position:=size;streams:=0;blocks:=0;var expanded uint64
	for position>0 {
		padding:=0
		for position>=4 {
			var four [4]byte
			if xzReadAt(ctx,reader,position-4,four[:])!=nil{return ErrInvalid}
			if !bytes.Equal(four[:],[]byte{0,0,0,0}){break}
			position-=4;padding++
			if padding>maxXZPaddingWords{return ErrInvalid}
		}
		if position==0||position<12{return ErrInvalid}
		var footer [12]byte
		if xzReadAt(ctx,reader,position-12,footer[:])!=nil||string(footer[10:])!="YZ"||footer[8]!=0||crc32.ChecksumIEEE(footer[4:10])!=binary.LittleEndian.Uint32(footer[:4]){return ErrInvalid}
		checkSize,ok:=xzCheckSize(footer[9]);if !ok{return ErrInvalid}
		indexSize:=(int64(binary.LittleEndian.Uint32(footer[4:8]))+1)*4
		if indexSize<8||indexSize>maxXZIndexBytes||indexSize>position-12{return ErrInvalid}
		indexStart:=position-12-indexSize
		index:=make([]byte,indexSize)
		if xzReadAt(ctx,reader,indexStart,index)!=nil{return ErrInvalid}
		records,e:=xzIndex(index);if e!=nil{return ErrInvalid}
		blocks+=len(records);if blocks>maxXZBlocks{return ErrInvalid}
		var padded int64
		for _,record:=range records{
			if record.unpadded>uint64(size)||record.uncompressed>uint64(maxExpanded)-expanded{return ErrInvalid}
			expanded+=record.uncompressed
			amount:=int64((record.unpadded+3)&^uint64(3))
			if amount>indexStart-12-padded{return ErrInvalid}
			padded+=amount
		}
		streamStart:=indexStart-padded-12
		if streamStart<0{return ErrInvalid}
		var header [12]byte
		if xzReadAt(ctx,reader,streamStart,header[:])!=nil||!bytes.Equal(header[:6],[]byte{0xfd,'7','z','X','Z',0})||header[6]!=0||header[7]!=footer[9]||crc32.ChecksumIEEE(header[6:8])!=binary.LittleEndian.Uint32(header[8:12]){return ErrInvalid}
		at:=streamStart+12
		for _,record:=range records{
			headerSize,compressed,e:=xzBlockHeader(ctx,reader,at,record,checkSize)
			if e!=nil||xzLZMA2Chunks(ctx,reader,at+headerSize,compressed)!=nil{return ErrInvalid}
			blockEnd:=at+int64((record.unpadded+3)&^uint64(3))
			if padding:=blockEnd-at-int64(record.unpadded);padding>0{
				var zero [3]byte
				if xzReadAt(ctx,reader,at+headerSize+compressed,zero[:int(padding)])!=nil{return ErrInvalid}
				for _,value:=range zero[:int(padding)]{if value!=0{return ErrInvalid}}
			}
			at=blockEnd
		}
		if at!=indexStart{return ErrInvalid}
		position=streamStart;streams++
		if streams>maxXZStreams{return ErrInvalid}
	}
	if streams==0{return ErrInvalid}
	return nil
}
