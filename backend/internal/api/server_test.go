package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// An optional body is read whatever Content-Length says: a chunked request
// has -1, and its body is decoded like a sized one.
func TestReadOptionalJSON(t *testing.T) {
	type in struct {
		MoveTo *int64 `json:"moveTo"`
	}
	for name, c := range map[string]struct {
		body    string
		chunked bool
		moveTo  int64 // 0: left nil
		status  int   // 0: no error
	}{
		"sized":         {body: `{"moveTo": 7}`, moveTo: 7},
		"chunked":       {body: `{"moveTo": 7}`, chunked: true, moveTo: 7},
		"empty":         {},
		"empty chunked": {chunked: true},
		"blank":         {body: " \n"},
		"unknown field": {body: `{"moveTo": 7, "x": 1}`, chunked: true, status: http.StatusBadRequest},
		"malformed":     {body: `{"moveTo":`, chunked: true, status: http.StatusBadRequest},
		"too big":       {body: `{"moveTo": 7}` + strings.Repeat(" ", maxJSONBody), chunked: true, status: http.StatusRequestEntityTooLarge},
	} {
		r := httptest.NewRequest("DELETE", "/x", strings.NewReader(c.body))
		if c.chunked {
			r.ContentLength = -1
		}
		var v in
		err := readOptionalJSON(r, &v)
		var he *httpError
		switch {
		case c.status != 0 && (!errors.As(err, &he) || he.status != c.status):
			t.Errorf("%s: %v, want %d", name, err, c.status)
		case c.status == 0 && err != nil:
			t.Errorf("%s: %v", name, err)
		case c.status == 0 && c.moveTo == 0 && v.MoveTo != nil:
			t.Errorf("%s: moveTo = %d, want it left nil", name, *v.MoveTo)
		case c.moveTo != 0 && (v.MoveTo == nil || *v.MoveTo != c.moveTo):
			t.Errorf("%s: moveTo = %v, want %d", name, v.MoveTo, c.moveTo)
		}
	}
}
