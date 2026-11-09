package layout

import (
	"errors"
	"slices"
	"testing"

	"solstice/internal/i18n"
)

func ids(fields []Field) []string {
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		out = append(out, f.ID)
	}
	return out
}

func find(fields []Field, id string) (Field, bool) {
	i := slices.IndexFunc(fields, func(f Field) bool { return f.ID == id })
	if i < 0 {
		return Field{}, false
	}
	return fields[i], true
}

// The override wins over the type's template, which wins over the default.
func TestResolveOrder(t *testing.T) {
	override := []Field{{ID: "labels"}, {ID: "priority"}}
	typ := []Field{{ID: "assignee"}, {ID: "description"}}
	for _, c := range []struct {
		name           string
		override, typ  []Field
		wantFirst      []string
		wantFromSource int
	}{
		{"override", override, typ, []string{"labels", "priority"}, 2},
		{"type", nil, typ, []string{"assignee", "description"}, 2},
		{"default", nil, nil, ids(Default(Create)), len(Default(Create))},
	} {
		got := Resolve(Create, c.override, c.typ, nil)
		if first := ids(got[:c.wantFromSource]); !slices.Equal(first, c.wantFirst) {
			t.Errorf("%s: starts with %v, want %v", c.name, first, c.wantFirst)
		}
		for _, f := range got[c.wantFromSource:] {
			if !f.Hidden {
				t.Errorf("%s: %s, which the layout misses, is shown", c.name, f.ID)
			}
		}
		if len(got) != len(Placeable(Create)) {
			t.Errorf("%s: %d fields, want every placeable one (%d): %v", c.name, len(got), len(Placeable(Create)), ids(got))
		}
	}
}

// A custom field the context offers and the layout doesn't place comes
// after the placed ones, shown and not required: in the side zone of the view.
func TestResolveAppendsOfferedCustom(t *testing.T) {
	typ := []Field{{ID: "priority", Zone: Side}, {ID: CustomID(7), Zone: Main, Required: false}}
	got := Resolve(View, nil, typ, []int64{7, 9})
	if want := []string{"priority", "customfield_7", "customfield_9"}; !slices.Equal(ids(got[:3]), want) {
		t.Fatalf("starts with %v, want %v", ids(got[:3]), want)
	}
	if f := got[2]; f.Hidden || f.Required || f.Zone != Side {
		t.Errorf("appended custom field: %+v", f)
	}
	if f := got[1]; f.Zone != Main {
		t.Errorf("placed custom field moved: %+v", f)
	}
	created := Resolve(Create, nil, nil, []int64{9})
	if f, ok := find(created, "customfield_9"); !ok || f.Hidden || f.Required || f.Zone != "" {
		t.Errorf("create form, appended custom field: %+v, %v", f, ok)
	}
}

// A custom field the layout places and the context doesn't offer is gone.
func TestResolveDropsCustomOutsideContext(t *testing.T) {
	override := []Field{{ID: CustomID(3), Required: true}, {ID: "priority"}, {ID: CustomID(4)}}
	got := Resolve(Create, override, nil, []int64{4})
	if _, ok := find(got, "customfield_3"); ok {
		t.Errorf("customfield_3 kept: %v", ids(got))
	}
	if want := []string{"priority", "customfield_4"}; !slices.Equal(ids(got[:2]), want) {
		t.Errorf("starts with %v, want %v", ids(got[:2]), want)
	}
}

// The defaults place every placeable field once, in its own form's terms.
func TestDefaults(t *testing.T) {
	for _, form := range []Form{Create, View} {
		d := Default(form)
		if err := Check(form, d); err != nil {
			t.Errorf("%s: %v", form, err)
		}
		got, want := ids(d), Placeable(form)
		slices.Sort(got)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("%s: places %v, want %v", form, got, want)
		}
	}
	if f, _ := find(Default(Create), "reporter"); !f.Hidden {
		t.Error("reporter shown on the create form, which never showed it")
	}
}

func TestCheck(t *testing.T) {
	for _, c := range []struct {
		form   Form
		fields []Field
		want   i18n.Key
	}{
		{Create, []Field{{ID: "summary"}}, i18n.LayoutFieldFixed},
		{View, []Field{{ID: "issuetype", Zone: Side}}, i18n.LayoutFieldFixed},
		{Create, []Field{{ID: "project"}}, i18n.LayoutFieldFixed},
		{Create, []Field{{ID: "couleur"}}, i18n.FieldUnknown},
		{Create, []Field{{ID: "created"}}, i18n.FieldUnknown},
		{Create, []Field{{ID: "customfield_x"}}, i18n.FieldUnknown},
		{Create, []Field{{ID: "customfield_07"}}, i18n.FieldUnknown},
		{Create, []Field{{ID: "priority"}, {ID: "priority"}}, i18n.LayoutFieldTwice},
		{Create, []Field{{ID: "customfield_7"}, {ID: "customfield_7"}}, i18n.LayoutFieldTwice},
		{Create, []Field{{ID: "priority", Zone: Main}}, i18n.LayoutZoneOnCreate},
		{View, []Field{{ID: "priority"}}, i18n.LayoutZoneInvalid},
		{View, []Field{{ID: "priority", Zone: "top"}}, i18n.LayoutZoneInvalid},
		{View, []Field{{ID: "priority", Zone: Side, Required: true}}, i18n.LayoutRequiredOnView},
		{Create, []Field{{ID: "priority", Hidden: true, Required: true}}, i18n.LayoutRequiredHidden},
	} {
		var e *Error
		if err := Check(c.form, c.fields); !errors.As(err, &e) || e.Key != c.want {
			t.Errorf("%s %+v: %v, want %s", c.form, c.fields, err, c.want)
		}
	}
	ok := []Field{{ID: "customfield_7", Required: true}, {ID: "priority", Hidden: true}}
	if err := Check(Create, ok); err != nil {
		t.Errorf("%+v: %v", ok, err)
	}
}
