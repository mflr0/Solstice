package api

// Instance settings administrators change in the interface: the instance's
// name, public address, time zone, default language, announcement banner,
// sign-in lifetime and password length. They live in the settings table
// and apply at once. The public address and the time zone may also come
// from the configuration file or the environment (SOLSTICE_HTTP_BASEURL,
// SOLSTICE_TZ): those are initial values, and the database wins once an
// administrator saved one.

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"solstice/internal/audit"
	"solstice/internal/i18n"
	"solstice/internal/sqlq"
)

// ConfigInfo tells how the server was started, for the system page.
type ConfigInfo struct {
	Version string
	Path    string // the configuration file
	Found   bool   // whether it exists
	Addr    string // listen address
}

// adminState is what the administration keeps in memory.
type adminState struct {
	started time.Time

	mu    sync.Mutex
	info  ConfigInfo
	gen   *generalSettings
	genAt time.Time

	impMu sync.Mutex
	imp   *adminImport // the running or last import

	sizeMu   sync.Mutex
	dataSize int64
	sizeErr  string
	sizeAt   time.Time
}

// SetConfigInfo records how the server was started.
func (s *Server) SetConfigInfo(info ConfigInfo) {
	s.adm.mu.Lock()
	s.adm.info, s.adm.gen = info, nil
	s.adm.mu.Unlock()
}

func (s *Server) configInfo() ConfigInfo {
	s.adm.mu.Lock()
	defer s.adm.mu.Unlock()
	return s.adm.info
}

const (
	sourceDefault  = "default"
	sourceFile     = "file"
	sourceDatabase = "database"
)

type bannerSetting struct {
	Enabled bool   `json:"enabled"`
	Text    string `json:"text"`
	Level   string `json:"level"` // info, warning
}

// generalSettings are the settings in effect. A loaded value is never
// changed: a change loads a new one.
type generalSettings struct {
	InstanceName      string            `json:"instanceName"`
	BaseURL           string            `json:"baseURL"`
	TimeZone          string            `json:"timeZone"` // "": the server's local zone
	DefaultLanguage   string            `json:"defaultLanguage"`
	Banner            bannerSetting     `json:"banner"`
	SessionDays       int               `json:"sessionDays"`
	PasswordMinLength int               `json:"passwordMinLength"`
	Sources           map[string]string `json:"sources"`
	Languages         []string          `json:"languages"`

	loc *time.Location
}

// generalFields maps each setting to its row in the settings table.
var generalFields = []struct{ field, name string }{
	{"instanceName", "general.instance_name"},
	{"baseURL", "general.base_url"},
	{"timeZone", "general.time_zone"},
	{"defaultLanguage", "general.default_language"},
	{"banner", "general.banner"},
	{"sessionDays", "general.session_days"},
	{"passwordMinLength", "general.password_min_length"},
}

const (
	defaultInstanceName = "Solstice"
	defaultSessionDays  = 30
	defaultPasswordMin  = 12
	maxInstanceName     = 100
	maxBannerText       = 500
)

// bootSource tells where a value given at start comes from: the
// configuration file (or the environment, which overrides it) when set.
func bootSource(value string) string {
	if value == "" {
		return sourceDefault
	}
	return sourceFile
}

// general returns the settings in effect, from a short-lived cache: they
// are read on most requests, and another process (an import) may change
// them.
func (s *Server) general(ctx context.Context) *generalSettings {
	s.adm.mu.Lock()
	defer s.adm.mu.Unlock()
	if s.adm.gen != nil && time.Since(s.adm.genAt) < 30*time.Second {
		return s.adm.gen
	}
	g, err := s.loadGeneral(context.WithoutCancel(ctx))
	if err != nil {
		s.log.Warn("reading the instance settings failed", "err", err)
		if s.adm.gen != nil {
			return s.adm.gen
		}
		return g
	}
	s.adm.gen, s.adm.genAt = g, time.Now()
	return g
}

// forgetGeneral drops the cached settings after a change.
func (s *Server) forgetGeneral() {
	s.forgetEmailLinks() // the public address is part of what links need
	s.adm.mu.Lock()
	s.adm.gen = nil
	s.adm.mu.Unlock()
}

// loadGeneral reads the settings; on error it returns the defaults (and
// the error).
func (s *Server) loadGeneral(ctx context.Context) (*generalSettings, error) {
	bootTZ := ""
	if s.tz != nil && s.tz != time.Local && s.tz.String() != "Local" {
		bootTZ = s.tz.String()
	}
	g := &generalSettings{
		InstanceName: defaultInstanceName, BaseURL: s.baseURL, TimeZone: bootTZ,
		DefaultLanguage: i18n.Default, Banner: bannerSetting{Level: "info"},
		SessionDays: defaultSessionDays, PasswordMinLength: defaultPasswordMin,
		Languages: i18n.Locales,
		Sources: map[string]string{
			"baseURL": bootSource(s.baseURL), "timeZone": bootSource(bootTZ),
		},
	}
	for _, f := range generalFields {
		if g.Sources[f.field] == "" {
			g.Sources[f.field] = sourceDefault
		}
	}
	// Without a database (some unit tests), the defaults are the settings.
	if s.db == nil {
		return g, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var loadErr error
	err := sqlq.Each(ctx, s.db, `SELECT name, value FROM settings WHERE name LIKE 'general.%'`, func(rows *sql.Rows) error {
		var name, raw string
		if err := rows.Scan(&name, &raw); err != nil {
			return err
		}
		for _, f := range generalFields {
			if f.name != name {
				continue
			}
			var err error
			switch f.field {
			case "instanceName":
				err = json.Unmarshal([]byte(raw), &g.InstanceName)
			case "baseURL":
				err = json.Unmarshal([]byte(raw), &g.BaseURL)
			case "timeZone":
				err = json.Unmarshal([]byte(raw), &g.TimeZone)
			case "defaultLanguage":
				err = json.Unmarshal([]byte(raw), &g.DefaultLanguage)
			case "banner":
				err = json.Unmarshal([]byte(raw), &g.Banner)
			case "sessionDays":
				err = json.Unmarshal([]byte(raw), &g.SessionDays)
			case "passwordMinLength":
				err = json.Unmarshal([]byte(raw), &g.PasswordMinLength)
			}
			if err != nil {
				loadErr = err
				continue
			}
			g.Sources[f.field] = sourceDatabase
		}
		return nil
	})
	if err == nil {
		err = loadErr
	}
	g.loc = time.Local
	if g.TimeZone != "" {
		if loc, lerr := time.LoadLocation(g.TimeZone); lerr == nil {
			g.loc = loc
		}
	}
	if !i18n.Supported(g.DefaultLanguage) {
		g.DefaultLanguage = i18n.Default
	}
	return g, err
}

// base is the public address in effect, "" when unknown.
func (s *Server) base() string { return s.general(context.Background()).BaseURL }

// defaultLanguage is the language of people who chose none.
func (s *Server) defaultLanguage() string { return s.general(context.Background()).DefaultLanguage }

func (s *Server) passwordMin(ctx context.Context) int { return s.general(ctx).PasswordMinLength }

// sessionLifetime is how long a new sign-in lasts.
func (s *Server) sessionLifetime(ctx context.Context) time.Duration {
	return time.Duration(s.general(ctx).SessionDays) * 24 * time.Hour
}

// checkNewPassword applies the password rules to a new password of the
// account username.
func (s *Server) checkNewPassword(ctx context.Context, password, username string) error {
	return passwordRules(password, username, s.passwordMin(ctx))
}

// passwordRules is the one password policy, for every way of setting a
// password: at least minLen characters, and not the username.
func passwordRules(password, username string, minLen int) error {
	if utf8.RuneCountInString(password) < minLen {
		return failf(http.StatusBadRequest, i18n.AccountPasswordTooShort, minLen)
	}
	if strings.EqualFold(strings.TrimSpace(password), username) {
		return fail(http.StatusBadRequest, i18n.AccountPasswordIsUsername)
	}
	return nil
}

// storedPasswordMin reads the minimum length from the settings table, for
// the setup wizard, which runs without a Server.
func storedPasswordMin(ctx context.Context, db *sql.DB) int {
	n := defaultPasswordMin
	var raw string
	if db.QueryRowContext(ctx, `SELECT value FROM settings WHERE name = 'general.password_min_length'`).Scan(&raw) == nil {
		if json.Unmarshal([]byte(raw), &n) != nil {
			n = defaultPasswordMin
		}
	}
	return n
}

// publicSettings is what anyone may know, signed in or not.
func (s *Server) publicSettings(w http.ResponseWriter, r *http.Request) error {
	g := s.general(r.Context())
	var banner any
	if g.Banner.Enabled && strings.TrimSpace(g.Banner.Text) != "" {
		banner = map[string]string{"text": g.Banner.Text, "level": g.Banner.Level}
	}
	return writeJSON(w, http.StatusOK, map[string]any{"instanceName": g.InstanceName,
		"defaultLanguage": g.DefaultLanguage, "banner": banner, "passwordMinLength": g.PasswordMinLength,
		"emailLinks": s.emailLinksReady(r.Context())})
}

func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, s.general(r.Context()))
}

// putSettings saves the settings given; null puts one back to its
// inherited value.
func (s *Server) putSettings(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	var in map[string]json.RawMessage
	if err := readJSON(r, &in); err != nil {
		return err
	}
	known := map[string]string{"sources": "", "languages": ""}
	for _, f := range generalFields {
		known[f.field] = f.name
	}
	for k := range in {
		if _, ok := known[k]; !ok {
			return failf(http.StatusBadRequest, i18n.FieldUnknown, k)
		}
	}
	ctx := r.Context()
	s.forgetGeneral()
	before := s.general(ctx)
	values := map[string]any{} // name → value to store; nil to delete
	for _, f := range generalFields {
		raw, ok := in[f.field]
		if !ok {
			continue
		}
		if string(raw) == "null" {
			values[f.name] = nil
			continue
		}
		v, err := checkSetting(f.field, raw)
		if err != nil {
			return err
		}
		values[f.name] = v
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC()
	for name, v := range values {
		if v == nil {
			_, err = tx.ExecContext(ctx, `DELETE FROM settings WHERE name = ?`, name)
		} else {
			var raw []byte
			if raw, err = json.Marshal(v); err == nil {
				_, err = tx.ExecContext(ctx, `INSERT INTO settings (name, value, updated_at) VALUES (?, ?, ?)
					ON DUPLICATE KEY UPDATE value = VALUES(value), updated_at = VALUES(updated_at)`, name, string(raw), now)
			}
		}
		if err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.forgetGeneral()
	after := s.general(ctx)
	if changes := settingChanges(before, after); len(changes) > 0 {
		s.auditLog(r, audit.SettingsGeneral, audit.Target{Type: "settings", ID: "general"}, changes)
	}
	return writeJSON(w, http.StatusOK, after)
}

// checkSetting validates a new value and returns it as stored.
func checkSetting(field string, raw json.RawMessage) (any, error) {
	invalid := failf(http.StatusBadRequest, i18n.FieldValueInvalid, field)
	switch field {
	case "instanceName":
		var v string
		if json.Unmarshal(raw, &v) != nil {
			return nil, invalid
		}
		v = strings.TrimSpace(v)
		if v == "" || utf8.RuneCountInString(v) > maxInstanceName {
			return nil, failf(http.StatusBadRequest, i18n.SettingsNameRequired, maxInstanceName)
		}
		return v, nil
	case "baseURL":
		var v string
		if json.Unmarshal(raw, &v) != nil {
			return nil, invalid
		}
		if strings.TrimSpace(v) == "" {
			return "", nil
		}
		return normalizeBaseURL(v)
	case "timeZone":
		var v string
		if json.Unmarshal(raw, &v) != nil {
			return nil, invalid
		}
		v = strings.TrimSpace(v)
		if v == "" {
			return "", nil
		}
		if err := checkTZ(v); err != nil {
			return nil, err
		}
		return v, nil
	case "defaultLanguage":
		var v string
		if json.Unmarshal(raw, &v) != nil || !i18n.Supported(v) {
			return nil, failf(http.StatusBadRequest, i18n.AccountLanguageUnsupported, strings.Join(i18n.Locales, ", "))
		}
		return v, nil
	case "banner":
		var v bannerSetting
		if json.Unmarshal(raw, &v) != nil {
			return nil, invalid
		}
		v.Text = strings.TrimSpace(v.Text)
		if v.Level == "" {
			v.Level = "info"
		}
		if v.Level != "info" && v.Level != "warning" {
			return nil, fail(http.StatusBadRequest, i18n.SettingsBannerLevelUnknown)
		}
		if utf8.RuneCountInString(v.Text) > maxBannerText {
			return nil, failf(http.StatusBadRequest, i18n.SettingsBannerTooLong, maxBannerText)
		}
		return v, nil
	case "sessionDays":
		var v int
		if json.Unmarshal(raw, &v) != nil || v < 1 || v > 365 {
			return nil, fail(http.StatusBadRequest, i18n.SettingsSessionLengthInvalid)
		}
		return v, nil
	case "passwordMinLength":
		var v int
		if json.Unmarshal(raw, &v) != nil || v < 8 || v > 128 {
			return nil, fail(http.StatusBadRequest, i18n.SettingsPasswordLengthInvalid)
		}
		return v, nil
	}
	return nil, invalid
}

// settingChanges lists what differs between two states, for the audit log.
func settingChanges(before, after *generalSettings) map[string]any {
	out := map[string]any{}
	pair := func(name string, from, to any) {
		if from != to {
			out[name] = map[string]any{"from": from, "to": to}
		}
	}
	pair("instanceName", before.InstanceName, after.InstanceName)
	pair("baseURL", before.BaseURL, after.BaseURL)
	pair("timeZone", before.TimeZone, after.TimeZone)
	pair("defaultLanguage", before.DefaultLanguage, after.DefaultLanguage)
	pair("banner", before.Banner, after.Banner)
	pair("sessionDays", before.SessionDays, after.SessionDays)
	pair("passwordMinLength", before.PasswordMinLength, after.PasswordMinLength)
	for k, v := range after.Sources {
		if before.Sources[k] != v && out[k] == nil {
			out[k] = map[string]any{"source": map[string]string{"from": before.Sources[k], "to": v}}
		}
	}
	return out
}

// languageKey carries the instance's default language in the request
// context, for locale.
const languageKey ctxKey = 100

// instanceMiddleware runs before every route: it puts the default language
// in the request context, and holds requests back while an import replaces
// the data (see admin_import.go).
func (s *Server) instanceMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(context.WithValue(r.Context(), languageKey, s.defaultLanguage()))
		if s.maintenance(w, r) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

// requestDefaultLanguage is the instance's default language for a request
// that went through instanceMiddleware, or "".
func requestDefaultLanguage(r *http.Request) string {
	l, _ := r.Context().Value(languageKey).(string)
	if slices.Contains(i18n.Locales, l) {
		return l
	}
	return ""
}
