//go:build !windows

package api

import (
 "context"
 "github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
)
func newDebianBinaryProofNativeOwner(context.Context)*debianProofNativeOwner{return &debianProofNativeOwner{}}
func(o *debianProofNativeOwner)positiveBinaryCompletion(debianProofChildResult)bool{return false}
func(o *debianProofNativeOwner)binaryChild(context.Context,*API,*debianProducerPlan,*debianBinaryNativeTransport,*debianBinarySpoolSet)(debianProofChildResult,error){return debianProofChildResult{},runtimeinstall.ErrInputProofUnavailable}
