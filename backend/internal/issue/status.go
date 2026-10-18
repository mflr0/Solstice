// Package issue holds the domain types of issues, shared by the API, events,
// webhooks, searches and the Jira import.
package issue

import (
	"slices"

	"solstice/internal/i18n"
)

// StatusCategory is the stage of work a status stands for. Behaviour that
// depends on a status (resolving, board columns, epics being done) reads its
// category, never its name.
type StatusCategory string

// The status categories, as stored in statuses.category.
const (
	Todo  StatusCategory = "todo"
	Doing StatusCategory = "doing"
	Done  StatusCategory = "done"
)

// StatusCategories lists the categories in the order work goes through them.
var StatusCategories = []StatusCategory{Todo, Doing, Done}

// Valid tells whether c is one of StatusCategories.
func (c StatusCategory) Valid() bool { return slices.Contains(StatusCategories, c) }

// OrTodo returns c, or Todo when c is not a category: an issue whose status
// is missing counts as not started.
func (c StatusCategory) OrTodo() StatusCategory {
	if c.Valid() {
		return c
	}
	return Todo
}

// Rank is the position of c in StatusCategories, for sorting.
func (c StatusCategory) Rank() int { return slices.Index(StatusCategories, c.OrTodo()) }

// Name is the category's name, to render in a language.
func (c StatusCategory) Name() i18n.Message {
	switch c.OrTodo() {
	case Doing:
		return i18n.M(i18n.StatusCategoryInProgress)
	case Done:
		return i18n.M(i18n.StatusCategoryDone)
	}
	return i18n.M(i18n.StatusCategoryToDo)
}

// JiraStatusCategory is a status category as Jira identifies it.
type JiraStatusCategory struct {
	ID     int
	Key    string
	Colour string
}

var jiraCategories = map[StatusCategory]JiraStatusCategory{
	Todo:  {2, "new", "blue-gray"},
	Doing: {4, "indeterminate", "yellow"},
	Done:  {3, "done", "green"},
}

// Jira returns Jira's fixed id, key and colour for c.
func (c StatusCategory) Jira() JiraStatusCategory { return jiraCategories[c.OrTodo()] }

// FromJiraStatusCategory maps a Jira status category id to Solstice's. Jira's
// "No category" and unknown ids count as Todo.
func FromJiraStatusCategory(id int64) StatusCategory {
	for c, j := range jiraCategories {
		if int64(j.ID) == id {
			return c
		}
	}
	return Todo
}
