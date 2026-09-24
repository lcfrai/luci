package hubapi

import (
	"errors"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

// ErrRequired is a required field that was absent or null. The Hub answers it
// 400 bad_request with details.field, like every FieldError (contract §4.2).
var ErrRequired = errors.New("hubapi: a required field is absent or null")

var digestType = reflect.TypeFor[Digest]()

// RequireDigests returns a *FieldError wrapping ErrRequired for the first zero
// Digest in req, in field order, or nil. req is a request struct or a pointer
// to one. Field is the JSON path: "criterionDigest", "snapshot.blobs[1]".
//
// Every value Digest in a request type is required; an optional one would be a
// *Digest, which is not descended. encoding/json leaves an absent or null
// Digest at Digest{}, which marshals as sha256:000…0 and which no other check
// refuses: an evidence record would keep the fabricated digest for good
// (append-only, §7.10), and an approval's briefDigest would be answered 422
// digest_mismatch where a missing field is 400 bad_request (§4.2). The Hub's
// Decode calls this on every decoded body (task 14), so no route can forget it
// (task 01 review C1, carried to task 03).
//
// It reaches what encoding/json reaches, by the names encoding/json uses: a
// non-nil pointer to a struct is descended (a present optional object's
// digests are required), and an untagged embedded struct's fields are the
// parent's. It walks maps and interfaces not at all; no request type has one.
//
// It costs nothing on a body with nothing wrong (task 03 review I1): a type
// that cannot hold a Digest — a revision's content, up to 64 KiB of bytes — is
// never entered, and the path is built only on the way back from a refusal.
// Route 26 decodes holding the org's write lock (task 19).
func RequireDigests(req any) error {
	v := reflect.ValueOf(req)
	if !v.IsValid() {
		return nil
	}
	if fe := requireDigests(v); fe != nil {
		return fe
	}
	return nil
}

func requireDigests(v reflect.Value) *FieldError {
	t := v.Type()
	if !holdsDigest(t) {
		return nil
	}
	if t == digestType {
		if digestAt(v).IsZero() {
			return &FieldError{Err: ErrRequired}
		}
		return nil
	}
	switch t.Kind() {
	case reflect.Pointer:
		if !v.IsNil() {
			return requireDigests(v.Elem())
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			if fe := requireDigests(v.Index(i)); fe != nil {
				fe.Field = "[" + strconv.Itoa(i) + "]" + below(fe.Field)
				return fe
			}
		}
	case reflect.Struct:
		for _, f := range fieldsOf(t) {
			if fe := requireDigests(v.Field(f.index)); fe != nil {
				if f.name != "" {
					fe.Field = f.name + below(fe.Field)
				}
				return fe
			}
		}
	}
	return nil
}

// below is how a child's path continues its parent's: "" (the child is the
// field), "[1]…" (an element), or ".manifest…" (a member).
func below(child string) string {
	if child == "" || child[0] == '[' {
		return child
	}
	return "." + child
}

// digestAt reads the Digest v holds without boxing it when v is addressable,
// which every value under a pointer is.
func digestAt(v reflect.Value) Digest {
	if v.CanAddr() {
		return *v.Addr().Interface().(*Digest)
	}
	return v.Interface().(Digest)
}

// digestField is one struct field RequireDigests enters: its index, and its
// JSON name, "" for an embedded struct whose fields are the parent's.
type digestField struct {
	index int
	name  string
}

var (
	holds  sync.Map // reflect.Type → bool
	fields sync.Map // struct reflect.Type → []digestField
)

// holdsDigest reports whether a value of t can hold a required Digest.
func holdsDigest(t reflect.Type) bool {
	if b, ok := holds.Load(t); ok {
		return b.(bool)
	}
	b := holdsDigestIn(t, map[reflect.Type]bool{})
	holds.Store(t, b)
	return b
}

func holdsDigestIn(t reflect.Type, visiting map[reflect.Type]bool) bool {
	if t == digestType {
		return true
	}
	if visiting[t] {
		return false // a recursive type: its other fields decide
	}
	visiting[t] = true
	switch t.Kind() {
	case reflect.Pointer:
		return t.Elem() != digestType && holdsDigestIn(t.Elem(), visiting)
	case reflect.Slice, reflect.Array:
		return holdsDigestIn(t.Elem(), visiting)
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			if _, ok := jsonName(t.Field(i)); ok && holdsDigestIn(t.Field(i).Type, visiting) {
				return true
			}
		}
	}
	return false
}

// fieldsOf lists the fields of struct type t that can hold a Digest, cached:
// reflect's Type.Field allocates, and RequireDigests runs on every body.
func fieldsOf(t reflect.Type) []digestField {
	if fs, ok := fields.Load(t); ok {
		return fs.([]digestField)
	}
	var fs []digestField
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if name, ok := jsonName(f); ok && holdsDigest(f.Type) {
			fs = append(fs, digestField{i, name})
		}
	}
	fields.Store(t, fs)
	return fs
}

// jsonName is f's JSON name as encoding/json decodes it, "" for an untagged
// embedded struct (or pointer to one), whose fields it promotes; ok is false
// for a field encoding/json never fills.
func jsonName(f reflect.StructField) (name string, ok bool) {
	tag := f.Tag.Get("json")
	if tag == "-" {
		return "", false
	}
	name, _, _ = strings.Cut(tag, ",")
	ft := f.Type
	if ft.Kind() == reflect.Pointer {
		ft = ft.Elem()
	}
	if f.Anonymous && name == "" && ft.Kind() == reflect.Struct {
		return "", true
	}
	if !f.IsExported() {
		return "", false
	}
	if name == "" {
		name = f.Name
	}
	return name, true
}
