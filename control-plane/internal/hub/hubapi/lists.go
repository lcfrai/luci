package hubapi

import "reflect"

// EnsureLists walks v — a pointer to a struct, a slice of them, or a Page —
// and replaces every nil slice and nil map it can reach with an empty one, so
// the body says [] or {} and never null (contract §7.1). The Hub's JSON writer
// calls it on every response; nothing else needs to remember the rule.
// It uses reflection and does not descend into json.RawMessage.
//
// It follows non-nil pointers, struct fields and slice elements. It leaves a
// nil pointer nil (the contract's null for an absent object), a byte slice as
// it is (a json.RawMessage's nil is an erased revision's null content), and
// does not descend into map values (Event.Data's are scalars).
//
// A struct passed by value cannot be changed, so its lists would go out null
// with no sign. EnsureLists panics for one, naming the fix, rather than do
// nothing (task 03).
func EnsureLists(v any) {
	if v == nil {
		return
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Struct {
		panic("hubapi.EnsureLists: a " + rv.Type().String() + " passed by value cannot be changed; pass a pointer to it")
	}
	fill(rv)
}

func fill(v reflect.Value) {
	switch v.Kind() {
	case reflect.Pointer:
		if !v.IsNil() {
			fill(v.Elem())
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() {
				fill(v.Field(i))
			}
		}
	case reflect.Slice:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			return
		}
		if v.IsNil() {
			if v.CanSet() {
				v.Set(reflect.MakeSlice(v.Type(), 0, 0))
			}
			return
		}
		for i := 0; i < v.Len(); i++ {
			fill(v.Index(i))
		}
	case reflect.Map:
		if v.IsNil() && v.CanSet() {
			v.Set(reflect.MakeMap(v.Type()))
		}
	}
}
