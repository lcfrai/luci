//go:build !windows

package api

import (
 "context"
 "time"
 "github.com/lcfrai/luci/control-plane/internal/backend"
 "github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
)

// No process/native helper fallback on unsupported platforms.
type debianNativeSample struct {At time.Time `json:"at"`;Physical,Commit uint64;CPU float64;Known bool}
type debianProofChildResult struct {Ordinal int `json:"ordinal"`;PID uint32 `json:"pid"`;CreationTime uint64 `json:"creationTime"`;Started,Ended time.Time;Exit uint32 `json:"exit"`;OriginalHandleOwned,ExitConfirmed,JobEmpty,HandlesClosed,CaptureSettled bool;Admission debianNativeSample;Stdout,Stderr []byte `json:"-"`}
type debianProofNativeOwner struct {began,wholeDeadline time.Time;census *debianProofNativeCensus}
func debianProducerPlatformSupported()bool{return false}
func debianProducerHostIdentity()(uint32,uint64,error){return 0,0,runtimeinstall.ErrInputProofUnavailable}
func debianProofEnvironment(string)([]uint16,error){return nil,runtimeinstall.ErrInputProofUnavailable}
func newDebianProofNativeOwner(context.Context)*debianProofNativeOwner{return &debianProofNativeOwner{}}
func(o *debianProofNativeOwner)retire(){}
func(o *debianProofNativeOwner)workLimit()time.Time{return time.Time{}}
func(o *debianProofNativeOwner)settle()bool{return false}
func(o *debianProofNativeOwner)child(context.Context,*API,*debianProducerPlan,int,[]string)(debianProofChildResult,error){return debianProofChildResult{},runtimeinstall.ErrInputProofUnavailable}
func(o *debianProofNativeOwner)positiveCompletion([]debianProofChildResult)bool{return false}
func(o *debianProofNativeOwner)readProduced(string,int64)([]byte,error){return nil,runtimeinstall.ErrInputProofUnavailable}
func(o *debianProofNativeOwner)closeProduced()bool{return false}
func(o *debianProofNativeOwner)writeBytes(string,[]byte,int)error{return runtimeinstall.ErrInputProofUnavailable}
func(o *debianProofNativeOwner)writeEvidenceBytes(string,[]byte,int)error{return runtimeinstall.ErrInputProofUnavailable}
func(o *debianProofNativeOwner)adoptProduced(context.Context,backend.DocumentImageAssemblyEvidenceGuard,[]backend.DocumentImageAssemblyEvidenceObject)error{return runtimeinstall.ErrInputProofUnavailable}
