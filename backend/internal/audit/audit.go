// Package audit keeps the audit log: who signed in, and who changed
// accounts, access rights and configuration, from where and when.
//
// Any feature records its events with Record; administrators read them
// through the API. Entries are kept for a retention period (the setting
// RetentionSetting, DefaultRetentionDays by default), then purged.
package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"solstice/internal/settings"
)

// Actions recorded by Solstice itself. Other features use the same dotted
// form: "<what>.<verb>" (e.g. "settings.mail", "setup.complete").
const (
	LoginSuccess   = "login.success"
	LoginFailure   = "login.failure"
	PasswordChange = "password.change"
	TokenCreate    = "token.create"
	TokenRevoke    = "token.revoke"
	UserCreate     = "user.create"
	UserUpdate     = "user.update"
	UserDeactivate = "user.deactivate"
	UserActivate   = "user.activate"
	AdminGrant     = "admin.grant"
	AdminRevoke    = "admin.revoke"
	ProjectPerms   = "project.permissions"
	WorkflowCreate = "workflow.create"
	WorkflowUpdate = "workflow.update"
	WorkflowDelete = "workflow.delete"
	StatusCreate   = "status.create"
	StatusUpdate   = "status.update"
	StatusDelete   = "status.delete"
	BoardCreate    = "board.create"
	BoardUpdate    = "board.update"
	BoardDelete    = "board.delete"
	FilterShare    = "filter.share"
	AuditSettings  = "settings.audit"

	// Administration from the interface.
	SettingsGeneral  = "settings.general"
	PasswordReset    = "password.reset"
	GroupCreate      = "group.create"
	GroupUpdate      = "group.update"
	GroupDelete      = "group.delete"
	GroupMembers     = "group.members"
	RoleCreate       = "role.create"
	RoleUpdate       = "role.update"
	RoleDelete       = "role.delete"
	ProjectRoles     = "project.roles"
	ProjectCreate    = "project.create"
	ProjectUpdate    = "project.update"
	ProjectArchive   = "project.archive"
	ProjectUnarchive = "project.unarchive"
	ProjectDelete    = "project.delete"
	CategoryCreate   = "category.create"
	CategoryUpdate   = "category.update"
	CategoryDelete   = "category.delete"
	ComponentCreate  = "component.create"
	ComponentUpdate  = "component.update"
	ComponentDelete  = "component.delete"
	VersionCreate    = "version.create"
	VersionUpdate    = "version.update"
	VersionOrder     = "version.order"
	VersionRelease   = "version.release"
	VersionMerge     = "version.merge"
	VersionDelete    = "version.delete"
	FieldCreate      = "field.create"
	FieldUpdate      = "field.update"
	FieldDelete      = "field.delete"
	FieldOptions     = "field.options"
	StatusOrder      = "status.order"
	ImportStart      = "import.start"
	ImportDone       = "import.done"
	ImportFailed     = "import.failed"
	SettingsMail     = "settings.mail"
	WebhookCreate    = "webhook.create"
	WebhookUpdate    = "webhook.update"
	WebhookDelete    = "webhook.delete"
	// Catalogue items: "<kind>.create", "<kind>.update", "<kind>.delete",
	// "<kind>.order" with kind issuetype, priority, resolution, linktype.
)

// Target is what an action was done to: a user, a project, a token…
type Target struct {
	Type  string // user, project, token, workflow, status, board, filter, settings…
	ID    string
	Label string // a readable name, kept as it was at the time
}

// Entry is one line of the audit log.
type Entry struct {
	UserID   int64  // who acted; 0 when unknown (a failed sign-in, the system)
	Username string // their user name, or the name typed for a failed sign-in
	IP       string
	Action   string
	Target   Target
	// Details are shown as they are to administrators. Never put secrets in
	// them; keys that look like one (password, secret, token…) are masked.
	Details map[string]any
}

// RetentionSetting is the name of the setting holding how many days entries
// are kept; 0 keeps them forever.
const RetentionSetting = "audit.retention_days"

// DefaultRetentionDays applies while the setting was never changed.
const DefaultRetentionDays = 365

// purgeEvery spaces the purges that Record runs as it goes.
const purgeEvery = time.Hour

var lastPurge atomic.Int64

// Record writes an entry. It does not depend on the caller's context being
// alive: an action that succeeded is logged even if the client went away.
// Every hour or so, it also purges the entries past the retention period.
func Record(ctx context.Context, db *sql.DB, e Entry) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	var details any
	if len(e.Details) > 0 {
		raw, err := json.Marshal(mask(e.Details))
		if err != nil {
			return err
		}
		details = string(raw)
	}
	now := time.Now().UTC()
	if _, err := db.ExecContext(ctx, `INSERT INTO audit_log (created_at, user_id, username, ip, action, target_type,
		target_id, target_label, details) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		now, nullID(e.UserID), nullText(e.Username, 255), nullText(e.IP, 64), cut(e.Action, 100),
		nullText(e.Target.Type, 50), nullText(e.Target.ID, 100), nullText(e.Target.Label, 255), details); err != nil {
		return err
	}
	if last := lastPurge.Load(); now.Sub(time.Unix(0, last)) > purgeEvery && lastPurge.CompareAndSwap(last, now.UnixNano()) {
		if _, err := Purge(ctx, db); err != nil {
			return err
		}
	}
	return nil
}

// Retention returns how many days entries are kept (0: forever).
func Retention(ctx context.Context, db *sql.DB) (int, error) {
	raw, ok, err := settings.New(db).Get(ctx, RetentionSetting)
	if err != nil || !ok {
		return DefaultRetentionDays, err
	}
	days, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || days < 0 {
		return DefaultRetentionDays, nil
	}
	return days, nil
}

// SetRetention changes how many days entries are kept (0: forever).
func SetRetention(ctx context.Context, db *sql.DB, days int) error {
	return settings.New(db).Set(ctx, RetentionSetting, strconv.Itoa(days))
}

// Purge deletes the entries older than the retention period and returns how
// many went.
func Purge(ctx context.Context, db *sql.DB) (int64, error) {
	days, err := Retention(ctx, db)
	if err != nil || days == 0 {
		return 0, err
	}
	res, err := db.ExecContext(ctx, `DELETE FROM audit_log WHERE created_at < ?`,
		time.Now().UTC().AddDate(0, 0, -days))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// secretKeys are parts of detail keys whose values are never logged.
var secretKeys = []string{"password", "passwd", "secret", "token", "apikey", "api_key", "credential"}

// mask copies details, hiding the values of keys that name a secret. A
// boolean holds no secret: "passwordChanged": true says that a password
// changed without showing it.
func mask(details map[string]any) map[string]any {
	out := make(map[string]any, len(details))
	for k, v := range details {
		lower := strings.ToLower(k)
		secret := false
		for _, s := range secretKeys {
			if strings.Contains(lower, s) {
				secret = true
			}
		}
		_, flag := v.(bool)
		switch {
		case secret && !flag:
			out[k] = "***"
		case isMap(v):
			out[k] = mask(v.(map[string]any))
		default:
			out[k] = v
		}
	}
	return out
}

func isMap(v any) bool {
	_, ok := v.(map[string]any)
	return ok
}

func nullID(id int64) any {
	if id == 0 {
		return nil
	}
	return id
}

func nullText(s string, n int) any {
	if s == "" {
		return nil
	}
	return cut(s, n)
}

// cut shortens s to at most n characters.
func cut(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}
