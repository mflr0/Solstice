package api

// Theme packages on the instance: list, import, replace, turn on or off,
// delete.

import (
	"errors"
	"io"
	"net/http"

	"solstice/internal/audit"
	"solstice/internal/auth"
	"solstice/internal/i18n"
	"solstice/internal/themes"
)

func (s *Server) themeAdminRoutes(route routeFunc) {
	route("GET /api/admin/themes", s.adminThemes, auth.ScopeAdmin)
	route("POST /api/admin/themes", s.importTheme, auth.ScopeAdmin)
	route("PUT /api/admin/themes/{id}", s.replaceTheme, auth.ScopeAdmin)
	route("DELETE /api/admin/themes/{id}", s.deleteTheme, auth.ScopeAdmin)
	route("POST /api/admin/themes/{id}/enable", s.switchTheme(true), auth.ScopeAdmin)
	route("POST /api/admin/themes/{id}/disable", s.switchTheme(false), auth.ScopeAdmin)
}

func themeTarget(e themes.Entry) audit.Target {
	return audit.Target{Type: audit.TargetTheme, ID: e.ID, Label: e.Name["en"]}
}

// themeError gives the registry's refusals their answer.
func themeError(err error) error {
	switch {
	case errors.Is(err, themes.ErrNotFound):
		return errNotFound
	case errors.Is(err, themes.ErrBuiltin):
		return fail(http.StatusConflict, i18n.ThemeBuiltinFixed)
	case errors.Is(err, themes.ErrFallback):
		return fail(http.StatusConflict, i18n.ThemeFallbackRequired)
	}
	return err
}

func themeRefused(findings []themes.Finding) error {
	return &httpError{status: http.StatusBadRequest, msg: i18n.M(i18n.ThemePackageRefused), extra: map[string]any{"findings": findings}}
}

// readThemeZip reads the part "file" of a request.
func readThemeZip(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	r.Body = http.MaxBytesReader(w, r.Body, themes.MaxZipBytes+1<<20)
	missing := fail(http.StatusBadRequest, i18n.ThemeFileMissing)
	tooLarge := failf(http.StatusRequestEntityTooLarge, i18n.ThemeFileTooLarge, themes.MaxPackageBytes>>20)
	unreadable := func(err error) error {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			return tooLarge
		}
		return missing
	}
	reader, err := r.MultipartReader()
	if err != nil {
		return nil, missing
	}
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			return nil, missing
		}
		if err != nil {
			return nil, unreadable(err)
		}
		if part.FormName() != "file" {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(part, themes.MaxZipBytes+1))
		if err != nil {
			return nil, unreadable(err)
		}
		if len(data) > themes.MaxZipBytes {
			return nil, tooLarge
		}
		return data, nil
	}
}

func (s *Server) adminThemes(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	list, err := s.themes.List(r.Context())
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, map[string]any{"contract": themes.Contract, "themes": list})
}

func (s *Server) importTheme(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	data, err := readThemeZip(w, r)
	if err != nil {
		return err
	}
	e, findings, err := s.themes.Import(r.Context(), data, currentUser(r).Username)
	if err != nil {
		return err
	}
	if len(findings) > 0 {
		return themeRefused(findings)
	}
	s.auditLog(r, audit.ThemeImport, themeTarget(e), map[string]any{"manifestId": e.ManifestID, "version": e.Version})
	return writeJSON(w, http.StatusCreated, e)
}

func (s *Server) replaceTheme(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	data, err := readThemeZip(w, r)
	if err != nil {
		return err
	}
	e, oldVersion, findings, err := s.themes.Replace(r.Context(), r.PathValue("id"), data, currentUser(r).Username)
	if err != nil {
		return themeError(err)
	}
	if len(findings) > 0 {
		return themeRefused(findings)
	}
	s.auditLog(r, audit.ThemeReplace, themeTarget(e), map[string]any{"from": oldVersion, "to": e.Version})
	return writeJSON(w, http.StatusOK, e)
}

func (s *Server) deleteTheme(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	e, err := s.themes.Delete(r.Context(), r.PathValue("id"))
	if err != nil {
		return themeError(err)
	}
	s.auditLog(r, audit.ThemeDelete, themeTarget(e), map[string]any{"version": e.Version})
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// switchTheme turns a theme on or off; doing it twice records it once.
func (s *Server) switchTheme(on bool) handler {
	action := audit.ThemeDisable
	if on {
		action = audit.ThemeEnable
	}
	return func(w http.ResponseWriter, r *http.Request) error {
		if err := requireAdmin(r); err != nil {
			return err
		}
		e, changed, err := s.themes.SetEnabled(r.Context(), r.PathValue("id"), on)
		if err != nil {
			return themeError(err)
		}
		if changed {
			s.auditLog(r, action, themeTarget(e), nil)
		}
		return writeJSON(w, http.StatusOK, e)
	}
}
