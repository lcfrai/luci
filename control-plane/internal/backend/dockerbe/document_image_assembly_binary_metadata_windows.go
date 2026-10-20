//go:build windows

package dockerbe

import (
 "context"
 "github.com/lcfrai/luci/control-plane/internal/backend"
)

func assemblyBinaryOriginalIdentity(ctx context.Context,b assemblyBinaryOriginalBinding)(backend.DocumentImageAssemblyBinaryIdentity,error){
 native,err:=lookupAssemblyBinaryOriginal(ctx,b);if err!=nil{return backend.DocumentImageAssemblyBinaryIdentity{},err}
 v:=native.info
 return backend.DocumentImageAssemblyBinaryIdentity{FileType:1,FileAttributes:v.FileAttributes,VolumeSerial:v.VolumeSerialNumber,FileIndex:uint64(v.FileIndexHigh)<<32|uint64(v.FileIndexLow),Bytes:uint64(v.FileSizeHigh)<<32|uint64(v.FileSizeLow),CreationFileTime:uint64(v.CreationTime.HighDateTime)<<32|uint64(v.CreationTime.LowDateTime),LastWriteFileTime:uint64(v.LastWriteTime.HighDateTime)<<32|uint64(v.LastWriteTime.LowDateTime),Links:v.NumberOfLinks},nil
}
