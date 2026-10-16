//go:build !windows

package api

import "context"

type debianBinaryTransportNative struct{}
func newDebianBinaryTransportNative(*debianBinaryNativeTransport) *debianBinaryTransportNative { return nil }
func (o *debianProofNativeOwner) duplicateBinaryOriginalAlias(context.Context, *debianBinaryNativeTransport, uint16) error { return errDebianProofCensus }
func (o *debianProofNativeOwner) settleBinaryReadTransport(context.Context, *debianBinaryNativeTransport) error { return errDebianProofCensus }
