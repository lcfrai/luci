//go:build !windows

package api

import "context"

type debianBinarySpoolNative struct{}
func newDebianBinarySpoolSet(context.Context,*debianProofNativeOwner,string,debianBinaryRawDecoder)(*debianBinarySpoolSet,error){return nil,errDebianBinarySpool}
func(s *debianBinarySpoolSet)persistFrame(context.Context,debianBinaryRawView)error{return errDebianBinarySpool}
func(s *debianBinarySpoolSet)readStderrOnce(context.Context)error{return errDebianBinarySpool}
func(s *debianBinarySpoolSet)finishRawSpool(context.Context)error{return errDebianBinarySpool}
func(s *debianBinarySpoolSet)closeRetiredRaw(context.Context)error{return errDebianBinarySpool}
func(s *debianBinarySpoolSet)transferRetainedNamespace(context.Context)error{return errDebianBinarySpool}
