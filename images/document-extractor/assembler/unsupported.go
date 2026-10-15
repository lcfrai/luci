//go:build !linux

package main

import "context"

func assemble(context.Context, DocumentImageAssemblyManifest, string) error { return errRefused }
