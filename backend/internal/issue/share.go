package issue

// ShareKind is who a filter is shared with, as filter_shares.kind stores it.
type ShareKind string

// Share kinds.
const (
	ShareEveryone ShareKind = "everyone"
	ShareGroup    ShareKind = "group"
	ShareProject  ShareKind = "project"
	ShareUser     ShareKind = "user"
)
