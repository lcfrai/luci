package hubapi

import (
	"errors"
	"reflect"
	"strconv"
	"strings"
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
func RequireDigests(req any) error {
	v := reflect.ValueOf(req)
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	if !v.IsValid() {
		return nil
	}
	return requireDigests(v, "")
}

func requireDigests(v reflect.Value, path string) error {
	switch {
	case v.Type() == digestType:
		if v.Interface().(Digest).IsZero() {
			return &FieldError{Field: path, Err: ErrRequired}
		}
	case v.Kind() == reflect.Struct:
		t := v.Type()
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
			if !f.IsExported() || name == "-" {
				continue
			}
			if name == "" {
				name = f.Name
			}
			if err := requireDigests(v.Field(i), join(path, name)); err != nil {
				return err
			}
		}
	case v.Kind() == reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			if err := requireDigests(v.Index(i), path+"["+strconv.Itoa(i)+"]"); err != nil {
				return err
			}
		}
	}
	return nil
}
