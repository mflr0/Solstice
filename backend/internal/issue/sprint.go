package issue

// SprintState is the state of a sprint, as sprints.state stores it.
type SprintState string

// Sprint states.
const (
	SprintFuture SprintState = "future"
	SprintActive SprintState = "active"
	SprintClosed SprintState = "closed"
)

// Version kinds, as issue_versions.kind stores them: the versions an issue
// is fixed in, and those it affects.
const (
	VersionFix     = "fix"
	VersionAffects = "affects"
)
