package documentinputs

import (
	"bytes"
	"encoding/binary"
	"io"
)

const maxWheelMembers = 32768

func preflightReadAt(reader io.ReaderAt,at int64,out []byte) error {
	for len(out)>0 {
		part:=len(out);if part>readBlock{part=readBlock}
		n,e:=reader.ReadAt(out[:part],at)
		if n!=part||e!=nil&&e!=io.EOF{return ErrInvalid}
		at+=int64(n);out=out[n:]
	}
	return nil
}

// preflightWheelDirectory bounds central-directory entry count and name bytes
// before archive/zip allocates a File for every untrusted central entry. ZIP64,
// spanned archives, and self-extracting prefixes are outside this input role.
func preflightWheelDirectory(reader io.ReaderAt,size int64) error {
	if reader==nil||size<22||size>maxWheelBytes{return ErrInvalid}
	tailSize:=size;if tailSize>22+65535{tailSize=22+65535}
	tail:=make([]byte,tailSize)
	if preflightReadAt(reader,size-tailSize,tail)!=nil{return ErrInvalid}
	end:=-1
	for i:=len(tail)-22;i>=0;i-- {
		if !bytes.Equal(tail[i:i+4],[]byte("PK\x05\x06")){continue}
		if i+22+int(binary.LittleEndian.Uint16(tail[i+20:i+22]))==len(tail){end=i;break}
	}
	if end<0{return ErrInvalid}
	record:=tail[end:end+22]
	if binary.LittleEndian.Uint16(record[4:6])!=0||binary.LittleEndian.Uint16(record[6:8])!=0{return ErrInvalid}
	count:=int(binary.LittleEndian.Uint16(record[10:12]))
	if count<1||count>maxWheelMembers||int(binary.LittleEndian.Uint16(record[8:10]))!=count{return ErrInvalid}
	dirSize:=int64(binary.LittleEndian.Uint32(record[12:16]))
	dirOffset:=int64(binary.LittleEndian.Uint32(record[16:20]))
	endOffset:=size-tailSize+int64(end)
	if dirSize<46||dirOffset<0||dirOffset+dirSize!=endOffset{return ErrInvalid}
	position:=dirOffset;var nameBytes int64
	for i:=0;i<count;i++ {
		if position+46>endOffset{return ErrInvalid}
		var header [46]byte
		if preflightReadAt(reader,position,header[:])!=nil||!bytes.Equal(header[:4],[]byte("PK\x01\x02")){return ErrInvalid}
		name:=int64(binary.LittleEndian.Uint16(header[28:30]))
		extra:=int64(binary.LittleEndian.Uint16(header[30:32]))
		comment:=int64(binary.LittleEndian.Uint16(header[32:34]))
		nameBytes+=name
		if name<1||name>1024||nameBytes>16<<20{return ErrInvalid}
		position+=46+name+extra+comment
		if position>endOffset{return ErrInvalid}
	}
	if position!=endOffset{return ErrInvalid}
	return nil
}
