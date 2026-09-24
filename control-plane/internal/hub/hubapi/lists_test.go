package hubapi

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// findNil reports the first nil slice or map reachable from v, a json.RawMessage
// excepted: its nil is the contract's null (an erased revision's content).
func findNil(v reflect.Value, path string) (string, bool) {
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if !v.IsNil() {
			return findNil(v.Elem(), path)
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if f := v.Type().Field(i); f.IsExported() {
				if p, ok := findNil(v.Field(i), path+"."+f.Name); ok {
					return p, true
				}
			}
		}
	case reflect.Map:
		if v.IsNil() {
			return path, true
		}
	case reflect.Slice:
		if v.Type() == reflect.TypeOf(json.RawMessage(nil)) {
			return "", false
		}
		if v.IsNil() {
			return path, true
		}
		for i := 0; i < v.Len(); i++ {
			if p, ok := findNil(v.Index(i), path+"[]"); ok {
				return p, true
			}
		}
	}
	return "", false
}

func TestEnsureListsWritesEmptyNotNull(t *testing.T) {
	for _, c := range []struct {
		v    any
		want []string
	}{
		{&Project{}, []string{`"remotes":[]`}},
		{&Release{}, []string{`"deployments":[]`, `"environment":null`}},
		{&Event{}, []string{`"data":{}`, `"actor":null`}},
		{&Page[Team]{}, []string{`"items":[]`, `"next":null`}},
		{&DesignApproval{}, []string{`"blobs":[]`}}, // Snapshot.Blobs, one struct down
		{&RefsResponse{}, []string{`"results":[]`, `"heads":{}`}},
		{&AuditPage{Events: []AuditEvent{{}}}, []string{`"events":[{`, `"data":{}`}}, // an embedded Event in an element
		{&Page[Release]{Items: []Release{{}}}, []string{`"deployments":[]`}},
		{&RefsResponse{Results: []RefResult{{Approval: &DesignApproval{}}}}, []string{`"blobs":[]`}}, // through a pointer
		{&BriefRevision{}, []string{`"content":null`}},                                               // RawMessage stays nil
		{&BriefWithRevision{Revision: &BriefRevision{}}, []string{`"content":null`, `"latestComplete":null`}},
		{&Project{Remotes: []Remote{{RepoKey: "github.com/a/b"}}}, []string{`"remotes":[{"repoKey":"github.com/a/b"`}},
	} {
		EnsureLists(c.v)
		b, err := json.Marshal(c.v)
		if err != nil {
			t.Fatalf("json.Marshal(%T) after EnsureLists: %v", c.v, err)
		}
		for _, w := range c.want {
			if !strings.Contains(string(b), w) {
				t.Errorf("%T after EnsureLists marshals\n %s\nwant it to hold %s", c.v, b, w)
			}
		}
	}
	// Every response type: nothing reachable stays a nil slice or map.
	for _, v := range responseTypes {
		p := reflect.New(reflect.TypeOf(v))
		EnsureLists(p.Interface())
		if path, ok := findNil(p, reflect.TypeOf(v).Name()); ok {
			t.Errorf("EnsureLists(&%s{}) left %s nil; it would go out null", reflect.TypeOf(v).Name(), path)
		}
	}
	// A slice of them has its elements filled.
	rs := []Release{{}, {}}
	EnsureLists(rs)
	if rs[1].Deployments == nil {
		t.Errorf("EnsureLists([]Release) left an element's deployments nil")
	}
	// Nothing to do is not an error, and a nil slice or map passed by value, which
	// cannot be replaced, is left alone rather than panicking (mutant S3).
	EnsureLists(nil)
	EnsureLists((*Project)(nil))
	EnsureLists([]Release(nil))
	EnsureLists(map[string]HeadInfo(nil))
}

// A struct passed by value cannot be changed, so its lists would go out null.
// EnsureLists says so rather than doing nothing.
func TestEnsureListsRefusesAStructByValue(t *testing.T) {
	for _, v := range []any{Project{}, Page[Team]{}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("EnsureLists(%T by value) returned; want a panic naming the fix", v)
				}
			}()
			EnsureLists(v)
		}()
	}
}
