package api

import "reflect"

// changed tells whether an audited value differs between two states.
// reflect.DeepEqual, as comparing slices held in an any panics.
func changed(from, to any) bool {
	return !reflect.DeepEqual(from, to)
}

// diff records under name, in an entry's details, what a value was and became,
// when it changed, and tells whether it did.
func diff(out map[string]any, name string, from, to any) bool {
	if !changed(from, to) {
		return false
	}
	out[name] = map[string]any{"from": from, "to": to}
	return true
}
