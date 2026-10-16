//go:build !windows

package api

import (
 "context"
 "github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
)

func(o *debianProofNativeOwner)nativeRetired()bool{return true}
func(o *debianProofNativeOwner)startObserver(context.Context,*debianProofQueryAdmission)error{return runtimeinstall.ErrInputProofUnavailable}
func(o *debianProofNativeOwner)initialAdmission(context.Context)error{return runtimeinstall.ErrInputProofUnavailable}
