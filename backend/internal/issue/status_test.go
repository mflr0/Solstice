package issue

import "testing"

func TestStatusCategoryOrTodo(t *testing.T) {
	for in, want := range map[StatusCategory]StatusCategory{
		Todo: Todo, Doing: Doing, Done: Done, "": Todo, "Done": Todo, "closed": Todo,
	} {
		if got := in.OrTodo(); got != want {
			t.Errorf("%q.OrTodo() = %q, want %q", in, got, want)
		}
	}
}

func TestStatusCategoryRank(t *testing.T) {
	if !(Todo.Rank() < Doing.Rank() && Doing.Rank() < Done.Rank()) {
		t.Errorf("ranks not in workflow order: %d %d %d", Todo.Rank(), Doing.Rank(), Done.Rank())
	}
	if got := StatusCategory("x").Rank(); got != Todo.Rank() {
		t.Errorf("unknown category ranks %d, want Todo's %d", got, Todo.Rank())
	}
}

func TestJiraStatusCategory(t *testing.T) {
	for _, c := range StatusCategories {
		j := c.Jira()
		if j.Key == "" || j.Colour == "" {
			t.Errorf("%q has no Jira key or colour", c)
		}
		if back := FromJiraStatusCategory(int64(j.ID)); back != c {
			t.Errorf("FromJiraStatusCategory(%d) = %q, want %q", j.ID, back, c)
		}
	}
	for _, id := range []int64{0, 1, 99} {
		if got := FromJiraStatusCategory(id); got != Todo {
			t.Errorf("FromJiraStatusCategory(%d) = %q, want todo", id, got)
		}
	}
}
