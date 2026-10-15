package api

// Updates. Global administrators learn of the releases published on GitHub
// and, on a Linux installation with the update helper (deploy/linux:
// solstice-update.path, solstice-update.service, update.sh), ask for one to
// be installed. The web process runs unprivileged: all it can do is drop a
// request file naming a version. The helper, running as root, validates it,
// downloads that release from GitHub, verifies it, backs the database up,
// swaps the binary, restarts the service and rolls back if the new version
// is not healthy; it reports in a status file this page reads.
//
// The release check is the only request Solstice makes on its own: at most
// every 12 hours, and never once an administrator turned checks off.

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"solstice/internal/audit"
	"solstice/internal/auth"
	"solstice/internal/settings"
)

const (
	releaseRepo     = "mflr0/Solstice"
	githubAPI       = "https://api.github.com"
	releasesPage    = "https://github.com/" + releaseRepo + "/releases"
	releaseDownload = releasesPage + "/download"

	checkInterval  = 12 * time.Hour   // automatic checks, at most this often
	manualCheckGap = 30 * time.Second // « Vérifier maintenant » twice in a row asks once
	checkTimeout   = 10 * time.Second
	// A request or a run older than this no longer blocks a new one: the
	// helper died with it (a reboot…).
	attemptStale = 30 * time.Minute

	// The helper's directory: the installer creates it (root:solstice, 1770)
	// with the marker file "helper"; the web process writes "request", the
	// helper writes "status".
	helperDir = "/var/lib/solstice/update"

	settingUpdateChecks  = "updates.checks"  // "off" stops the checks
	settingUpdateRelease = "updates.release" // the last check, JSON
	settingUpdateResult  = "updates.result"  // the last helper result written to the audit log
)

// Audit log actions.
const (
	auditUpdateCheck    = "update.check"
	auditUpdateSettings = "update.settings"
	auditUpdateRequest  = "update.request"
	auditUpdateDenied   = "update.denied"
	auditUpdateResult   = "update.result"
)

// Installation kinds.
const (
	installSystemd = "systemd"
	installDocker  = "docker"
	installManual  = "manual"
)

// Attempt states, as the helper writes them; pending is the request not
// taken up yet.
const (
	attemptPending    = "pending"
	attemptRunning    = "running"
	attemptOK         = "ok"
	attemptRolledBack = "rolled_back"
	attemptFailed     = "failed"
)

// Release check failures, translated by the interface.
const (
	checkUnreachable = "unreachable" // no answer
	checkRefused     = "refused"     // an HTTP error (rate limit…)
	checkInvalid     = "invalid"     // an answer that is not a release
)

// releaseTagPattern is semantic versioning without build metadata: what a
// release tag may hold after its "v". update.sh applies the same pattern.
var releaseTagPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)` +
	`(-(0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*)(\.(0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*))*)?$`)

const maxVersionLen = 64

type semver struct {
	core [3]uint64
	pre  []string
}

// parseVersion reads "1.2.3", "v1.2.3" or "1.2.3-rc.1".
func parseVersion(s string) (semver, bool) {
	s = strings.TrimPrefix(s, "v")
	if len(s) > maxVersionLen || !releaseTagPattern.MatchString(s) {
		return semver{}, false
	}
	core, pre, _ := strings.Cut(s, "-")
	var v semver
	for i, part := range strings.Split(core, ".") {
		n, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return semver{}, false
		}
		v.core[i] = n
	}
	if pre != "" {
		v.pre = strings.Split(pre, ".")
	}
	return v, true
}

// compare orders versions by semantic versioning precedence.
func (a semver) compare(b semver) int {
	for i := range a.core {
		if c := cmp.Compare(a.core[i], b.core[i]); c != 0 {
			return c
		}
	}
	switch {
	case len(a.pre) == 0 && len(b.pre) == 0:
		return 0
	case len(a.pre) == 0: // a release comes after its pre-releases
		return 1
	case len(b.pre) == 0:
		return -1
	}
	for i := 0; i < len(a.pre) && i < len(b.pre); i++ {
		x, y := a.pre[i], b.pre[i]
		xn, xerr := strconv.ParseUint(x, 10, 64)
		yn, yerr := strconv.ParseUint(y, 10, 64)
		switch {
		case xerr == nil && yerr == nil:
			if c := cmp.Compare(xn, yn); c != 0 {
				return c
			}
		case xerr == nil: // numeric identifiers come first
			return -1
		case yerr == nil:
			return 1
		default:
			if c := strings.Compare(x, y); c != 0 {
				return c
			}
		}
	}
	return cmp.Compare(len(a.pre), len(b.pre))
}

// newerVersion tells whether latest is newer than current. A development
// build ("dev"), or anything that is not a version, never has a newer one.
func newerVersion(current, latest string) bool {
	c, ok := parseVersion(current)
	if !ok {
		return false
	}
	l, ok := parseVersion(latest)
	return ok && l.compare(c) > 0
}

// updater holds the update check's state. The zero value is ready; tests
// set the fields marked so.
type updater struct {
	version string // the running version (main.version)

	mu         sync.Mutex // one release check at a time
	lastManual time.Time
	resultMu   sync.Mutex // the result of an attempt is audited once

	// Fixed in production; tests only.
	apiBase   string // GitHub's API
	dir       string // the helper's directory
	dockerEnv string // the file Docker puts in containers
	goos      string
	goarch    string
	getenv    func(string) string
	now       func() time.Time
	client    *http.Client
}

// SetVersion tells the API which version is running, for the update check.
func (s *Server) SetVersion(v string) { s.upd.version = v }

func (u *updater) current() string {
	if u.version == "" {
		return "dev"
	}
	return u.version
}

func (u *updater) clock() time.Time {
	if u.now != nil {
		return u.now().UTC()
	}
	return time.Now().UTC()
}

func (u *updater) api() string        { return cmp.Or(u.apiBase, githubAPI) }
func (u *updater) helper() string     { return cmp.Or(u.dir, helperDir) }
func (u *updater) os() string         { return cmp.Or(u.goos, runtime.GOOS) }
func (u *updater) arch() string       { return cmp.Or(u.goarch, runtime.GOARCH) }
func (u *updater) dockerFile() string { return cmp.Or(u.dockerEnv, "/.dockerenv") }

func (u *updater) env(key string) string {
	if u.getenv != nil {
		return u.getenv(key)
	}
	return os.Getenv(key)
}

func (u *updater) httpClient() *http.Client {
	if u.client != nil {
		return u.client
	}
	return &http.Client{Timeout: checkTimeout}
}

// install tells how this instance was installed: "systemd" when the update
// helper is there (its installer leaves a marker), "docker" in a container,
// "manual" otherwise. Only the first can update itself.
func (u *updater) install() string {
	if u.os() == "linux" && isFile(filepath.Join(u.helper(), "helper")) {
		return installSystemd
	}
	if u.env("SOLSTICE_INSTALL") == "docker" || isFile(u.dockerFile()) {
		return installDocker
	}
	return installManual
}

func isFile(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Mode().IsRegular()
}

type releaseAsset struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	URL  string `json:"url"`
}

// releaseCheck is the outcome of the last check, kept in the settings.
type releaseCheck struct {
	CheckedAt   time.Time      `json:"checkedAt"`
	Error       string         `json:"error,omitempty"`   // see checkUnreachable…
	Version     string         `json:"version,omitempty"` // "" when nothing is published
	PublishedAt *time.Time     `json:"publishedAt,omitempty"`
	Notes       string         `json:"notes,omitempty"` // Markdown, as written on GitHub
	URL         string         `json:"url,omitempty"`
	Assets      []releaseAsset `json:"assets,omitempty"`
}

const maxNotes = 64 << 10

// assetPattern names the files a release may carry.
var assetPattern = regexp.MustCompile(`^(solstice-[0-9A-Za-z.-]+-[a-z0-9]+-[a-z0-9]+\.(tar\.gz|zip)|SHA256SUMS)$`)

// parseRelease reads GitHub's answer for a release. Only what is checked
// is kept: a version tag, links to github.com, notes of a sane length.
func parseRelease(r io.Reader) (releaseCheck, error) {
	var in struct {
		TagName     string    `json:"tag_name"`
		Body        string    `json:"body"`
		HTMLURL     string    `json:"html_url"`
		Draft       bool      `json:"draft"`
		Prerelease  bool      `json:"prerelease"`
		PublishedAt time.Time `json:"published_at"`
		Assets      []struct {
			Name string `json:"name"`
			Size int64  `json:"size"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(r).Decode(&in); err != nil {
		return releaseCheck{}, err
	}
	if in.Draft || !strings.HasPrefix(in.TagName, "v") {
		return releaseCheck{}, fmt.Errorf("not a release tag: %q", in.TagName)
	}
	version := strings.TrimPrefix(in.TagName, "v")
	if _, ok := parseVersion(version); !ok {
		return releaseCheck{}, fmt.Errorf("not a release tag: %q", in.TagName)
	}
	out := releaseCheck{Version: version, Notes: in.Body, URL: releasesPage + "/tag/" + in.TagName}
	if strings.HasPrefix(in.HTMLURL, releasesPage+"/") {
		out.URL = in.HTMLURL
	}
	if !in.PublishedAt.IsZero() {
		at := in.PublishedAt.UTC()
		out.PublishedAt = &at
	}
	if len(out.Notes) > maxNotes {
		out.Notes = strings.ToValidUTF8(out.Notes[:maxNotes], "") + "\n…"
	}
	for _, a := range in.Assets {
		if assetPattern.MatchString(a.Name) && strings.HasPrefix(a.URL, releaseDownload+"/"+in.TagName+"/") {
			out.Assets = append(out.Assets, releaseAsset{Name: a.Name, Size: a.Size, URL: a.URL})
		}
	}
	return out, nil
}

// fetchLatest asks GitHub for the latest release. Failures are part of the
// result: they are shown, and count as a check.
func (u *updater) fetchLatest(ctx context.Context) (releaseCheck, error) {
	out := releaseCheck{CheckedAt: u.clock()}
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.api()+"/repos/"+releaseRepo+"/releases/latest", nil)
	if err != nil {
		out.Error = checkInvalid
		return out, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	// GitHub requires one; it carries nothing about this instance.
	req.Header.Set("User-Agent", "Solstice")
	res, err := u.httpClient().Do(req)
	if err != nil {
		out.Error = checkUnreachable
		return out, err
	}
	defer res.Body.Close()
	switch {
	case res.StatusCode == http.StatusNotFound:
		return out, nil // nothing published yet
	case res.StatusCode != http.StatusOK:
		out.Error = checkRefused
		return out, fmt.Errorf("GitHub answered %s", res.Status)
	}
	rel, err := parseRelease(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		out.Error = checkInvalid
		return out, err
	}
	rel.CheckedAt = out.CheckedAt
	return rel, nil
}

func (s *Server) updateChecksEnabled(ctx context.Context) (bool, error) {
	v, _, err := settings.New(s.db).Get(ctx, settingUpdateChecks)
	return v != "off", err
}

// latestRelease returns the last check, checking again first when it is
// due: never checked or 12 hours old, with checks on; or asked for (force),
// unless it was done a moment ago.
func (s *Server) latestRelease(ctx context.Context, force bool) (releaseCheck, error) {
	enabled, err := s.updateChecksEnabled(ctx)
	if err != nil {
		return releaseCheck{}, err
	}
	store := settings.New(s.db)
	load := func() (releaseCheck, bool, error) {
		var last releaseCheck
		ok, err := store.GetJSON(ctx, settingUpdateRelease, &last)
		if err != nil {
			// A value this version cannot read: as if never checked.
			var se *json.SyntaxError
			var te *json.UnmarshalTypeError
			if errors.As(err, &se) || errors.As(err, &te) {
				return releaseCheck{}, false, nil
			}
		}
		return last, ok, err
	}
	due := func(last releaseCheck, ok bool) bool {
		now := s.upd.clock()
		if force {
			return now.Sub(s.upd.lastManual) >= manualCheckGap
		}
		return enabled && (!ok || now.Sub(last.CheckedAt) >= checkInterval || last.CheckedAt.After(now))
	}
	last, ok, err := load()
	if err != nil || !due(last, ok) {
		return last, err
	}
	s.upd.mu.Lock()
	defer s.upd.mu.Unlock()
	// Another request may have checked while this one waited.
	if last, ok, err = load(); err != nil || !due(last, ok) {
		return last, err
	}
	if force {
		s.upd.lastManual = s.upd.clock()
	}
	fresh, err := s.upd.fetchLatest(ctx)
	if err != nil {
		s.log.Warn("update check failed", "err", err)
	}
	if fresh.Error != "" && ok {
		// GitHub out of reach: what was known stays known.
		checkedAt, failure := fresh.CheckedAt, fresh.Error
		fresh = last
		fresh.CheckedAt, fresh.Error = checkedAt, failure
	}
	if err := store.SetJSON(ctx, settingUpdateRelease, fresh); err != nil {
		return fresh, err
	}
	return fresh, nil
}

// updateAttempt is the last update asked for, as the helper reports it.
type updateAttempt struct {
	At     time.Time `json:"at"`
	From   string    `json:"from"`
	To     string    `json:"to"`
	Status string    `json:"status"`
	Log    string    `json:"log"`
}

// updateRequest is the file the helper acts on. It reads the version and
// nothing else; the rest is for whoever looks at the directory.
type updateRequest struct {
	Version     string    `json:"version"`
	RequestedBy string    `json:"requestedBy"`
	At          time.Time `json:"at"`
}

// readSmall reads a regular file of at most limit bytes.
func readSmall(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("%s: not a regular file", path)
	}
	return io.ReadAll(io.LimitReader(f, limit))
}

// lastAttempt reads the helper's status, or the request it has not taken up
// yet. nil when there was never any.
func (u *updater) lastAttempt() *updateAttempt {
	var status *updateAttempt
	if raw, err := readSmall(filepath.Join(u.helper(), "status"), 1<<20); err == nil {
		var a updateAttempt
		if json.Unmarshal(raw, &a) == nil && validAttempt(&a) {
			status = &a
		}
	}
	if status != nil && status.Status == attemptRunning {
		return status
	}
	if raw, err := readSmall(filepath.Join(u.helper(), "request"), 4<<10); err == nil {
		var req updateRequest
		if json.Unmarshal(raw, &req) == nil && releaseTagPattern.MatchString(req.Version) {
			return &updateAttempt{At: req.At.UTC(), From: u.current(), To: req.Version, Status: attemptPending}
		}
	}
	return status
}

var versionLike = regexp.MustCompile(`^[0-9A-Za-z.+-]{1,64}$`)

func validAttempt(a *updateAttempt) bool {
	switch a.Status {
	case attemptRunning, attemptOK, attemptRolledBack, attemptFailed:
	default:
		return false
	}
	if !versionLike.MatchString(a.From) {
		a.From = "?"
	}
	if !versionLike.MatchString(a.To) {
		a.To = "?"
	}
	if len(a.Log) > 64<<10 {
		a.Log = strings.ToValidUTF8(a.Log[len(a.Log)-64<<10:], "")
	}
	a.At = a.At.UTC()
	return true
}

// busy tells whether an update is waiting or running.
func (u *updater) busy(a *updateAttempt) bool {
	return a != nil && (a.Status == attemptPending || a.Status == attemptRunning) &&
		u.clock().Sub(a.At) < attemptStale
}

// writeRequest drops the request for the helper. The file appears whole
// (written aside, then renamed): the helper starts as soon as it exists.
func (u *updater) writeRequest(req updateRequest) error {
	raw, err := json.Marshal(req)
	if err != nil {
		return err
	}
	dir := u.helper()
	f, err := os.CreateTemp(dir, ".request-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(append(raw, '\n')); err != nil {
		f.Close()
		return err
	}
	if err := f.Chmod(0o640); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), filepath.Join(dir, "request"))
}

// archiveName is a release archive's name, as scripts/release.sh writes it.
func archiveName(version, goos, goarch string) string {
	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}
	return "solstice-" + version + "-" + goos + "-" + goarch + ext
}

// instructions are the commands that install a version where Solstice cannot
// do it itself, and the archive for this machine.
func (u *updater) instructions(install, version string) (commands []string, download string) {
	switch install {
	case installDocker:
		// The image is built from the sources (deploy/docker/compose.yaml).
		return []string{
			"git fetch --tags",
			"git checkout v" + version,
			"SOLSTICE_VERSION=" + version + " ./deploy/docker/install.sh",
		}, ""
	case installManual:
		goos, goarch := u.os(), u.arch()
		name := archiveName(version, goos, goarch)
		download = releaseDownload + "/v" + version + "/" + name
		switch goos {
		case "windows":
			return nil, download
		case "linux":
			// The installer keeps the configuration and data, and sets up the
			// helper: the next updates are done from this page.
			return []string{
				"curl -fLO " + download,
				"tar -xzf " + name,
				"cd " + strings.TrimSuffix(name, ".tar.gz"),
				"sudo ./install.sh",
			}, download
		default:
			return []string{"curl -fLO " + download, "tar -xzf " + name}, download
		}
	}
	return nil, ""
}

type updateStateOut struct {
	Current       string         `json:"current"`
	Latest        *string        `json:"latest"`
	Newer         bool           `json:"newer"`
	Notes         string         `json:"notes"`
	PublishedAt   *time.Time     `json:"publishedAt"`
	ReleaseURL    string         `json:"releaseUrl"`
	CheckedAt     *time.Time     `json:"checkedAt"`
	CheckError    *string        `json:"checkError"` // unreachable, refused, invalid
	ChecksEnabled bool           `json:"checksEnabled"`
	Install       string         `json:"install"` // systemd, docker, manual
	CanApply      bool           `json:"canApply"`
	Commands      []string       `json:"commands"` // docker and manual: what to run
	Download      string         `json:"download"` // manual: the archive for this machine
	LastAttempt   *updateAttempt `json:"lastAttempt"`
}

func (s *Server) updateState(ctx context.Context, force bool) (*updateStateOut, error) {
	rel, err := s.latestRelease(ctx, force)
	if err != nil {
		return nil, err
	}
	enabled, err := s.updateChecksEnabled(ctx)
	if err != nil {
		return nil, err
	}
	u := &s.upd
	out := &updateStateOut{
		Current:       u.current(),
		Notes:         rel.Notes,
		PublishedAt:   rel.PublishedAt,
		ReleaseURL:    cmp.Or(rel.URL, releasesPage),
		ChecksEnabled: enabled,
		Install:       u.install(),
		Commands:      []string{},
		LastAttempt:   u.lastAttempt(),
	}
	if !rel.CheckedAt.IsZero() {
		out.CheckedAt = &rel.CheckedAt
	}
	if rel.Error != "" {
		out.CheckError = &rel.Error
	}
	if rel.Version != "" {
		out.Latest = &rel.Version
		out.Newer = newerVersion(out.Current, rel.Version)
	}
	if out.Newer {
		out.CanApply = out.Install == installSystemd && !u.busy(out.LastAttempt)
		if cmds, download := u.instructions(out.Install, rel.Version); cmds != nil || download != "" {
			out.Commands, out.Download = append(out.Commands, cmds...), download
		}
	}
	s.auditUpdateResult(ctx, out.LastAttempt)
	return out, nil
}

// auditUpdateResult writes the outcome of an update to the audit log, once.
// The helper cannot: it is the process that ran it, not this one, that
// knows the database. Whichever version runs afterwards records it when an
// administrator next loads the state (the badge does, on any page).
func (s *Server) auditUpdateResult(ctx context.Context, a *updateAttempt) {
	if a == nil || (a.Status != attemptOK && a.Status != attemptRolledBack && a.Status != attemptFailed) {
		return
	}
	key := a.At.Format(time.RFC3339) + " " + a.To + " " + a.Status
	s.upd.resultMu.Lock()
	defer s.upd.resultMu.Unlock()
	store := settings.New(s.db)
	if prev, _, err := store.Get(ctx, settingUpdateResult); err != nil || prev == key {
		return
	}
	if err := store.Set(ctx, settingUpdateResult, key); err != nil {
		s.log.Error("update result not recorded", "err", err)
		return
	}
	if err := audit.Record(ctx, s.db, audit.Entry{
		Action:  auditUpdateResult,
		Target:  audit.Target{Type: "version", ID: a.To, Label: "Solstice " + a.To},
		Details: map[string]any{"from": a.From, "to": a.To, "status": a.Status},
	}); err != nil {
		s.log.Error("audit log write failed", "action", auditUpdateResult, "err", err)
	}
}

func (s *Server) updateRoutes(route func(string, handler, bool)) {
	route("GET /api/admin/updates", s.getUpdates, true)
	route("POST /api/admin/updates/check", s.checkUpdates, true)
	route("PUT /api/admin/updates/settings", s.updateSettings, true)
	route("POST /api/admin/updates/apply", s.applyRelease, true)
}

func (s *Server) getUpdates(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	out, err := s.updateState(r.Context(), false)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}

func (s *Server) checkUpdates(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	out, err := s.updateState(r.Context(), true)
	if err != nil {
		return err
	}
	details := map[string]any{"latest": out.Latest}
	if out.CheckError != nil {
		details["error"] = *out.CheckError
	}
	s.auditLog(r, auditUpdateCheck, audit.Target{Type: "settings", ID: "updates"}, details)
	return writeJSON(w, http.StatusOK, out)
}

func (s *Server) updateSettings(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	var in struct {
		ChecksEnabled *bool `json:"checksEnabled"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	if in.ChecksEnabled == nil {
		return failf(http.StatusBadRequest, "Invalid request: %s", "checksEnabled")
	}
	value := "on"
	if !*in.ChecksEnabled {
		value = "off"
	}
	if err := settings.New(s.db).Set(r.Context(), settingUpdateChecks, value); err != nil {
		return err
	}
	s.auditLog(r, auditUpdateSettings, audit.Target{Type: "settings", ID: "updates"},
		map[string]any{"checksEnabled": *in.ChecksEnabled})
	out, err := s.updateState(r.Context(), false)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}

// applyRelease asks the helper to install the latest release. The
// administrator types their password again: a session left open must not
// be enough to replace the server's program.
func (s *Server) applyRelease(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	var in struct {
		Version  string `json:"version"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	me := currentUser(r)
	who, err := s.checkPassword(r, me.Username, in.Password)
	if errors.Is(err, auth.ErrNoSession) || (err == nil && who.ID != me.ID) {
		s.auditLog(r, auditUpdateDenied, audit.Target{Type: "version", ID: in.Version}, map[string]any{"reason": "password"})
		return fail(http.StatusForbidden, "Incorrect password.")
	}
	if err != nil {
		return err
	}

	u := &s.upd
	if u.install() != installSystemd {
		return fail(http.StatusConflict, "This installation cannot be updated from the interface: follow the instructions on the page.")
	}
	rel, err := s.latestRelease(r.Context(), false)
	if err != nil {
		return err
	}
	if rel.Version == "" || in.Version != rel.Version {
		return fail(http.StatusConflict, "This is not the latest release. Check for updates again.")
	}
	if !newerVersion(u.current(), rel.Version) {
		return fail(http.StatusConflict, "Solstice is already up to date.")
	}
	if u.busy(u.lastAttempt()) {
		return fail(http.StatusConflict, "An update is already in progress.")
	}
	if err := u.writeRequest(updateRequest{Version: rel.Version, RequestedBy: me.Username, At: u.clock()}); err != nil {
		return err
	}
	s.auditLog(r, auditUpdateRequest, audit.Target{Type: "version", ID: rel.Version, Label: "Solstice " + rel.Version},
		map[string]any{"from": u.current(), "to": rel.Version})
	out, err := s.updateState(r.Context(), false)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusAccepted, out)
}
